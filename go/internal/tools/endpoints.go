package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/TestaVivaDK/plannner-connector/internal/graph"
	"github.com/TestaVivaDK/plannner-connector/internal/logger"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

//go:embed endpoints.json
var endpointsJSON []byte

type endpointConfig struct {
	PathPattern string            `json:"pathPattern"`
	Method      string            `json:"method"`
	ToolName    string            `json:"toolName"`
	Scopes      []string          `json:"scopes"`
	LLMTip      string            `json:"llmTip,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// OData params exposed without $ prefix for LLM compatibility.
var odataParams = []string{"filter", "select", "top", "orderby", "expand", "count", "search"}

var pathParamRe = regexp.MustCompile(`\{([^}]+)\}`)

func extractPathParams(pattern string) []string {
	matches := pathParamRe.FindAllStringSubmatch(pattern, -1)
	params := make([]string, 0, len(matches))
	for _, m := range matches {
		params = append(params, m[1])
	}
	return params
}

func registerEndpointTools(s *server.MCPServer, gc *graph.Client) {
	var endpoints []endpointConfig
	if err := json.Unmarshal(endpointsJSON, &endpoints); err != nil {
		if logger.Log != nil {
			logger.Log.Error("failed to parse endpoints.json", "error", err)
		}
		return
	}

	count := 0
	for _, ep := range endpoints {
		ep := ep // capture
		pathParams := extractPathParams(ep.PathPattern)

		// Build tool options.
		opts := []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
		}

		desc := fmt.Sprintf("%s %s", strings.ToUpper(ep.Method), ep.PathPattern)
		if ep.LLMTip != "" {
			desc += "\n\nTIP: " + ep.LLMTip
		}
		opts = append(opts, mcp.WithDescription(desc))

		// Path params (required).
		for _, p := range pathParams {
			opts = append(opts, mcp.WithString(p, mcp.Required(), mcp.Description(fmt.Sprintf("Path parameter: %s", p))))
		}

		// OData params (optional).
		for _, op := range odataParams {
			opts = append(opts, mcp.WithString(op, mcp.Description(fmt.Sprintf("OData query parameter $%s", op))))
		}

		// nextLink param.
		opts = append(opts, mcp.WithString("nextLink", mcp.Description("Pagination: pass @odata.nextLink URL to fetch the next page")))

		tool := mcp.NewTool(ep.ToolName, opts...)

		headers := ep.Headers // capture for closure

		s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// If nextLink is provided, use it directly.
			nextLink := req.GetString("nextLink", "")
			if nextLink != "" {
				u, err := url.Parse(nextLink)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf(`{"error":"invalid nextLink URL: %s"}`, err)), nil
				}
				nextPath := strings.Replace(u.Path, "/v1.0", "", 1) + "?" + u.RawQuery
				raw, err := gc.Get(nextPath, nil, headers)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
				}
				return mcp.NewToolResultText(formatJSON(raw)), nil
			}

			// Resolve path params.
			resolvedPath := ep.PathPattern
			for _, p := range pathParams {
				val := req.GetString(p, "")
				if val == "" {
					return mcp.NewToolResultError(fmt.Sprintf(`{"error":"Missing required parameter: %s"}`, p)), nil
				}
				resolvedPath = strings.Replace(resolvedPath, "{"+p+"}", url.PathEscape(val), 1)
			}

			// Collect OData query params.
			queryParams := make(map[string]string)
			for _, op := range odataParams {
				val := req.GetString(op, "")
				if val != "" {
					queryParams["$"+op] = val
				}
			}

			raw, err := gc.Get(resolvedPath, queryParams, headers)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
			}
			return mcp.NewToolResultText(formatJSON(raw)), nil
		})
		count++
	}

	if logger.Log != nil {
		logger.Log.Info(fmt.Sprintf("Registered %d endpoint-driven tools", count))
	}
}

// formatJSON pretty-prints a json.RawMessage.
func formatJSON(raw json.RawMessage) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(b)
}
