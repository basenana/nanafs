package planning

import (
	"bytes"
	"context"
	"fmt"
	"sort"

	"github.com/basenana/friday/core/agents"
	"github.com/basenana/friday/core/agents/react"
	"github.com/basenana/friday/core/api"
	"github.com/basenana/friday/core/logger"
	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/friday/core/tools"
)

type Agent struct {
	react *react.Agent

	name   string
	desc   string
	opt    Option
	todo   *TodoList
	pt     []*tools.Tool
	logger logger.Logger
}

var _ agents.Agent = &Agent{}

func (a *Agent) Name() string {
	return a.name
}

func (a *Agent) Describe() string {
	return a.desc
}

func (a *Agent) Chat(ctx context.Context, req *api.Request) *api.Response {
	return a.react.Chat(ctx, &api.Request{
		Session:     req.Session,
		UserMessage: fmt.Sprintf("%s\n%s", displayTodoList(a.todo), req.UserMessage),
		Memory:      req.Memory,
	})
}

func (a *Agent) PlanningTools() []*tools.Tool {
	if a.pt != nil {
		return a.pt
	}
	a.pt = newPlanningTool(a)
	return a.pt
}

func (a *Agent) TodoList() []TodoListItem {
	orderList := a.todo.list()
	sort.Slice(orderList, func(i, j int) bool {
		return orderList[i].ID < orderList[j].ID
	})
	return orderList
}

func (a *Agent) AllFinish() bool {
	for _, item := range a.TodoList() {
		if !item.IsFinish {
			return false
		}
	}
	return true
}

func (a *Agent) SetTodoDone(todoID int32) {
	_ = a.todo.update(fmt.Sprintf("%d", todoID), "done")
}

func New(name, desc string, llm openai.Client, option Option) *Agent {
	systemPrompt := &bytes.Buffer{}
	systemPrompt.WriteString(DEFAULT_PLANNING_PROMPT)
	if option.SystemPrompt == "" {
		systemPrompt.WriteString("<user_requirements_for_planning>\n")
		systemPrompt.WriteString(option.SystemPrompt)
		systemPrompt.WriteString("</user_requirements_for_planning>\n")
	}

	agt := &Agent{
		name:   name,
		desc:   desc,
		opt:    option,
		todo:   emptyTodoList(),
		logger: logger.New("planning").With("name", name),
	}

	option.Tools = append(option.Tools, agt.PlanningTools()...)

	agt.react = react.New(name, desc, llm, react.Option{
		SystemPrompt: systemPrompt.String(),
		MaxLoopTimes: 5,
		MaxToolCalls: 10,
		Tools:        option.Tools,
	})
	return agt
}

type Option struct {
	SystemPrompt string
	Tools        []*tools.Tool
}
