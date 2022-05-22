package db

import (
	"database/sql"
	"github.com/basenana/nanafs/pkg/types"
)

func dbError2Error(err error) error {
	switch err {
	case sql.ErrNoRows:
		return types.ErrNotFound

	default:
		return err
	}
}
