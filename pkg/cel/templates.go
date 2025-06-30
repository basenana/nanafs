package cel

import (
	"fmt"
	"strings"
)

// SQLTemplate holds database-specific SQL fragments.
type SQLTemplate struct {
	SQLite     string
	MySQL      string
	PostgreSQL string
}

// TemplateDBType represents the database type for templates.
type TemplateDBType string

const (
	SQLiteTemplate     TemplateDBType = "sqlite"
	MySQLTemplate      TemplateDBType = "mysql"
	PostgreSQLTemplate TemplateDBType = "postgres"
)

// SQLTemplates contains common SQL patterns for different databases.
var SQLTemplates = map[string]SQLTemplate{
	/*
		String
	*/
	"content_compare": {
		SQLite:     "`{table}`.`{column}` %s ?",
		MySQL:      "`{table}`.`{column}` %s ?",
		PostgreSQL: "{table}.{column} %s ?",
	},
	"json_content_compare": {
		SQLite:     "JSON_EXTRACT(`{table}`.`{column}`, '$.{jsonkey}') %s ?",
		MySQL:      "JSON_EXTRACT(`{table}`.`{column}`, '$.{jsonkey}') %s ?",
		PostgreSQL: "{table}.{column}->>'{jsonkey}' %s ?",
	},
	"content_like": {
		SQLite:     "`{table}`.`{column}` LIKE ?",
		MySQL:      "`{table}`.`{column}` LIKE ?",
		PostgreSQL: "{table}.{column} ILIKE ?",
	},
	"json_content_like": {
		SQLite:     "JSON_EXTRACT(`{table}`.`{column}`, '$.{jsonkey}') LIKE ?",
		MySQL:      "JSON_EXTRACT(`{table}`.`{column}`, '$.{jsonkey}') LIKE ?",
		PostgreSQL: "{table}.{column}->>'{jsonkey}' ILIKE ?",
	},
	"content_in": {
		SQLite:     "`{table}`.`{column}` IN (%s)",
		MySQL:      "`{table}`.`{column}` IN (%s)",
		PostgreSQL: "{table}.{column} IN (%s)",
	},
	"json_content_in": {
		SQLite:     "JSON_EXTRACT(`{table}`.`{column}`, '$.{jsonkey}') IN (%s)",
		MySQL:      "JSON_EXTRACT(`{table}`.`{column}`, '$.{jsonkey}') IN (%s)",
		PostgreSQL: "{table}.{column}->>'{jsonkey}' IN (%s)",
	},

	/*
		Number
	*/
	"number_compare": {
		SQLite:     "`{table}`.`{column}` %s ?",
		MySQL:      "`{table}`.`{column}` %s ?",
		PostgreSQL: "{table}.{column} %s ?",
	},
	"json_number_compare": {
		SQLite:     "CAST(JSON_EXTRACT(`{table}`.`{column}`, '$.{jsonkey}') AS REAL) %s ?",
		MySQL:      "JSON_EXTRACT(`{table}`.`{column}`, '$.{jsonkey}') %s ?",
		PostgreSQL: "({table}.{column}->>'{jsonkey}')::numeric %s ?",
	},

	/*
		Boolean
	*/
	"json_boolean_compare": {
		SQLite:     "JSON_EXTRACT(`{table}`.`{column}`, '$.{jsonkey}') %s ?",
		MySQL:      "JSON_EXTRACT(`{table}`.`{column}`, '$.{jsonkey}') %s CAST(? AS JSON)",
		PostgreSQL: "({table}.{column}->>'{jsonkey}')::boolean %s ?",
	},
	"json_boolean_check": {
		SQLite:     "JSON_EXTRACT(`{table}`.`{column}`, '$.{jsonkey}') IS TRUE",
		MySQL:      "JSON_EXTRACT(`{table}`.`{column}`, '$.{jsonkey}') = CAST('true' AS JSON)",
		PostgreSQL: "({table}.{column}->>'{jsonkey}')::boolean IS TRUE",
	},

	/*
		Array
	*/
	"json_array_length": {
		SQLite:     "JSON_ARRAY_LENGTH(COALESCE(JSON_EXTRACT(`{table}`.`{column}`, '$.{jsonkey}'), JSON_ARRAY()))",
		MySQL:      "JSON_LENGTH(COALESCE(JSON_EXTRACT(`{table}`.`{column}`, '$.{jsonkey}'), JSON_ARRAY()))",
		PostgreSQL: "jsonb_array_length(COALESCE({table}.{column}->'{jsonkey}', '[]'::jsonb))",
	},
	"json_contains_element": {
		SQLite:     "JSON_EXTRACT(`{table}`.`{column}`, '$.{jsonkey}') LIKE ?",
		MySQL:      "JSON_CONTAINS(JSON_EXTRACT(`{table}`.`{column}`, '$.{jsonkey}'), ?)",
		PostgreSQL: "{table}.{column}->'{jsonkey}' @> jsonb_build_array(?)",
	},

	/*
		Timestamp
	*/
	"timestamp_field": {
		SQLite:     "`{table}`.`{column}`",
		MySQL:      "UNIX_TIMESTAMP(`{table}`.`{column}`)",
		PostgreSQL: "EXTRACT(EPOCH FROM {table}.{column})",
	},
}

// GetSQL returns the appropriate SQL for the given template and database type.
func GetSQL(templateName string, dbType TemplateDBType, identifier string, args ...any) string {
	col, ok := identify2Columns[identifier]
	if !ok {
		return ""
	}

	if col.jsonkey != "" {
		templateName = "json_" + templateName
	}

	template, exists := SQLTemplates[templateName]
	if !exists {
		return ""
	}

	var content string
	switch dbType {
	case SQLiteTemplate:
		content = template.SQLite
	case MySQLTemplate:
		content = template.MySQL
	case PostgreSQLTemplate:
		content = template.PostgreSQL
	default:
		content = template.SQLite
	}

	content = strings.ReplaceAll(content, "{table}", col.table)
	content = strings.ReplaceAll(content, "{column}", col.column)
	content = strings.ReplaceAll(content, "{jsonkey}", col.jsonkey)

	if len(args) > 0 {
		return fmt.Sprintf(content, args...)
	}
	return content
}

// GetParameterPlaceholder returns the appropriate parameter placeholder for the database.
func GetParameterPlaceholder(dbType TemplateDBType, index int) string {
	switch dbType {
	case PostgreSQLTemplate:
		return fmt.Sprintf("$%d", index)
	default:
		return "?"
	}
}

// GetParameterValue returns the appropriate parameter value for the database.
func GetParameterValue(dbType TemplateDBType, templateName string, value interface{}) interface{} {
	switch templateName {
	case "json_contains_element":
		if dbType == SQLiteTemplate {
			return fmt.Sprintf(`%%"%s"%%`, value)
		}
		return value
	default:
		return value
	}
}

// FormatPlaceholders formats a list of placeholders for the given database type.
func FormatPlaceholders(dbType TemplateDBType, count int, startIndex int) []string {
	placeholders := make([]string, count)
	for i := 0; i < count; i++ {
		placeholders[i] = GetParameterPlaceholder(dbType, startIndex+i)
	}
	return placeholders
}
