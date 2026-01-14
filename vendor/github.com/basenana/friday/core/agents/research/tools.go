package research

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/basenana/friday/core/agents/summarize"
	agtapi2 "github.com/basenana/friday/core/api"
	"github.com/basenana/friday/core/memory"
	"github.com/basenana/friday/core/tools"
	"github.com/basenana/friday/core/types"
)

func newLeaderTool(agt *Agent) []*tools.Tool {
	return []*tools.Tool{
		tools.NewTool(
			"run_blocking_subagents",
			tools.WithDescription("Submit multiple independent tasks, each task will launch a subagent to conduct research in parallel."),
			tools.WithArray("task_describe_list",
				tools.Required(),
				tools.Items(map[string]interface{}{"type": "string", "description": "The item description must be specific, measurable, achievable, and strongly related to the goal."}),
				tools.Description("The task description needs to be executable and have assessable completion conditions."),
			),
			tools.WithString("reasoning",
				tools.Required(),
				tools.Description("The reason and purpose of creating a sub-agent"),
			),
			tools.WithToolHandler(agt.runBlockingsSubagentHandler),
		),
	}
}

func (a *Agent) runBlockingsSubagentHandler(ctx context.Context, request *tools.Request) (*tools.Result, error) {
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
		parentMem     = a.setupSubagentMemory(ctx)
		subAgentTools []*tools.Tool
	)
	for _, t := range a.opt.Tools {
		subAgentTools = append(subAgentTools, t)
	}

	if sp := parentMem.Scratchpad(); sp != nil {
		subAgentTools = append(subAgentTools, tools.ScratchpadWriteTools(sp)...)
	}

	var (
		agt     = a.newReAct("subagent", "research worker", promptWithMoreInfo(a.opt.SubAgentPrompt), subAgentTools)
		wg      = sync.WaitGroup{}
		result  = make(chan string, len(tasks))
		reports []string
	)

	for _, t := range taskDescList {
		wg.Add(1)
		go func(task string) {
			var (
				startAt = time.Now()
				mem     = parentMem.Copy()
			)

			a.logger.Infof("subagent task: %s", task)
			defer wg.Done()
			content, err := agtapi2.ReadAllContent(ctx, agt.Chat(ctx, &agtapi2.Request{
				Session:     mem.Session(),
				UserMessage: task,
				Memory:      mem,
			}))
			if err != nil {
				a.logger.Warnw("run subagent task failed", "task", task, "err", err)
				result <- strings.Join(
					[]string{"Subagent Task:", task, task, "Report:", content, "Error:", err.Error()}, "\n")
				return
			}

			result <- strings.Join([]string{"Subagent Task:", task, task, "Report:", content}, "\n")
			a.logger.Infow("subagent task finish", "task", task, "elapsed", time.Since(startAt).String())
		}(t)
	}
	wg.Wait()
	close(result)

	for content := range result {
		reports = append(reports, content)
	}

	return tools.NewToolResultText(tools.Res2Str(reports)), nil
}

func (a *Agent) setupSubagentMemory(ctx context.Context) *memory.Memory {

	var (
		parentMem = agtapi2.MemoryFromContext(ctx)
		history   = parentMem.History()
		tokens    int64
	)

	for _, msg := range history {
		tokens += msg.FuzzyTokens()
	}

	if tokens < 10000 {
		return parentMem
	}

	a.logger.Infow("compressing the subagent context", "parentTokens", tokens)

	sum := summarize.New("stagesummary", "", a.llm, summarize.Option{})
	summary, err := agtapi2.ReadAllContent(ctx, sum.Chat(ctx, &agtapi2.Request{Session: parentMem.Session(), Memory: parentMem.Copy()}))
	if err != nil || summary == "" {
		a.logger.Warnw("failed to get stage summary, using origin memory", "err", err)
		return parentMem
	}

	parentMem = parentMem.Copy()
	parentMem.Modify(func(messages []types.Message) []types.Message {
		newMessage := make([]types.Message, len(messages))
		if len(messages) > 0 {
			newMessage = append(newMessage, messages[0])
		}
		newMessage = append(newMessage, types.Message{
			AgentMessage: fmt.Sprintf("This is a summary of the current work progress.: \n%s", summary)})
		return newMessage
	})
	return parentMem
}
