package types

// Kind of Object
type Kind string

const (
	/*
		system-wide kind
	*/
	GroupKind = "group"

	/*
		text based file kind
	*/
	TextKind = "text"

	/*
		format doc kind
	*/
	FmtDocKind = "fmtdoc"

	/*
		media file kind
	*/
	ImageKind = "image"
	VideoKind = "video"
	AudioKind = "audio"

	/*
		web based file kind
	*/
	WebArchiveKind = "web"

	/*
		time management system object kind
	*/
	TodoActionItemKind = "tdaction"
	TodoProjectKind    = "tdproject"
	TodoGroupKind      = "tdgroup"

	/*
		ungrouped files
	*/
	RawKind = "raw"
)
