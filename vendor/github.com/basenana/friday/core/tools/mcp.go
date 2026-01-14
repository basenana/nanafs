package tools

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

type MCPServer struct {
	Name     string
	Describe string

	SSE *MCPSse

	client *client.Client
}

func (s *MCPServer) Connect() error {
	httpTransport, err := transport.NewStreamableHTTP(s.SSE.Endpoint)
	if err != nil {
		return fmt.Errorf("failed to create HTTP transport: %w", err)
	}
	s.client = client.NewClient(httpTransport)
	return nil
}

func (s *MCPServer) InitTools(ctx context.Context) ([]*Tool, error) {
	result, err := s.client.ListTools(ctx, mcp.ListToolsRequest{Header: s.sseHeaders()})
	if err != nil {
		return nil, err
	}
	tools := make([]*Tool, len(result.Tools))
	for i := range result.Tools {
		tool := &result.Tools[i]
		tools[i] = covertMCPTool(tool)
		tools[i].Handler = s.mcpToolAdaptor(tool)
	}
	return tools, nil
}

func (s *MCPServer) sseHeaders() http.Header {
	if s.SSE.Headers == nil {
		return http.Header{}
	}
	h := http.Header{}
	for k, v := range s.SSE.Headers {
		h.Set(k, v)
	}
	return h
}

func (s *MCPServer) mcpToolAdaptor(mcpTool *mcp.Tool) ToolHandlerFunc {
	return func(ctx context.Context, request *Request) (*Result, error) {
		result, err := s.client.CallTool(ctx, mcp.CallToolRequest{
			Request: mcp.Request{},
			Header:  s.sseHeaders(),
			Params: mcp.CallToolParams{
				Name:      "",
				Arguments: request.Arguments,
				Meta:      nil,
			},
		})
		if err != nil {
			return nil, err
		}
		return NewToolResultText(Res2Str(result)), nil
	}
}

func covertMCPTool(tool *mcp.Tool) *Tool {
	return &Tool{
		Name:        tool.Name,
		Description: tool.Description,
		Annotations: make(map[string]string),
		InputSchema: ToolInputSchema{
			Type:       tool.InputSchema.Type,
			Properties: tool.InputSchema.Properties,
			Required:   tool.InputSchema.Required,
		},
	}

}

type MCPSse struct {
	Endpoint string
	Headers  map[string]string
}
