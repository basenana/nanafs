package tools

import (
	"context"
)

type ToolSet interface {
	List() []*Tool
}

type ToolHandlerFunc func(ctx context.Context, request *Request) (*Result, error)

type Tool struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Annotations map[string]string `json:"annotations,omitempty"`
	InputSchema ToolInputSchema   `json:"inputSchema"`
	Handler     ToolHandlerFunc   `json:"-"`
}

func (t *Tool) JsonSchema() map[string]interface{} {
	if t.InputSchema.Required == nil {
		t.InputSchema.Required = make([]string, 0)
	}
	return map[string]interface{}{"type": "object", "properties": t.InputSchema.Properties, "required": t.InputSchema.Required}
}

func (t *Tool) GetName() string               { return t.Name }
func (t *Tool) GetDescription() string        { return t.Description }
func (t *Tool) GetParameters() map[string]any { return t.JsonSchema() }

func NewTool(name string, options ...ToolOption) *Tool {
	t := &Tool{
		Name:        name,
		InputSchema: ToolInputSchema{Properties: make(map[string]interface{})},
	}

	for _, opt := range options {
		opt(t)
	}
	return t
}

type ToolInputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
}

type Request struct {
	Arguments map[string]interface{} `json:"arguments"`
	SessionID string                 `json:"sessionId"`
}

type Result struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// NewToolResultText creates a new CallToolResult with a text content
func NewToolResultText(text string) *Result {
	return &Result{
		Content: []Content{
			TextContent{
				Type: "text",
				Text: text,
			},
		},
	}
}

// NewToolResultError creates a new CallToolResult with an error message.
// Any errors that originate from the tool SHOULD be reported inside the result object.
func NewToolResultError(text string) *Result {
	return &Result{
		Content: []Content{
			TextContent{
				Type: "text",
				Text: text,
			},
		},
		IsError: true,
	}
}

type Content interface {
	isContent()
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (t TextContent) isContent() {}

var _ Content = TextContent{}

type PropertyOption func(map[string]interface{})

type ToolOption func(*Tool)

// WithDescription adds a description to the Tool.
// The description should provide a clear, human-readable explanation of what the tool does.
func WithDescription(description string) ToolOption {
	return func(t *Tool) {
		t.Description = description
	}
}

func WithToolAnnotations(annotations map[string]string) ToolOption {
	return func(t *Tool) {
		t.Annotations = annotations
	}
}

func WithToolHandler(handler ToolHandlerFunc) ToolOption {
	return func(t *Tool) {
		t.Handler = handler
	}
}

//
// Common Property Options
//

// Description adds a description to a property in the JSON Schema.
// The description should explain the purpose and expected values of the property.
func Description(desc string) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["description"] = desc
	}
}

// Required marks a property as required in the tool's input schema.
// Required properties must be provided when using the tool.
func Required() PropertyOption {
	return func(schema map[string]interface{}) {
		schema["required"] = true
	}
}

// Title adds a display-friendly title to a property in the JSON Schema.
// This title can be used by UI components to show a more readable property name.
func Title(title string) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["title"] = title
	}
}

//
// String Property Options
//

// DefaultString sets the default value for a string property.
// This value will be used if the property is not explicitly provided.
func DefaultString(value string) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["default"] = value
	}
}

// Enum specifies a list of allowed values for a string property.
// The property value must be one of the specified enum values.
func Enum(values ...string) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["enum"] = values
	}
}

// MaxLength sets the maximum length for a string property.
// The string value must not exceed this length.
func MaxLength(max int) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["maxLength"] = max
	}
}

// MinLength sets the minimum length for a string property.
// The string value must be at least this length.
func MinLength(min int) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["minLength"] = min
	}
}

// Pattern sets a regex pattern that a string property must match.
// The string value must conform to the specified regular expression.
func Pattern(pattern string) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["pattern"] = pattern
	}
}

//
// Number Property Options
//

// DefaultNumber sets the default value for a number property.
// This value will be used if the property is not explicitly provided.
func DefaultNumber(value float64) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["default"] = value
	}
}

// Max sets the maximum value for a number property.
// The number value must not exceed this maximum.
func Max(max float64) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["maximum"] = max
	}
}

// Min sets the minimum value for a number property.
// The number value must not be less than this minimum.
func Min(min float64) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["minimum"] = min
	}
}

// MultipleOf specifies that a number must be a multiple of the given value.
// The number value must be divisible by this value.
func MultipleOf(value float64) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["multipleOf"] = value
	}
}

