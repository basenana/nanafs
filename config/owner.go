package config

import (
	"os/user"
	"strconv"
)

type FsOwner struct {
	Uid int64 `json:"uid"`
	Gid int64 `json:"gid"`
}

func defaultOwner() *FsOwner {
	u, err := user.Current()
	if err != nil {
		return nil
	}
	result := &FsOwner{}
	result.Uid, _ = strconv.ParseInt(u.Uid, 10, 64)
	result.Gid, _ = strconv.ParseInt(u.Gid, 10, 64)
	return result
}
