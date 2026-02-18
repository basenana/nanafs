package research

import (
	"context"

	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/friday/core/session"
	"github.com/basenana/friday/core/tools"
	"github.com/basenana/friday/core/types"
)

type Report struct {
	mainSession string
	title       string
	report      string
}

var _ session.BeforeAgentHook = (*Report)(nil)
var _ session.BeforeModelHook = (*Report)(nil)
var _ session.AfterModelHook = (*Report)(nil)

func (r *Report) BeforeAgent(ctx context.Context, sess *session.Session, req session.AgentRequest) error {
	if r.mainSession == "" {
		r.mainSession = sess.Root.ID
	}

	if r.mainSession != sess.ID {
		return nil
	}

	req.AppendTools(r.submitReportTool())
	return nil
}

func (r *Report) BeforeModel(ctx context.Context, sess *session.Session, req openai.Request) error {
	if r.mainSession != sess.ID {
		return nil
	}

	req.AppendSystemPrompt(FINAL_REPORT_PROMPT)
	return nil
}

func (r *Report) AfterModel(ctx context.Context, sess *session.Session, req openai.Request, apply *openai.Apply) error {
	if r.mainSession != sess.ID {
		return nil
	}

	if len(apply.ToolUse) > 0 {
		return nil
	}

	if r.title != "" {
		return nil
	}

	sess.AppendMessage(&types.Message{AgentMessage: "The final report has not been submitted. " +
		"If you believe the task is complete, please use the \"submit_final_report\" tool to submit the final report."})
	apply.Continue = true
	return nil
}

func (r *Report) GetReport() (string, string) {
	return r.title, r.report
}

func (r *Report) submitReportTool() *tools.Tool {
	return tools.NewTool(
		"submit_final_report",
		tools.WithDescription(SUBMIT_REPORT_DESC_PROMPT),
		tools.WithString("title",
			tools.Required(),
			tools.Description("The title of final report"),
		),
		tools.WithString("markdown",
			tools.Required(),
			tools.Description("The content body of final report"),
		),
		tools.WithToolHandler(func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
			title, ok := request.Arguments["title"].(string)
			if !ok || len(title) == 0 {
				return tools.NewToolResultError("missing required parameter: title"), nil
			}

			markdown, ok := request.Arguments["markdown"].(string)
			if !ok || len(title) == 0 {
				return tools.NewToolResultError("missing required parameter: markdown"), nil
			}

			r.title = title
			r.report = markdown

			return tools.NewToolResultText("submitted"), nil
		}),
	)
}

func NewReport() *Report {
	return &Report{}
}
