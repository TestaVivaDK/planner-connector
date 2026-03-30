package tools

import (
	"github.com/TestaVivaDK/plannner-connector/internal/auth"
	"github.com/TestaVivaDK/plannner-connector/internal/graph"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterAll registers every MCP tool on the given server.
func RegisterAll(s *server.MCPServer, am *auth.AuthManager, gc *graph.Client) {
	registerAuthTools(s, am)
	registerEndpointTools(s, gc)
	registerPlanTools(s, gc)
	registerBucketTools(s, gc)
	registerTaskTools(s, gc)
	registerTaskDetailTools(s, gc)
}
