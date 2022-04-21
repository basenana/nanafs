package plugin

type PluginType string

const (
	PluginMeta       PluginType = "meta"
	PluginSource     PluginType = "source"
	PluginProcess    PluginType = "process"
	PluginCustomType PluginType = "custom_type"
)
