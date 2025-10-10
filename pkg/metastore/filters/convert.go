package filters

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/cel"
	"github.com/basenana/nanafs/pkg/types"
	"gorm.io/gorm"
	"strings"
)

type ConvertContext struct {
	Buffer strings.Builder
	Args   []any
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

func Join(tx *gorm.DB, where string) *gorm.DB {
	switch tx.Dialector.Name() {
	case "sqlite":
		if strings.Contains(where, "`child`.") {
			tx = tx.Joins("JOIN children ON children.child_id = entry.id")
		}
		if strings.Contains(where, "`egroup`.") {
			tx = tx.Joins("JOIN entry_property AS egroup ON egroup.entry = entry.id").Where("egroup.type = ?", types.PropertyTypeGroupAttr)
		}
		if strings.Contains(where, "`property`.") {
			tx = tx.Joins("JOIN entry_property AS property ON property.entry = entry.id").Where("property.type = ?", types.PropertyTypeProperty)
		}
		if strings.Contains(where, "`document`.") {
			tx = tx.Joins("JOIN entry_property AS document ON document.entry = entry.id").Where("document.type = ?", types.PropertyTypeDocument)
		}
	case "postgres":
		if strings.Contains(where, "child.") {
			tx = tx.Joins("JOIN children ON children.child_id = entry.id")
		}
		if strings.Contains(where, "egroup.") {
			tx = tx.Joins("JOIN entry_property AS egroup ON egroup.entry = entry.id").Where("egroup.type = ?", types.PropertyTypeGroupAttr)
		}
		if strings.Contains(where, "property.") {
			tx = tx.Joins("JOIN entry_property AS property ON property.entry = entry.id").Where("property.type = ?", types.PropertyTypeProperty)
		}
		if strings.Contains(where, "document.") {
			tx = tx.Joins("JOIN entry_property AS document ON document.entry = entry.id").Where("document.type = ?", types.PropertyTypeDocument)
		}
	}
	return tx
}
