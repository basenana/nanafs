package db

import (
	"context"
	"github.com/jmoiron/sqlx"
)

// CurrentMaxInode
// FIXME: replace the stupid way
func CurrentMaxInode(ctx context.Context, db *sqlx.DB) (inode int64, err error) {
	var total int64
	err = db.Get(&total, `SELECT count(*) FROM "object"`)
	if err != nil {
		return
	}
	if total == 0 {
		return
	}
	query := `SELECT max(inode) FROM "object"`
	err = db.Get(&inode, query)
	return
}
