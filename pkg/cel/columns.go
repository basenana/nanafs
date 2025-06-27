package cel

import "github.com/google/cel-go/cel"

type column struct {
	table   string
	column  string
	jsonkey string
	valtype *cel.Type
}

var identify2Columns = map[string]column{
	"entry.id":          {},
	"entry.name":        {},
	"entry.aliases":     {},
	"entry.kind":        {},
	"entry.is_group":    {},
	"entry.size":        {},
	"entry.owner":       {},
	"entry.group_owner": {},
	"entry.created_at":  {},
	"entry.changed_at":  {},
	"entry.modified_at": {},
	"entry.access_at":   {},

	// children table
	"child.parent": {},
	"child.child":  {},
	"child.name":   {},

	// properties
	"tags": {},
	"url":  {},
	"site": {},

	// document properties
	"document.title":       {},
	"document.author":      {},
	"document.year":        {},
	"document.source":      {},
	"document.abstract":    {},
	"document.notes":       {},
	"document.keywords":    {},
	"document.url":         {},
	"document.headerImage": {},
	"document.unread":      {},
	"document.marked":      {},

	// group properties
	"group.source": {},
}

var (
	ColumnsList       = []string{"document.keywords"}
	ColumnsBool       = []string{"entry.created_at"}
	ColumnsTime       = []string{"entry.created_at"}
	ColumnsSizeable   = []string{"document.keywords"}
	ColumnsComparable = []string{"entry.created_at"}
)

func CheckValueType(identifier string, value any) error {
	return nil
}