//
// Boolean Property Options
//

// DefaultBool sets the default value for a boolean property.
// This value will be used if the property is not explicitly provided.
func DefaultBool(value bool) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["default"] = value
	}
}

//
// Array Property Options
//

// DefaultArray sets the default value for an array property.
// This value will be used if the property is not explicitly provided.
func DefaultArray[T any](value []T) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["default"] = value
	}
}

//
// Property Type Helpers
//

// WithBoolean adds a boolean property to the tool schema.
// It accepts property options to configure the boolean property's behavior and constraints.
func WithBoolean(name string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		schema := map[string]interface{}{
			"type": "boolean",
		}

		for _, opt := range opts {
			opt(schema)
		}

		// Remove required from property schema and add to InputSchema.required
		if required, ok := schema["required"].(bool); ok && required {
			delete(schema, "required")
			t.InputSchema.Required = append(t.InputSchema.Required, name)
		}

		t.InputSchema.Properties[name] = schema
	}
}

// WithNumber adds a number property to the tool schema.
// It accepts property options to configure the number property's behavior and constraints.
func WithNumber(name string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		schema := map[string]interface{}{
			"type": "number",
		}

		for _, opt := range opts {
			opt(schema)
		}

		// Remove required from property schema and add to InputSchema.required
		if required, ok := schema["required"].(bool); ok && required {
			delete(schema, "required")
			t.InputSchema.Required = append(t.InputSchema.Required, name)
		}

		t.InputSchema.Properties[name] = schema
	}
}

// WithString adds a string property to the tool schema.
// It accepts property options to configure the string property's behavior and constraints.
func WithString(name string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		schema := map[string]interface{}{
			"type": "string",
		}

		for _, opt := range opts {
			opt(schema)
		}

		// Remove required from property schema and add to InputSchema.required
		if required, ok := schema["required"].(bool); ok && required {
			delete(schema, "required")
			t.InputSchema.Required = append(t.InputSchema.Required, name)
		}

		t.InputSchema.Properties[name] = schema
	}
}

// WithObject adds an object property to the tool schema.
// It accepts property options to configure the object property's behavior and constraints.
func WithObject(name string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		schema := map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}

		for _, opt := range opts {
			opt(schema)
		}

		// Remove required from property schema and add to InputSchema.required
		if required, ok := schema["required"].(bool); ok && required {
			delete(schema, "required")
			t.InputSchema.Required = append(t.InputSchema.Required, name)
		}

		t.InputSchema.Properties[name] = schema
	}
}

// WithArray adds an array property to the tool schema.
// It accepts property options to configure the array property's behavior and constraints.
func WithArray(name string, opts ...PropertyOption) ToolOption {
	return func(t *Tool) {
		schema := map[string]interface{}{
			"type": "array",
		}

		for _, opt := range opts {
			opt(schema)
		}

		// Remove required from property schema and add to InputSchema.required
		if required, ok := schema["required"].(bool); ok && required {
			delete(schema, "required")
			t.InputSchema.Required = append(t.InputSchema.Required, name)
		}

		t.InputSchema.Properties[name] = schema
	}
}

// Properties defines the properties for an object schema
func Properties(props map[string]interface{}) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["properties"] = props
	}
}

// AdditionalProperties specifies whether additional properties are allowed in the object
// or defines a schema for additional properties
func AdditionalProperties(schema interface{}) PropertyOption {
	return func(schemaMap map[string]interface{}) {
		schemaMap["additionalProperties"] = schema
	}
}

// MinProperties sets the minimum number of properties for an object
func MinProperties(min int) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["minProperties"] = min
	}
}

// MaxProperties sets the maximum number of properties for an object
func MaxProperties(max int) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["maxProperties"] = max
	}
}

// PropertyNames defines a schema for property names in an object
func PropertyNames(schema map[string]interface{}) PropertyOption {
	return func(schemaMap map[string]interface{}) {
		schemaMap["propertyNames"] = schema
	}
}

// Items defines the schema for array items
func Items(schema interface{}) PropertyOption {
	return func(schemaMap map[string]interface{}) {
		schemaMap["items"] = schema
	}
}

// MinItems sets the minimum number of items for an array
func MinItems(min int) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["minItems"] = min
	}
}

// MaxItems sets the maximum number of items for an array
func MaxItems(max int) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["maxItems"] = max
	}
}

// UniqueItems specifies whether array items must be unique
func UniqueItems(unique bool) PropertyOption {
	return func(schema map[string]interface{}) {
		schema["uniqueItems"] = unique
	}
}
