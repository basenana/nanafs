# 🔩 Go Flow

Go Flow is a lightweight workflow framework based on state machines, 
designed to simplify the creation and management of workflows in Go.

## ✨ Getting Started

```shell
go get github.com/basenana/go-flow
```

### Sample Task

You can create a workflow directly from functions:

```go
builder := flow.NewFlowBuilder("sample-flow-01")

builder.Function("task-1", func(ctx context.Context) error {
    fmt.Println("do something in task 1")
    return nil
}))
builder.Function("task-2", func(ctx context.Context) error {
    fmt.Println("do something in task 2")
    return nil
}))
builder.Function("task-3", func(ctx context.Context) error {
    fmt.Println("do something in task 3")
    return nil
}))

sampleFlow := builder.Finish()

runner := flow.NewRunner(sampleFlow)
_ = runner.Start(context.TODO())
```

## Observer

You can customize the observer to track the status of flows and tasks, 
enabling operations such as persistence and logging.

```go
type storageObserver struct {
	flow map[string]*flow.Flow
	task map[string]flow.Task

	sync.Mutex
}

func (s *storageObserver) Handle(event flow.UpdateEvent) {
	s.Lock()
	defer s.Unlock()

	fmt.Printf("update flow %s", event.Flow.ID)
	s.flow[event.Flow.ID] = event.Flow

	if event.Task != nil {
		fmt.Printf("update flow %s task %s", event.Flow.ID, event.Task.GetName())
		s.task[event.Task.GetName()] = event.Task
	}
}

var _ flow.Observer = &storageObserver{}
```

Register the observer in the builder:
```go
builder := flow.NewFlowBuilder("sample-flow-01").Observer(&storageObserver{
		flow: make(map[string]*flow.Flow),
		task: make(map[string]flow.Task),
	})
```

### DAG

For more complex tasks, workflows based on DAGs are also supported:

```go
builder := flow.NewFlowBuilder("dag-flow-01").Coordinator(flow.NewDAGCoordinator())

builder.Task(flow.NewFuncTask("task-1", func(ctx context.Context) error {
    fmt.Println("do something in task 1")
    return nil
}))
builder.Task(flow.NewFuncTask("task-2", func(ctx context.Context) error {
    fmt.Println("do something in task 2")
    return nil
}))

task3 := flow.NewFuncTask("task-3", func(ctx context.Context) error {
    fmt.Println("do something in task 3")
    return nil
})
builder.Task(flow.WithDependent(task3, []string{"task-1", "task-2"}))

dagFlow := builder.Finish()

runner := flow.NewRunner(dagFlow)
_ = runner.Start(context.TODO())
```

## 🧩 Extensible

### My Task

You can also define your own Task types and corresponding Executors:

```go
type MyTask struct {
	flow.BasicTask
	parameters map[string]string
}

func NewMyTask(name string, parameters map[string]string) flow.Task {
	return &MyTask{BasicTask: flow.BasicTask{Name: name}, parameters: parameters}
}

type MyExecutor struct{}

var _ flow.Executor = &MyExecutor{}

func (m *MyExecutor) Setup(ctx context.Context) error {
	return nil
}

func (m *MyExecutor) Exec(ctx context.Context, flow *flow.Flow, task flow.Task) error {
	myTask, ok := task.(*MyTask)
	if !ok {
		return fmt.Errorf("not my task")
	}

	fmt.Printf("exec my task: %s\n", myTask.Name)
	return nil
}

func (m *MyExecutor) Teardown(ctx context.Context) error {
	return nil
}
```

Load the custom Tasks into the Flow:

```go
builder := flow.NewFlowBuilder("my-flow-01").Executor(&MyExecutor{})

builder.Task(NewMyTask("my-task-1", make(map[string]string)))
builder.Task(NewMyTask("my-task-2", make(map[string]string)))
builder.Task(NewMyTask("my-task-3", make(map[string]string)))

myFlow := builder.Finish()

runner := flow.NewRunner(myFlow)
_ = runner.Start(context.TODO())
```