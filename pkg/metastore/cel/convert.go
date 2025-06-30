package cel

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/cel"
	"gorm.io/gorm"
	"strings"
)

type ConvertContext struct {
	Buffer     strings.Builder
	Args       []any
	ArgsOffset int
}

func NewConvertContext() *ConvertContext {
	return &ConvertContext{
		Buffer: strings.Builder{},
		Args:   []any{},
	}
}

func Convert(ctx *ConvertContext, db *gorm.DB, filter string) error {
	parsedExpr, err := cel.Parse(filter)
	if err != nil {
		return err
	}

	switch db.Dialector.Name() {
	case "sqlite":
		return convertWithTemplatesWithSQLite(ctx, parsedExpr.GetExpr())
	case "postgres":
		return convertWithParameterIndexWithPG(ctx, parsedExpr.GetExpr())
	default:
		return fmt.Errorf("unsupported database dialect: %s", db.Dialector.Name())
	}
}
