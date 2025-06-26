package cel

var entryColumns = map[string]struct{}{
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
}

var documentFields = map[string]struct{}{
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
}

var groupFields = map[string]struct{}{
	"group.source": {},
}
