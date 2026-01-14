package openai

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/basenana/friday/core/logger"
	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
	"golang.org/x/time/rate"
)

type client struct {
	openai     openai.Client
	model      Model
	apiLimiter *rate.Limiter
	logger     logger.Logger
}

func (c *client) Completion(ctx context.Context, request Request) Response {
	resp := newSimpleResponse()
	go func() {
		defer resp.close()
		var (
			p       = c.chatCompletionNewParams(request)
			startAt = time.Now()
			err     error
		)

		defer func() {
			c.logger.Infow("completion-with-streaming finish", "elapsed", time.Since(startAt).String())
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

func (c *client) CompletionNonStreaming(ctx context.Context, request Request) (string, error) {
	var (
		p       = c.chatCompletionNewParams(request)
		startAt = time.Now()
		err     error
	)

	defer func() {
		c.logger.Infow("completion-non-streaming finish", "elapsed", time.Since(startAt).String())
	}()

Retry:
	if err = c.apiLimiter.Wait(ctx); err != nil {
		c.logger.Errorw("new completion error", "err", err)
		return "", err
	}
	if time.Since(startAt).Seconds() > 1 {
		c.logger.Infow("client-side llm api throttled", "wait", time.Since(startAt).String())
	}

	response, err := c.openai.Chat.Completions.New(ctx, *p,
		[]option.RequestOption{
			option.WithJSONSet("stream", false), // for some model using stream as default
		}...)
	if err != nil {
		if isTooManyError(err) {
			time.Sleep(time.Second * 10)
			c.logger.Warn("too many requests try again")
			goto Retry
		}
		c.logger.Errorw("completion error", "err", err)
		return "", err
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	return response.Choices[0].Message.Content, nil
}

func (c *client) StructuredPredict(ctx context.Context, request Request, model any) error {
	messages := request.History()
	if len(messages) == 0 || messages[0].SystemMessage == "" {
		return fmt.Errorf("user request is empty")
	}
	prompt := DEFAULT_STRUCTURED_PREDICT_PROMPT
	prompt = strings.ReplaceAll(prompt, "{insert_user_request_here}", messages[0].SystemMessage)
	schemaRaw, _ := json.Marshal(jsonschema.Reflect(model))
	prompt = strings.ReplaceAll(prompt, "{insert_json_schema_here}", string(schemaRaw))

	jsonbody, err := c.CompletionNonStreaming(ctx, NewSimpleRequest(prompt))
	if err != nil {
		c.logger.Errorw("get completion error", "err", err)
		return err
	}

	err = extractJSON(jsonbody, model)
	if err != nil {
		c.logger.Errorw("failed to extract json", "content", jsonbody, "err", err)
		return err
	}
	return nil
}

func (c *client) chatCompletionNewParams(request Request) *openai.ChatCompletionNewParams {
	p := openai.ChatCompletionNewParams{
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
			p.Messages = append(p.Messages,
				openai.ToolMessage(msg.ToolContent, msg.ToolCallID),
			)
		case msg.ImageURL != "":
			p.Messages = append(p.Messages,
				openai.UserMessage([]openai.ChatCompletionContentPartUnionParam{
					openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{URL: msg.ImageURL}),
				}),
			)
		}
	}

	tools := request.ToolDefines()
	for _, t := range tools {
		p.Tools = append(p.Tools, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        t.Name,
				Strict:      param.NewOpt(c.model.StrictMode),
				Description: param.NewOpt(t.Description),
				Parameters:  t.Parameters,
			},
			Type: "function",
		})
	}

	return &p
}
func New(host, apiKey string, model Model) Client {
	return newClient(host, apiKey, model)
}

func newClient(host, apiKey string, model Model) *client {
	tp := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	if model.Proxy != "" {
		proxyUrl, err := url.Parse(model.Proxy)
		if err == nil {
			tp.Proxy = http.ProxyURL(proxyUrl)
		}
	}
	cli := &http.Client{
		Transport: tp,
		Timeout:   time.Hour,
	}

	oc := openai.NewClient(
		option.WithBaseURL(host),
		option.WithAPIKey(apiKey),
		option.WithHTTPClient(cli),
	)

	if model.QPM == 0 {
		model.QPM = 20
	}

	return &client{
		openai:     oc,
		model:      model,
		apiLimiter: rate.NewLimiter(rate.Limit(float64(model.QPM)/60), int(model.QPM/2)),
		logger:     logger.New("openai"),
	}
}
