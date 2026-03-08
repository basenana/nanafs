package planning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/basenana/friday/core/session"
	"github.com/basenana/friday/core/tools"
)

func writeTodoListHandler(sess *session.Session) tools.ToolHandlerFunc {
	return func(ctx context.Context, request *tools.Request) (*tools.Result, error) {
		todoList, ok := request.Arguments["todo_list"].([]any)
		if !ok {
			return tools.NewToolResultError("missing required parameter: todo_list"), nil
		}

		todo := &TodoList{}
		for _, todoItem := range todoList {
			todoInfo, ok := todoItem.(map[string]interface{})
			if !ok || len(todoInfo) == 0 {
				return tools.NewToolResultError("invalid todo_list format"), nil
			}

			describe, ok := todoInfo["describe"].(string)
			if !ok || describe == "" {
				return tools.NewToolResultError("invalid todo_list format: describe is required"), nil
			}
			status, ok := todoInfo["status"].(string)
			if !ok || status == "" {
				return tools.NewToolResultError("invalid todo_list format: status is required"), nil
			}

			todo.Todos = append(todo.Todos, &TodoItem{Describe: describe, Status: status})
		}

		todoFile := todoFilePath(sess)
		err := sess.Workdir.MkdirAll(filepath.Dir(todoFile))
		if err != nil {
			return tools.NewToolResultError(fmt.Sprintf("init todo list error: %s", err)), nil
		}

		data, err := json.Marshal(todo)
		if err != nil {
			return nil, err
		}
		err = sess.Workdir.Write(todoFile, string(data))
		if err != nil {
			return tools.NewToolResultError(fmt.Sprintf("saving todo list error: %s", err)), nil
		}

		return tools.NewToolResultText(fmt.Sprintf("Updated todo list to:\n %s", displayTodoList(todo))), nil
	}
}

type TodoList struct {
	Todos []*TodoItem `json:"todos"`
}

func emptyTodoList() *TodoList {
	return &TodoList{Todos: make([]*TodoItem, 0, 10)}
}

type TodoItem struct {
	Describe string `json:"describe"`
	Status   string `json:"status"`
}

func displayTodoList(todo *TodoList) string {
	buf := &bytes.Buffer{}
	buf.WriteString("<current_todo_list>\n")
	todoList := todo.Todos
	if len(todoList) > 0 {
		for _, t := range todoList {
			buf.WriteString(fmt.Sprintf("describe=%s status=%v\n", t.Describe, t.Status))
		}
	} else {
		buf.WriteString("[EMPTY]\n")
	}
	buf.WriteString("</current_todo_list>\n")
	return buf.String()
}

func todoFilePath(sess *session.Session) string {
	return filepath.Join(".friday", "sessions", sess.Root.ID, fmt.Sprintf("todo_list_%s.json", sess.ID))
}
