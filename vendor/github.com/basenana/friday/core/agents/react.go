package agents

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/basenana/friday/core/api"
	"github.com/basenana/friday/core/logger"
	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/friday/core/session"
	"github.com/basenana/friday/core/tools"
	"github.com/basenana/friday/core/types"
)

type react struct {
	llm    openai.Client
	tools  []*tools.Tool
	option Option
	logger logger.Logger
}

func (a *react) Chat(ctx context.Context, req *api.Request) *api.Response {
	resp := api.NewResponse()

	sess := req.Session
	if sess == nil {
		sess = session.New(types.NewID(), a.llm)
	}

	err := sess.RunHooks(ctx, types.SessionHookBeforeAgent, session.HookPayload{AgentRequest: req})
	if err != nil {
		resp.Fail(err)
		return resp
	}

	sess.AppendMessage(&types.Message{UserMessage: req.UserMessage})
	a.logger.Infow("handle request", "message", logger.FirstLine(req.UserMessage), "session", sess.ID)
	go a.reactLoop(ctx, sess, resp, req.Tools)
	return resp
}

func (a *react) reactLoop(ctx context.Context, sess *session.Session, resp *api.Response, reqTools []*tools.Tool) {
	defer resp.Close()

	// Merge agent tools with request tools (request tools take precedence)
	mergedTools := make([]*tools.Tool, 0, len(a.tools)+len(reqTools))
	toolNames := make(map[string]bool)

	// Add request tools first (higher precedence)
	for _, t := range reqTools {
		mergedTools = append(mergedTools, t)
		toolNames[t.Name] = true
	}
	// Add agent tools that aren't in request tools
	for _, t := range a.tools {
		if !toolNames[t.Name] {
			mergedTools = append(mergedTools, t)
		}
	}

	var (
		startAt   = time.Now()
		loopTimes = 0
		keepRun   bool
		err       error
	)

	defer func() {
		a.logger.Infow("react loop finish",
			"loopTimes", loopTimes, "maxLoopTimes", a.option.MaxLoopTimes, "session", sess.ID, "elapsed", time.Since(startAt).String())
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			keepRun, err = a.doAct(ctx, sess, resp, mergedTools)
		}
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "exceed max message tokens") {
				compactErr := sess.CompactHistory(ctx)
				if compactErr == nil {
					continue
				}
				a.logger.Warnw("failed to compact history", "error", compactErr.Error())
			}
			resp.Fail(err)
			return
		}

		if !keepRun {
			return
		}

		loopTimes++
		if loopTimes > a.option.MaxLoopTimes {
			a.logger.Warnw("too many loop times exceeded", "session", sess.ID)
			return
		}
	}
}

func (a *react) doAct(ctx context.Context, sess *session.Session, resp *api.Response, toolList []*tools.Tool) (bool, error) {
	var (
		content      string
		reasoning    string
		agentMessage string
		messageCount int
		toolUse      []openai.ToolUse
		err          error

		keepRun    = false
		llmReq     = newLLMRequest(a.option.SystemPrompt, sess, toolList)
		warnTicker = time.NewTicker(time.Minute)
	)

	defer warnTicker.Stop()

	// before_model hooks
	err = sess.RunHooks(ctx, types.SessionHookBeforeModel, session.HookPayload{ModelRequest: llmReq})
	if err != nil {
		return false, err
	}
	stream := a.llm.Completion(ctx, llmReq)

WaitMessage:
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case err = <-stream.Error():
			if err != nil {
				return false, err
			}
		case <-warnTicker.C:
			a.logger.Warnw("still waiting llm completed", "receivedMessage", messageCount, "session", sess.ID)

		case msg, ok := <-stream.Message():
			if !ok {
				break WaitMessage
			}

			messageCount += 1
			switch {
			case len(msg.Content) > 0:
				content += msg.Content
				api.SendDelta(resp, types.Delta{Content: msg.Content})

			case len(msg.ToolUse) > 0:
				for i := range msg.ToolUse {
					tool := msg.ToolUse[i]
					if strings.HasPrefix(tool.Name, "topic_finish_") {
						keepRun = false
						continue WaitMessage
					}
					toolUse = append(toolUse, tool)
				}

			case len(msg.Reasoning) > 0:
				reasoning += msg.Reasoning
				api.SendDelta(resp, types.Delta{Reasoning: msg.Reasoning})
			}
		}
	}

	a.logger.Infow("message finish",
		"fuzzyTokens", sess.Tokens(), "promptTokens", stream.Tokens().PromptTokens,
		"completionTokens", stream.Tokens().CompletionTokens, "session", sess.ID)

	content = strings.TrimSpace(content)
	reasoning = strings.TrimSpace(reasoning)

	if strings.Contains(content, "topic_finish_") {
		a.logger.Warnw("topic_finish tool use incorrect", "content", content, "session", sess.ID)
		agentMessage += "If you believe the conversation is complete, use the tool to end the conversation.\n"
	} else if strings.Contains(content, "<tool_use") {
		a.logger.Warnw("tool use incorrect", "content", content, "session", sess.ID)
		agentMessage += "The tool is used in an incorrect format; please try using the tool again.\n"
	}

	if reasoning != "" {
		sess.AppendMessage(&types.Message{AssistantReasoning: reasoning})
	}
	if len(content) > 0 {
		sess.AppendMessage(&types.Message{AssistantMessage: content})
	}
	if agentMessage != "" {
		keepRun = true
		sess.AppendMessage(&types.Message{AgentMessage: agentMessage})
	}

	// after_model hooks
	appl := &openai.Apply{ToolUse: toolUse}
	err = sess.RunHooks(ctx, types.SessionHookAfterModel, session.HookPayload{ModelRequest: llmReq, ModelApply: appl})
	if err != nil {
		return false, err
	}
	toolUse = appl.ToolUse

	if len(toolUse) > 0 {
		keepRun = true
		a.doToolCalls(ctx, sess, toolUse, reasoning, toolList)
	}
	if appl.Continue {
		keepRun = true
	}
	if appl.Abort {
		keepRun = false
	}
	return keepRun, nil
}

