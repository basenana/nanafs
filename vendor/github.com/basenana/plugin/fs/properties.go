package fs

import (
	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/types"
	"github.com/basenana/plugin/utils"
)

func buildProperties(request *api.Request) types.Properties {
	properties := types.Properties{}

	// Safely extract document map
	documentMapRaw, documentOK := request.Parameter["document"]
	if documentOK {
		if documentMap, ok := documentMapRaw.(map[string]interface{}); ok {
			document := &types.Document{}
			utils.UnmarshalMap(documentMap, document)
			properties = document.Properties
		}
	}

	// Safely extract properties map (takes priority)
	propertiesMapRaw, propertiesOK := request.Parameter["properties"]
	if propertiesOK {
		if propertiesMap, ok := propertiesMapRaw.(map[string]interface{}); ok {
			utils.UnmarshalMap(propertiesMap, &properties)
		}
	}

	summaryRaw, summaryOK := request.Parameter["summary"]
	if summaryOK {
		if summary, ok := summaryRaw.(string); ok && summary != "" {
			properties.Summarize = summary
		}
	}

	return properties
}
