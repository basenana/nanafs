package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
)

type column struct {
	table   string
	column  string
	jsonkey string
	valtype *cel.Type
}

var identify2Columns = map[string]column{
	"id":          {table: "entry", column: "id", jsonkey: "", valtype: cel.IntType},
	"aliases":     {table: "entry", column: "aliases", jsonkey: "", valtype: cel.StringType},
	"kind":        {table: "entry", column: "kind", jsonkey: "", valtype: cel.StringType},
	"is_group":    {table: "entry", column: "is_group", jsonkey: "", valtype: cel.BoolType},
	"size":        {table: "entry", column: "size", jsonkey: "", valtype: cel.IntType},
	"owner":       {table: "entry", column: "owner", jsonkey: "", valtype: cel.IntType},
	"group_owner": {table: "entry", column: "group_owner", jsonkey: "", valtype: cel.IntType},
	"created_at":  {table: "entry", column: "created_at", jsonkey: "", valtype: cel.IntType},
	"changed_at":  {table: "entry", column: "changed_at", jsonkey: "", valtype: cel.IntType},
	"modified_at": {table: "entry", column: "modified_at", jsonkey: "", valtype: cel.IntType},
	"access_at":   {table: "entry", column: "access_at", jsonkey: "", valtype: cel.IntType},

	// children table
	"parent": {table: "children", column: "parent", jsonkey: "", valtype: cel.IntType},
	"child":  {table: "children", column: "child", jsonkey: "", valtype: cel.IntType},
	"name":   {table: "children", column: "name", jsonkey: "", valtype: cel.StringType},

	// properties
	"tag":           {table: "property", column: "value", jsonkey: "tags", valtype: cel.StringType},
	"tags":          {table: "property", column: "value", jsonkey: "tags", valtype: cel.ListType(cel.StringType)},
	"url":           {table: "property", column: "value", jsonkey: "url", valtype: cel.StringType},
	"site":          {table: "property", column: "value", jsonkey: "site", valtype: cel.StringType},
	"index_version": {table: "property", column: "value", jsonkey: "index_version", valtype: cel.StringType},
	"summarize":     {table: "property", column: "value", jsonkey: "summarize", valtype: cel.StringType},

	// document properties
	"title":    {table: "document", column: "value", jsonkey: "title", valtype: cel.StringType},
	"abstract": {table: "document", column: "value", jsonkey: "abstract", valtype: cel.StringType},
	"notes":    {table: "document", column: "value", jsonkey: "notes", valtype: cel.StringType},
	"keyword":  {table: "document", column: "value", jsonkey: "keywords", valtype: cel.StringType},
	"keywords": {table: "document", column: "value", jsonkey: "keywords", valtype: cel.ListType(cel.StringType)},
	"unread":   {table: "document", column: "value", jsonkey: "unread", valtype: cel.BoolType},
	"marked":   {table: "document", column: "value", jsonkey: "marked", valtype: cel.BoolType},

	// group properties
	"group.source": {table: "egroup", column: "value", jsonkey: "source", valtype: cel.StringType},
}

var (
	ColumnsBool       []string
	ColumnsSizeable   []string
	ColumnsComparable []string
	ColumnsList       = []string{"tag", "tags", "keyword", "keywords"}
	ColumnsTime       = []string{"created_at", "changed_at", "modified_at", "access_at"}
)

func init() {
	for id, col := range identify2Columns {
		switch col.valtype {
		case cel.IntType:
			ColumnsComparable = append(ColumnsComparable, id)
		case cel.StringType:
			ColumnsComparable = append(ColumnsComparable, id)
		case cel.BoolType:
			ColumnsComparable = append(ColumnsComparable, id)
			ColumnsBool = append(ColumnsBool, id)
		case cel.ListType(cel.StringType):
			ColumnsSizeable = append(ColumnsSizeable, id)
		}
	}
}

func CheckValueType(identifier string, value any) error {
	col, ok := identify2Columns[identifier]
	if !ok {
		return fmt.Errorf("identifier %s not found in columns", identifier)
	}

	switch col.valtype {
	case cel.IntType:
		_, ok = value.(int)
		if ok {
			return nil
		}
		_, ok = value.(int64)
		if ok {
			return nil
		}
	case cel.StringType:
		_, ok = value.(string)
		if ok {
			return nil
		}
	case cel.BoolType:
		_, ok = value.(bool)
		if ok {
			return nil
		}
	default:
		return fmt.Errorf("unexpected constant type")
	}
	return fmt.Errorf("val %s is invalid for %s", value, identifier)
}
