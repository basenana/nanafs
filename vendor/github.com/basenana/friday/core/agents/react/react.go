package react

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	agtapi2 "github.com/basenana/friday/core/api"
	"github.com/basenana/friday/core/logger"
	"github.com/basenana/friday/core/memory"
	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/friday/core/tools"
	"github.com/basenana/friday/core/types"
)

type Agent struct {
	name string
	desc string
	llm  openai.Client

	tools      []*tools.Tool
	toolCalled int

	option Option
	logger logger.Logger
}

func (a *Agent) Name() string {
	return a.name
}

func (a *Agent) Describe() string {
	return a.desc
}

func (a *Agent) Chat(ctx context.Context, req *agtapi2.Request) *agtapi2.Response {
	var (
		resp = agtapi2.NewResponse()
	)
	if req.Session == nil {
		req.Session = types.NewDummySession()
	}

	if req.Memory == nil {
		req.Memory = memory.NewEmpty(req.Session.ID)
	}

	ctx = agtapi2.NewContext(ctx, req.Session,
		agtapi2.WithMemory(req.Memory),
		agtapi2.WithResponse(resp),
	)

	mem := req.Memory
	mem.AppendMessages(types.Message{UserMessage: req.UserMessage})

	a.logger.Infow("handle request", "message", req.UserMessage, "session", req.Session.ID)
	go a.reactLoop(ctx, mem, resp)
	return resp
}

func (a *Agent) reactLoop(ctx context.Context, mem *memory.Memory, resp *agtapi2.Response) {
	defer resp.Close()

	var (
		session     = mem.Session().ID
		startAt     = time.Now()
		usage       = &loopUsage{Limit: a.option.MaxLoopTimes, ToolLimit: a.option.MaxToolCalls}
		statusCode  code
		supplements []types.Message
		stream      openai.Response
		err         error
	)

	defer func() {
		a.logger.Infow("react loop finish",
			"toolUse", usage.ToolUse, "extraTry", usage.Try, "session", session, "elapsed", time.Since(startAt).String())
	}()

	for {
		if usage.ToolUse >= usage.ToolLimit {
			a.logger.Warnw("too many tool calls exceeded", "session", session)
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
			err = mem.RunBeforeModelHook(ctx)
			if err != nil {
				resp.Fail(err)
				return
			}
			stream = a.llm.Completion(ctx, newLLMRequest(a.option.SystemPrompt, mem, a.tools))
			err = mem.RunAfterModelHook(ctx)
			if err != nil {
				resp.Fail(err)
				return
			}

			supplements, statusCode = a.handleLLMStream(ctx, stream, mem, resp, usage)
		}

		if statusCode == 1 {
			return
		}

		if len(supplements) > 0 {
			for _, supplement := range supplements {
				if supplement.ToolContent != "" {
					usage.ToolUse++
				}
			}
			mem.AppendMessages(supplements...)
			continue
		}

		/*
			extraTry implements a safeguard mechanism
			tracking retry attempts beyond expected levels in LLM processes
		*/
		usage.Try++
		if usage.Try >= usage.Limit {
			a.logger.Warnw("too many loop times exceeded", "session", session)
			return
		}
		if usage.Try > 3 {
			a.logger.Warnw("the LLM did not terminate the loop as expected", "status", statusCode, "extraTry", usage.Try, "session", session)
		}
		switch statusCode {
		case 2:
			mem.AppendMessages(types.Message{AgentMessage: `The tool is used in an incorrect format; please try using the tool again.`})
		default:
			mem.AppendMessages(types.Message{AgentMessage: "If you believe the conversation is complete, use the tool to end the conversation."})
		}
	}
}

func (a *Agent) handleLLMStream(ctx context.Context, stream openai.Response, mem *memory.Memory, resp *agtapi2.Response, usage *loopUsage) ([]types.Message, code) {
	var (
		content      string
		reasoning    string
		messageCount int
		toolUse      []openai.ToolUse

		session     = mem.Session().ID
		statusCode  = code(0) // not finish
		supplements []types.Message
		warnTicker  = time.NewTicker(time.Minute)
	)

	defer warnTicker.Stop()

WaitMessage:
	for {
		select {
		case <-ctx.Done():
			resp.Fail(ctx.Err())
			return supplements, 0
		case err := <-stream.Error():
			if err != nil {
				resp.Fail(err)
				return supplements, 0
			}
		case <-warnTicker.C:
			a.logger.Warnw("still waiting llm completed", "receivedMessage", messageCount, "session", session)

		case msg, ok := <-stream.Message():
			if !ok {
				break WaitMessage
			}

			messageCount += 1 // check llm api is hang
			switch {
			case len(msg.Content) > 0:
				content += msg.Content
				agtapi2.SendEventToResponse(ctx, types.NewContentEvent(msg.Content))

			case len(msg.ToolUse) > 0:
				for i := range msg.ToolUse {
					tool := msg.ToolUse[i]
					if strings.HasPrefix(tool.Name, "topic_finish_") {
						statusCode.update(1)
						continue WaitMessage
					}
					toolUse = append(toolUse, tool)
				}

			case len(msg.Reasoning) > 0:
				reasoning += msg.Reasoning
				agtapi2.SendEventToResponse(ctx, types.NewReasoningEvent(msg.Reasoning))
			}
		}
	}

	a.logger.Infow("message finish",
		"fuzzyTokens", mem.Tokens(), "promptTokens", stream.Tokens().PromptTokens,
		"completionTokens", stream.Tokens().CompletionTokens, "session", session)

	func() {
		/*
			WORKAROUND FOR STUPID LLM
		*/
		content = strings.TrimSpace(content)
		reasoning = strings.TrimSpace(reasoning)

		if content == "" {
			return
		}

		if strings.Contains(content, "topic_finish_") {
			a.logger.Warnw("topic_finish tool use incorrect", "content", content, "session", session)
			statusCode.update(1)
		} else if strings.Contains(content, "<tool_use") {
			a.logger.Warnw("tool use incorrect", "content", content, "session", session)
			statusCode.update(2)
		}
	}()

	if reasoning != "" {
		mem.AppendMessages(types.Message{AssistantReasoning: reasoning})
	}

	if len(toolUse) > 0 {
		toolCallMessages := a.doToolCalls(ctx, mem, toolUse, reasoning, usage)
		if len(toolCallMessages) > 0 {
			supplements = append(supplements, toolCallMessages...)
		}
	}

	if len(content) > 0 {
		mem.AppendMessages(types.Message{AssistantMessage: content})
	}
	return supplements, statusCode
}

