package object

type CustomColumn struct {
	Columns []CustomColumnItem `json:"columns"`
}

type CustomColumnItem struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}
