package types

import "errors"

var (
	ErrNotFound    = errors.New("no record")
	ErrIsExist     = errors.New("record existed")
	ErrNoGroup     = errors.New("not group")
	ErrIsGroup     = errors.New("this object is a group")
	ErrUnsupported = errors.New("unsupported operation")
)
