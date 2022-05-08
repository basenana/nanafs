package types

import "errors"

var (
	ErrNotFound    = errors.New("no record")
	ErrNameTooLong = errors.New("name too long")
	ErrIsExist     = errors.New("record existed")
	ErrNoGroup     = errors.New("not group")
	ErrIsGroup     = errors.New("this object is a group")
	ErrNoPerms     = errors.New("not group")
	ErrUnsupported = errors.New("unsupported operation")
)
