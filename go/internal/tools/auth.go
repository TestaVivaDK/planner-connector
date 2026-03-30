package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/TestaVivaDK/plannner-connector/internal/auth"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// jsonResult marshals v to indented JSON and returns a text tool result.
func jsonResult(v any) *mcp.CallToolResult {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("json marshal error: %s", err))
	}
	return mcp.NewToolResultText(string(b))
}

// jsonError returns an error tool result with a JSON-encoded error message.
func jsonError(msg string) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, msg))
}

func registerAuthTools(s *server.MCPServer, am *auth.AuthManager) {
	// planner-login
	loginTool := mcp.NewTool("planner-login",
		mcp.WithDescription("Authenticate with Microsoft. Opens the browser for sign-in and waits for completion."),
		mcp.WithBoolean("force", mcp.Description("Force a new login even if already logged in")),
	)
	s.AddTool(loginTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		force := req.GetBool("force", false)

		if !force {
			status, err := am.TestLogin()
			if err != nil {
				return jsonError(fmt.Sprintf("Auth failed: %s", err)), nil
			}
			if success, _ := status["success"].(bool); success {
				result := map[string]any{"status": "Already logged in"}
				for k, v := range status {
					result[k] = v
				}
				return jsonResult(result), nil
			}
		}

		err := am.AcquireTokenInteractively()
		if err != nil {
			return jsonError(fmt.Sprintf("Auth failed: %s", err)), nil
		}

		status, err := am.TestLogin()
		if err != nil {
			return jsonError(fmt.Sprintf("Auth failed: %s", err)), nil
		}

		if success, _ := status["success"].(bool); success {
			result := map[string]any{"status": "Login successful"}
			for k, v := range status {
				result[k] = v
			}
			return jsonResult(result), nil
		}

		return mcp.NewToolResultError(`{"status":"Login failed — the user may not have completed sign-in.","hint":"Call planner-login again to retry."}`), nil
	})

	// planner-logout
	logoutTool := mcp.NewTool("planner-logout",
		mcp.WithDescription("Log out from Microsoft"),
	)
	s.AddTool(logoutTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := am.Logout(); err != nil {
			return mcp.NewToolResultError(`{"error":"Logout failed"}`), nil
		}
		return jsonResult(map[string]string{"message": "Logged out"}), nil
	})

	// planner-auth-status
	statusTool := mcp.NewTool("planner-auth-status",
		mcp.WithDescription("Check Microsoft auth status"),
	)
	s.AddTool(statusTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := am.TestLogin()
		if err != nil {
			return jsonError(err.Error()), nil
		}
		return jsonResult(result), nil
	})
}
