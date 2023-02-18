package db

import (
	"database/sql"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"gorm.io/gorm"
)

func dbError2Error(err error) error {
	switch err {
	case sql.ErrNoRows:
		return types.ErrNotFound

	default:
		return err
	}
}

func queryFilter(tx *gorm.DB, filter types.Filter) *gorm.DB {
	if filter.ID != 0 {
		tx = tx.Where("id = ?", filter.ID)
	}
	if filter.ParentID != 0 {
		tx = tx.Where("parent_id = ?", filter.ParentID)
	}
	if filter.RefID != 0 {
		tx = tx.Where("ref_id = ?", filter.RefID)
	}
	if filter.Kind != "" {
		tx = tx.Where("kind = ?", filter.Kind)
	}
	if filter.Namespace != "" {
		tx = tx.Where("namespace = ?", filter.Namespace)
	}
	return tx
}

func labelSearchKey(k, v string) string {
	return fmt.Sprintf("%s=%s", k, v)
}
