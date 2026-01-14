package planning

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/basenana/friday/core/tools"
)

func newPlanningTool(agt *Agent) []*tools.Tool {
	return []*tools.Tool{
		tools.NewTool(
			"list_todolist",
			tools.WithDescription("View the current Todo List content and completion status."),
			tools.WithToolHandler(agt.listTodoListHandler),
		),
		tools.NewTool(
			"append_todolist",
			tools.WithDescription("Adding tasks to the to-do list. Only tasks added to the to-do list can be tracked and executed, this tool is the ONLY WAY to update the Todo List."),
			tools.WithArray("task_descriptions",
				tools.Required(),
				tools.Items(map[string]interface{}{"type": "string", "description": "Describe the task to be performed and how to determine the task's outcome in one sentence."}),
				tools.Description("A list describing a series of tasks, each task described in a single sentence. Each added task must have a quantifiable completion goal."),
			),
			tools.WithToolHandler(agt.appendTodoListHandler),
		),
		tools.NewTool(
			"update_todolist_item",
			tools.WithDescription("Update the status of the task in the current todo list to speed up task scheduling and execution."),
			tools.WithString("item_id",
				tools.Required(),
				tools.Description("The ID of the task in the todo list"),
			),
			tools.WithString("status",
				tools.Required(),
				tools.Enum("not_finish", "done", "canceled"),
				tools.Description("Update task status"),
			),
			tools.WithToolHandler(agt.updateTodoListItemHandler),
		),
	}
}

func (a *Agent) listTodoListHandler(ctx context.Context, request *tools.Request) (*tools.Result, error) {
	content := displayTodoList(a.todo)
	return tools.NewToolResultText(content), nil
}

func (a *Agent) appendTodoListHandler(ctx context.Context, request *tools.Request) (*tools.Result, error) {
	actList, ok := request.Arguments["task_descriptions"].([]any)
	if !ok || len(actList) == 0 {
		return tools.NewToolResultError("missing required parameter: task_descriptions"), nil
	}

	for _, act := range actList {
		desc, ok := act.(string)
		if ok {
			a.todo.append(desc)
		} else {
			a.todo.append(fmt.Sprintf("%v", act))
		}
	}

	content := displayTodoList(a.todo)
	content += "\nOnce you believe the task has been updated and recorded, use the \"topic_finish_close\" tool to end the conversation and submit the todo list for execution."
	return tools.NewToolResultText(content), nil
}

func (a *Agent) updateTodoListItemHandler(ctx context.Context, request *tools.Request) (*tools.Result, error) {
	item, ok := request.Arguments["item_id"].(string)
	if !ok || item == "" {
		return tools.NewToolResultError("missing required parameter: item_id"), nil
	}
	status, ok := request.Arguments["status"].(string)
	if !ok || status == "" {
		return tools.NewToolResultError("missing required parameter: status"), nil
	}

	err := a.todo.update(item, status)
	if err != nil {
		return tools.NewToolResultError(err.Error()), nil
	}

	return tools.NewToolResultText(displayTodoList(a.todo)), nil
}

type TodoList struct {
	Items map[string]*TodoListItem

	idCnt int32
	mux   sync.Mutex
}

func emptyTodoList() *TodoList {
	return &TodoList{Items: make(map[string]*TodoListItem)}
}

func (t *TodoList) list() []TodoListItem {
	t.mux.Lock()
	defer t.mux.Unlock()
	var result = make([]TodoListItem, 0, len(t.Items))
	for _, item := range t.Items {
		result = append(result, *item)
	}
	return result
}

func (t *TodoList) append(actions ...string) {
	t.mux.Lock()
	defer t.mux.Unlock()

	for _, act := range actions {
		t.idCnt += 1
		item := &TodoListItem{
			ID:       t.idCnt,
			Describe: act,
		}
		t.Items[fmt.Sprintf("%d", item.ID)] = item
	}
}

func (t *TodoList) update(itemId, status string) error {
	t.mux.Lock()
	item, ok := t.Items[itemId]
	t.mux.Unlock()

	if !ok {
		return fmt.Errorf("action %s not found", itemId)
	}

	switch status {
	case "done", "finish", "finished", "completed", "complete":
		item.IsFinish = true
	case "canceled", "cancel":
		item.IsFinish = true
	case "not_finish":
		item.IsFinish = false
	default:
		return fmt.Errorf("unknown status %s, accepted status: 'done' 'canceled'", status)
	}
	return nil
}

type TodoListItem struct {
	ID       int32  `json:"id"`
	Describe string `json:"describe"`
	IsFinish bool   `json:"is_finish"`
}

func displayTodoList(todo *TodoList) string {
	buf := &bytes.Buffer{}
	buf.WriteString("--- Current TODO List ---\n")
	todoList := todo.list()
	if len(todoList) > 0 {
		for _, t := range todoList {
			buf.WriteString(fmt.Sprintf("id=%d describe=%s done=%v\n", t.ID, t.Describe, t.IsFinish))
		}
	} else {
		buf.WriteString("[EMPTY]\n")
	}
	buf.WriteString("--- \n")
	return buf.String()
}
