package types

const (
	VersionKey = "nanafs.version"
	KindKey    = "nanafs.kind"
)

type Labels struct {
	Labels []Label `json:"labels"`
}

func (l Labels) Get(key string) *Label {
	for _, label := range l.Labels {
		if label.Key == key {
			return &label
		}
	}
	return nil
}

type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
