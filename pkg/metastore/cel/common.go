package cel

import "strings"

type ConvertContext struct {
	Buffer strings.Builder
	Args   []any
	// The offset of the next argument in the condition string.
	// Mainly using for PostgreSQL.
	ArgsOffset int
}
