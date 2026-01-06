package types

type Document struct {
	Content    string     `json:"content"`
	Properties Properties `json:"properties"`
}

type Properties struct {
	Title string `json:"title"`

	// papers
	Author string `json:"author,omitempty"`
	Year   string `json:"year,omitempty"`
	Source string `json:"source,omitempty"`

	// content
	Abstract string   `json:"abstract,omitempty"`
	Notes    string   `json:"notes,omitempty"`
	Keywords []string `json:"keywords,omitempty"`

	// web
	URL         string `json:"url,omitempty"`
	HeaderImage string `json:"header_image,omitempty"`

	Unread    bool  `json:"unread"`
	Marked    bool  `json:"marked"`
	PublishAt int64 `json:"publish_at,omitempty"`
}
