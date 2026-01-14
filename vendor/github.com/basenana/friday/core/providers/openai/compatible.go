package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	"github.com/basenana/friday/core/logger"
)

type CompatibleClient struct {
	*client
	logger logger.Logger
}

func (c *CompatibleClient) Completion(ctx context.Context, request Request) Response {
	resp := newCompatibleResponse()
	go func() {
		defer resp.close()
		var (
			p       = c.chatCompletionNewParams(request)
			startAt = time.Now()
			err     error
		)

		defer func() {
			c.logger.Infow("[LLM-CALL] completion-with-streaming finish", "elapsed", time.Since(startAt).String())
		}()

	Retry:
		if err = c.apiLimiter.Wait(ctx); err != nil {
			c.logger.Errorw("new completion stream error", "err", err)
			resp.fail(err)
			return
		}
		if time.Since(startAt).Seconds() > 1 {
			c.logger.Infow("client-side llm api throttled", "wait", time.Since(startAt).String())
		}

		stream := c.openai.Chat.Completions.NewStreaming(ctx, *p)

		for stream.Next() {
			chunk := stream.Current()
			resp.updateUsage(chunk.Usage)

			if len(chunk.Choices) == 0 {
				continue
			}

			//c.logger.Infow("new choices found", "chunk", chunk)
			ch := chunk.Choices[0]
			resp.nextChoice(ch)
		}

		if err = stream.Err(); err != nil {
			if isTooManyError(err) {
				time.Sleep(time.Second * 10)
				c.logger.Warn("too many requests try again")
				goto Retry
			}
			c.logger.Errorw("completion stream error", "err", err)
			resp.fail(err)
			return
		}
	}()
	return resp
}

func (c *CompatibleClient) chatCompletionNewParams(request Request) *openai.ChatCompletionNewParams {
	p := &openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{},
		Model:    c.model.Name,
		TopP:     param.NewOpt(1.0),
		N:        param.NewOpt(int64(1)),
	}

	if c.model.Temperature != nil {
		p.Temperature = param.NewOpt(*c.model.Temperature)
	}
	if c.model.FrequencyPenalty != nil {
		p.FrequencyPenalty = param.NewOpt(*c.model.FrequencyPenalty)
	}
	if c.model.PresencePenalty != nil {
		p.PresencePenalty = param.NewOpt(*c.model.PresencePenalty)
	}

	history := request.History()
	for _, msg := range history {
		switch {
		case msg.SystemMessage != "":
			p.Messages = append(p.Messages,
				openai.SystemMessage(msg.SystemMessage),
			)

		case msg.UserMessage != "":
			p.Messages = append(p.Messages,
				openai.UserMessage(msg.UserMessage),
			)

		case msg.AgentMessage != "":
			p.Messages = append(p.Messages,
				openai.UserMessage(msg.AgentMessage),
			)

		case msg.AssistantMessage != "":
			p.Messages = append(p.Messages,
				openai.AssistantMessage(msg.AssistantMessage),
			)

		case msg.ToolName != "": // tool call
			tmsg := &openai.ChatCompletionAssistantMessageParam{
				ToolCalls: []openai.ChatCompletionMessageToolCallParam{
					{ID: msg.ToolCallID, Function: openai.ChatCompletionMessageToolCallFunctionParam{Arguments: msg.ToolArguments, Name: msg.ToolName}, Type: "function"},
				},
			}
			if msg.AssistantReasoning != "" {
				tmsg.SetExtraFields(map[string]any{"reasoning_content": msg.AssistantReasoning})
			}
			p.Messages = append(p.Messages,
				openai.ChatCompletionMessageParamUnion{OfAssistant: tmsg},
			)

		case msg.ToolContent != "":
			tur := &ToolUseResult{Name: msg.ToolCallID, Result: msg.ToolContent}
			content, err := xml.Marshal(tur)
			if err == nil {
				p.Messages = append(p.Messages,
					openai.ToolMessage(string(content), msg.ToolCallID),
				)
			} else {
				p.Messages = append(p.Messages,
					openai.ToolMessage(msg.ToolContent, msg.ToolCallID),
				)
			}

		case msg.ImageURL != "":
			p.Messages = append(p.Messages,
				openai.UserMessage([]openai.ChatCompletionContentPartUnionParam{
					openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{URL: msg.ImageURL}),
				}),
			)
		}
	}

	// rewrite system prompt
	toolList := request.ToolDefines()
	if len(toolList) > 0 {
		p.Tools = nil

		buf := &bytes.Buffer{}
		messages := request.History()
		if len(messages) == 0 || messages[0].SystemMessage == "" {
			c.logger.Warnw("no system prompt found")
			return p
		}

		system := messages[0]
		buf.WriteString(system.SystemMessage)
		buf.WriteString("\n")

		buf.WriteString(DEFAULT_TOOL_USE_PROMPT)
		buf.WriteString("<available_tools>\n")
		buf.WriteString("Above example were using notional tools that might not exist for you. You only have access to these tools:\n")
		toolDefine := &ToolDefinePrompt{}
		for _, tool := range toolList {
			argContent, _ := json.Marshal(tool.Parameters)
			toolDefine.Tools = append(toolDefine.Tools, ToolPrompt{
				Name:        tool.Name,
				Description: tool.Description,
				Arguments:   string(argContent),
			})
		}
		defineContent, _ := xml.Marshal(toolDefine)
		buf.Write(defineContent)
		buf.WriteString("\n")
		buf.WriteString("</available_tools>\n")

		p.Messages[0] = openai.SystemMessage(buf.String())
	}

	return p
}

func NewCompatible(host, apiKey string, model Model) Client {
	return &CompatibleClient{
		client: newClient(host, apiKey, model),
		logger: logger.New("openai.compatible"),
	}
}
