package workflow

type Manager struct {
	workflows map[string]*Workflow
	jobs      map[string]*Job
}
