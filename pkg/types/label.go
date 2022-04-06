package types

type Labels struct {
	Labels []Label `json:"labels"`
}

type Label struct {
	Key string
}