func (a *Agent) doToolCalls(ctx context.Context, mem *memory.Memory, toolUses []openai.ToolUse, reasoning string, usage *loopUsage) []types.Message {
	var (
		result       []types.Message
		update       = make(chan []types.Message, len(toolUses))
		toolUseCount = usage.ToolUse // remind the usage
		wg           = &sync.WaitGroup{}
	)

	for i := range toolUses {
		use := toolUses[i]
		wg.Add(1)
		go func(tc int) {
			defer wg.Done()
			// for long tool use such like an agent call
			update <- a.tryToolCall(ctx, mem, use, reasoning, tc)
		}(toolUseCount + i + 1)
	}
	wg.Wait()
	close(update)

	for message := range update {
		if len(message) == 0 {
			continue
		}
		result = append(result, message...)
	}
	return result
}

func (a *Agent) tryToolCall(ctx context.Context, mem *memory.Memory, use openai.ToolUse, reasoning string, toolUseCount int) []types.Message {
	var (
		result    []types.Message
		extraArgs = agtapi2.OverwriteToolArgsFromContext(ctx)
		useMark   = use.ID
		session   = mem.Session().ID
	)

	if useMark == "" {
		useMark = use.Name
	}

	// request a tool call message
	// DeepSeek v3.2: if the model support using tool in reasoning, the reasoning need return
	result = append(result, types.Message{ToolCallID: useMark, ToolName: use.Name, ToolArguments: use.Arguments, AssistantReasoning: reasoning})

	td := a.getToolByName(mem, use.Name)
	if td == nil {
		msg := fmt.Sprintf("tool %s not found", use.Name)
		result = append(result, types.Message{ToolCallID: useMark, ToolContent: msg})
		agtapi2.SendEventToResponse(ctx, types.NewToolUseEvent(use.Name, use.Arguments, "", msg))
		a.logger.Warnw(msg, "tool", use.Name, "session", session)
		return result
	}

	if use.Error != "" {
		result = append(result, types.Message{ToolCallID: useMark, ToolContent: use.Error})
		agtapi2.SendEventToResponse(ctx, types.NewToolUseEvent(use.Name, use.Arguments, td.Description, use.Error))
		a.logger.Warnw("try tool call error", "tool", use.Name, "error", use.Error, "session", session)
		return result
	}

	toolUse := &ToolUse{GenID: use.ID, Name: use.Name, Arguments: use.Arguments}
	a.logger.Infow("父 using tool", "tool", toolUse.Name, "args", toolUse.Arguments, "session", session)
	msg, err := toolCall(ctx, mem, toolUse, extraArgs, td, toolUseCount)
	if err != nil {
		result = append(result, types.Message{ToolCallID: toolUse.ID(), ToolContent: fmt.Sprintf("using tool failed: %s", err)})
		agtapi2.SendEventToResponse(ctx, types.NewToolUseEvent(use.Name, use.Arguments, td.Description, err.Error()))
		a.logger.Warnw("using tool failed", "tool", use.Name, "error", err, "session", session)
		return result
	}

	result = append(result, types.Message{ToolCallID: toolUse.ID(), ToolContent: msg})
	agtapi2.SendEventToResponse(ctx, types.NewToolUseEvent(use.Name, use.Arguments, td.Description, ""))

	return result
}

func (a *Agent) getToolByName(mem *memory.Memory, name string) *tools.Tool {
	for _, tool := range mem.Tools() {
		if tool.Name == name {
			return tool
		}
	}
	for _, tool := range a.tools {
		if tool.Name == name {
			return tool
		}
	}
	return nil
}

func New(name, desc string, llm openai.Client, option Option) *Agent {
	if option.SystemPrompt == "" {
		option.SystemPrompt = DEFAULT_SYSTEM_PROMPT
	}
	if option.MaxLoopTimes == 0 {
		option.MaxLoopTimes = 5
	}
	if option.MaxToolCalls == 0 {
		option.MaxToolCalls = 20
	}

	agt := &Agent{
		name:   name,
		desc:   desc,
		llm:    llm,
		tools:  option.Tools,
		option: option,
		logger: logger.New("react").With("name", name),
	}

	return agt
}

type Option struct {
	SystemPrompt string
	MaxLoopTimes int
	MaxToolCalls int

	Tools []*tools.Tool
}

/*
code : react loop status

	0 - not finish
	1 - finish
	2 - tool use issue
*/
type code int32

func (c *code) update(n int32) {
	if c == nil {
		return
	}
	if n > int32(*c) {
		*c = code(n)
	}
}

type loopUsage struct {
	Try       int
	Limit     int
	ToolUse   int
	ToolLimit int
}
