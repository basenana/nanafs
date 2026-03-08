package research

import (
	"context"
	"strings"

	"github.com/basenana/friday/core/agents"
	agtapi "github.com/basenana/friday/core/api"
	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/friday/core/session"
	"github.com/basenana/friday/core/tools"
	"github.com/basenana/friday/core/types"
)

func newResearchLeader(agt *Agent, sess *session.Session, agentTools []*tools.Tool) agents.Agent {
	leaderTools := newLeaderTool(agt.worker, sess, agentTools, agt.opt)
	leaderTools = append(leaderTools, agentTools...)
	return agents.New(agt.llm, agents.Option{
		SystemPrompt: promptWithMoreInfo(agt.opt.LeaderPrompt),
		MaxLoopTimes: agt.opt.MaxResearchLoopTimes,
		Tools:        leaderTools,
	})
}

func newLeaderTool(worker agents.Agent, sess *session.Session, agentTools []*tools.Tool, option Option) []*tools.Tool {
	return []*tools.Tool{
		tools.NewTool(
			"run_blocking_subagents",
			tools.WithDescription(DEFAULT_RUN_SUBAGENT_DESC_PROMPT),
			tools.WithArray("task_describe_list",
				tools.Required(),
				tools.Items(map[string]interface{}{"type": "string", "description": "The item description must be specific, measurable, achievable, and strongly related to the goal."}),
				tools.Description("The task description needs to be executable and have assessable completion conditions."),
			),
			tools.WithString("reasoning",
				tools.Required(),
				tools.Description("The reason and purpose of creating a sub-agent"),
			),
			tools.WithToolHandler(blockingSubagentTool(worker, sess, agentTools)),
		),
	}
}

func blockingSubagentTool(worker agents.Agent, sess *session.Session, agentTools []*tools.Tool) tools.ToolHandlerFunc {
	return func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
		tasks, ok := request.Arguments["task_describe_list"].([]any)
		if !ok || len(tasks) == 0 {
			return tools.NewToolResultError("missing required parameter: task_describe_list"), nil
		}
		var taskDescList []string
		for _, taskDescStr := range tasks {
			taskDesc, ok := taskDescStr.(string)
			if !ok {
				return tools.NewToolResultError("task_describe_list must be a string array"), nil
			}
			taskDescList = append(taskDescList, taskDesc)
		}

		var (
			result  = make(chan string, len(tasks))
			reports []string
		)

		subRoot := sess.Fork()
		_ = subRoot.CompactHistory(ctx)

		for _, t := range taskDescList {
			func(task string) {
				subSession := subRoot.Fork()
				subSession.History[0] = types.Message{UserMessage: task} // reset task
				content, err := agtapi.ReadAllContent(ctx, worker.Chat(ctx, &agtapi.Request{
					Session:     subSession,
					UserMessage: task,
					Tools:       agentTools,
				}))
				if err != nil {
					result <- strings.Join(
						[]string{"Subagent Task:", task, task, "Report:", content, "Error:", err.Error()}, "\n")
					return
				}

				result <- strings.Join([]string{"Subagent Task:", task, task, "Report:", content}, "\n")
			}(t)
		}
		close(result)

		for content := range result {
			reports = append(reports, content)
		}

		return tools.NewToolResultText(tools.Res2Str(reports)), nil
	}
}

func NewDefaultWorker(llm openai.Client, opt Option) agents.Agent {
	return agents.New(llm, agents.Option{
		SystemPrompt: SUBAGENT_PROMPT,
		MaxLoopTimes: 30,
		Tools:        opt.ResearchTools,
	})
}
