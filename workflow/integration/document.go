package integration

import (
	"github.com/basenana/nanafs/pkg/types"
	"time"
)

type Document struct {
	EntryID   int64     `json:"entryId"`
	EntryName string    `json:"entryName"`
	Namespace string    `json:"namespace"`
	Source    string    `json:"source"`
	Content   string    `json:"content,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	ChangedAt time.Time `json:"changedAt"`

	types.DocumentProperties `json:",inline"`
}