func (a *react) doToolCalls(ctx context.Context, sess *session.Session, toolUses []openai.ToolUse, reasoning string, toolList []*tools.Tool) {
	var (
		result []*types.Message
		update = make(chan []*types.Message, len(toolUses))
		wg     = &sync.WaitGroup{}
	)

	for i := range toolUses {
		use := toolUses[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			update <- a.tryToolCall(ctx, sess, use, reasoning, toolList)
		}()
	}
	wg.Wait()
	close(update)

	for message := range update {
		if len(message) == 0 {
			continue
		}
		result = append(result, message...)
	}

	sess.AppendMessage(result...)
}

func (a *react) tryToolCall(ctx context.Context, sess *session.Session, use openai.ToolUse, reasoning string, toolList []*tools.Tool) []*types.Message {
	var (
		result  []*types.Message
		useMark = use.ID
	)

	if useMark == "" {
		useMark = use.Name
	}

	result = append(result, &types.Message{ToolCallID: useMark, ToolName: use.Name, ToolArguments: use.Arguments, AssistantReasoning: reasoning})

	td := getToolByName(toolList, use.Name)
	if td == nil {
		msg := fmt.Sprintf("tool %s not found", use.Name)
		result = append(result, &types.Message{ToolCallID: useMark, ToolContent: msg})
		a.logger.Warnw(msg, "tool", use.Name, "session", sess.ID)
		return result
	}

	if use.Error != "" {
		result = append(result, &types.Message{ToolCallID: useMark, ToolContent: use.Error})
		a.logger.Warnw("try tool call error", "tool", use.Name, "error", use.Error, "session", sess.ID)
		return result
	}

	toolUse := &ToolUse{GenID: use.ID, Name: use.Name, Arguments: use.Arguments}
	a.logger.Infow("using tool", "tool", toolUse.Name, "args", toolUse.Arguments, "session", sess.ID)
	msg, err := toolCall(ctx, sess, toolUse, td)
	if err != nil {
		result = append(result, &types.Message{ToolCallID: toolUse.ID(), ToolContent: fmt.Sprintf("using tool failed: %s", err)})
		a.logger.Warnw("using tool failed", "tool", use.Name, "error", err, "session", sess.ID)
		return result
	}

	result = append(result, &types.Message{ToolCallID: toolUse.ID(), ToolContent: msg})
	return result
}

func getToolByName(toolList []*tools.Tool, name string) *tools.Tool {
	for _, tool := range toolList {
		if tool.Name == name {
			return tool
		}
	}
	return nil
}

func New(llm openai.Client, option Option) Agent {
	if option.SystemPrompt == "" {
		option.SystemPrompt = DEFAULT_SYSTEM_PROMPT
	}
	if option.MaxLoopTimes == 0 {
		option.MaxLoopTimes = 50
	}

	agt := &react{
		llm:    llm,
		tools:  option.Tools,
		option: option,
		logger: logger.New("react"),
	}

	return agt
}

type Option struct {
	SystemPrompt string
	MaxLoopTimes int

	Tools []*tools.Tool
}
