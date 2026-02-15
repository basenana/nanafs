package subagents

import (
	"context"
	"fmt"

	"github.com/basenana/friday/core/agents"
	"github.com/basenana/friday/core/logger"
	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/friday/core/session"
	"github.com/basenana/friday/core/tools"
)

type Subagents struct {
	llm         openai.Client
	subAgents   []ExpertAgent
	option      Option
	mainSession string
	logger      logger.Logger
}

var _ session.BeforeModelHook = &Subagents{}
var _ session.BeforeAgentHook = &Subagents{}

func (a *Subagents) BeforeAgent(ctx context.Context, sess *session.Session, req session.AgentRequest) error {
	if a.mainSession == "" {
		a.mainSession = sess.Root.ID
	}
	if a.mainSession != sess.ID {
		return nil
	}
	req.AppendTools(a.buildMainAgentTools(sess)...)
	return nil
}

func (a *Subagents) BeforeModel(ctx context.Context, sess *session.Session, req openai.Request) error {
	if a.mainSession != sess.ID {
		return nil
	}
	req.AppendSystemPrompt(initSystemPrompt(a.option))
	return nil
}

func (a *Subagents) buildMainAgentTools(sess *session.Session) []*tools.Tool {
	subAgentTools := make([]*tools.Tool, 0)
	subAgentTools = append(subAgentTools,
		tools.NewTool(fmt.Sprintf("run_task"),
			tools.WithDescription(initTaskDescribePrompt(a.option)),
			tools.WithString("agent_name",
				tools.Required(),
				tools.Description("The name of the subagent."),
			),
			tools.WithString("task_describe",
				tools.Required(),
				tools.Description("Provide a concise description of the problem you are requesting a solution to, along with suggestions for the expected response."),
			),
			tools.WithToolHandler(callSubagentTool(a.option.SubAgents, sess, a.option.Tools)),
		),
	)

	return subAgentTools
}

func NewHook(llm openai.Client, opt Option) *Subagents {
	if opt.SystemPrompt == "" {
		opt.SystemPrompt = RUN_TASK_PROMPT
	}
	if opt.TaskDescribePrompt == "" {
		opt.TaskDescribePrompt = SUBAGENT_TASK_DESC_PROMPT
	}

	return &Subagents{
		llm:       llm,
		subAgents: opt.SubAgents,
		option:    opt,
		logger:    logger.New("subagents"),
	}
}

type Option struct {
	SystemPrompt       string
	TaskDescribePrompt string
	Tools              []*tools.Tool
	SubAgents          []ExpertAgent
}

type ExpertAgent struct {
	Name     string
	Describe string
	Agent    agents.Agent
}
