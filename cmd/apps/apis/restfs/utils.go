package restfs

import "strings"

func pathEntries(path string) []string {
	path = strings.Trim(path, "/fs/")
	return strings.Split(path, "/")
}
