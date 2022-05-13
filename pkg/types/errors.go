package types

import "errors"

var (
	ErrNotFound    = errors.New("no record")
	ErrNameTooLong = errors.New("name too long")
	ErrIsExist     = errors.New("record existed")
	ErrNotEmpty    = errors.New("group not empty")
	ErrNoGroup     = errors.New("not group")
	ErrIsGroup     = errors.New("this object is a group")
	ErrNoAccess    = errors.New("no access")
	ErrNoPerm      = errors.New("no permission")
	ErrOutOfFS     = errors.New("out of nanafs control")
	ErrUnsupported = errors.New("unsupported operation")
)
