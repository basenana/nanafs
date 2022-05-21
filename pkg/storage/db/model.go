package db

import "time"

type Object struct {
	ID         string    `db:"id"`
	Name       string    `db:"name"`
	Aliases    string    `db:"aliases"`
	ParentID   string    `db:"parent_id"`
	RefID      string    `db:"ref_id"`
	Kind       string    `db:"kind"`
	Hash       string    `db:"hash"`
	Size       int64     `db:"size"`
	Inode      uint64    `db:"inode"`
	Namespace  string    `db:"namespace"`
	CreatedAt  time.Time `db:"created_at"`
	ChangedAt  time.Time `db:"changed_at"`
	ModifiedAt time.Time `db:"modified_at"`
	AccessAt   time.Time `db:"access_at"`
	Data       []byte    `db:"data"`
}

type ObjectLabel struct {
	ID    string `db:"id"`
	Key   string `db:"key"`
	Value string `db:"value"`
}
