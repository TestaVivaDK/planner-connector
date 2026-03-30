package tools

import (
	"context"
	"fmt"

	"github.com/TestaVivaDK/plannner-connector/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerPlanTools(s *server.MCPServer, gc *graph.Client) {
	// create-plan
	createPlan := mcp.NewTool("create-plan",
		mcp.WithDescription("Create a new Planner plan. The owner must be a Microsoft 365 Group ID."),
		mcp.WithString("title", mcp.Required(), mcp.Description("Plan title")),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Group ID that owns the plan (use list-groups to find)")),
		mcp.WithDestructiveHintAnnotation(true),
	)
	s.AddTool(createPlan, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title, err := req.RequireString("title")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		owner, err := req.RequireString("owner")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		raw, err := gc.Post("/planner/plans", map[string]any{"title": title, "owner": owner})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		return mcp.NewToolResultText(formatJSON(raw)), nil
	})

	// update-plan
	updatePlan := mcp.NewTool("update-plan",
		mcp.WithDescription("Update a Planner plan (title, category descriptions). ETag is auto-fetched if not provided."),
		mcp.WithString("plan-id", mcp.Required(), mcp.Description("Plan ID")),
		mcp.WithString("title", mcp.Description("New plan title")),
		mcp.WithObject("categoryDescriptions", mcp.Description(`Category label descriptions, e.g. {"category1": "Urgent", "category2": "Bug"}`)),
		mcp.WithString("etag", mcp.Description("ETag for optimistic concurrency (auto-fetched if omitted)")),
		mcp.WithDestructiveHintAnnotation(true),
	)
	s.AddTool(updatePlan, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		planID, err := req.RequireString("plan-id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		etag := req.GetString("etag", "")
		if etag == "" {
			etag, err = gc.GetEtag("/planner/plans/" + planID)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
			}
		}
		body := make(map[string]any)
		if v := req.GetString("title", ""); v != "" {
			body["title"] = v
		}
		args := req.GetArguments()
		if v, ok := args["categoryDescriptions"]; ok {
			body["categoryDescriptions"] = v
		}
		raw, err := gc.Patch("/planner/plans/"+planID, body, etag)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		return mcp.NewToolResultText(formatJSON(raw)), nil
	})

	// delete-plan
	deletePlan := mcp.NewTool("delete-plan",
		mcp.WithDescription("Delete a Planner plan. ETag is auto-fetched if not provided."),
		mcp.WithString("plan-id", mcp.Required(), mcp.Description("Plan ID")),
		mcp.WithString("etag", mcp.Description("ETag for optimistic concurrency (auto-fetched if omitted)")),
		mcp.WithDestructiveHintAnnotation(true),
	)
	s.AddTool(deletePlan, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		planID, err := req.RequireString("plan-id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		etag := req.GetString("etag", "")
		if etag == "" {
			etag, err = gc.GetEtag("/planner/plans/" + planID)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
			}
		}
		_, err = gc.Delete("/planner/plans/"+planID, etag)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		return jsonResult(map[string]any{"success": true, "message": "Plan deleted"}), nil
	})
}
