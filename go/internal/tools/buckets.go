package tools

import (
	"context"
	"fmt"

	"github.com/TestaVivaDK/plannner-connector/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerBucketTools(s *server.MCPServer, gc *graph.Client) {
	// create-bucket
	createBucket := mcp.NewTool("create-bucket",
		mcp.WithDescription("Create a new bucket in a Planner plan."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Bucket name")),
		mcp.WithString("planId", mcp.Required(), mcp.Description("Plan ID to create the bucket in")),
		mcp.WithString("orderHint", mcp.Description(`Order hint for positioning (use " !" for first position)`)),
		mcp.WithDestructiveHintAnnotation(true),
	)
	s.AddTool(createBucket, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := req.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		planID, err := req.RequireString("planId")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body := map[string]any{"name": name, "planId": planID}
		if v := req.GetString("orderHint", ""); v != "" {
			body["orderHint"] = v
		}
		raw, err := gc.Post("/planner/buckets", body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		return mcp.NewToolResultText(formatJSON(raw)), nil
	})

	// update-bucket
	updateBucket := mcp.NewTool("update-bucket",
		mcp.WithDescription("Update a Planner bucket (name, orderHint). ETag is auto-fetched if not provided."),
		mcp.WithString("bucket-id", mcp.Required(), mcp.Description("Bucket ID")),
		mcp.WithString("name", mcp.Description("New bucket name")),
		mcp.WithString("orderHint", mcp.Description("New order hint")),
		mcp.WithString("etag", mcp.Description("ETag (auto-fetched if omitted)")),
		mcp.WithDestructiveHintAnnotation(true),
	)
	s.AddTool(updateBucket, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		bucketID, err := req.RequireString("bucket-id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		etag := req.GetString("etag", "")
		if etag == "" {
			etag, err = gc.GetEtag("/planner/buckets/" + bucketID)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
			}
		}
		body := make(map[string]any)
		if v := req.GetString("name", ""); v != "" {
			body["name"] = v
		}
		if v := req.GetString("orderHint", ""); v != "" {
			body["orderHint"] = v
		}
		raw, err := gc.Patch("/planner/buckets/"+bucketID, body, etag)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		return mcp.NewToolResultText(formatJSON(raw)), nil
	})

	// delete-bucket
	deleteBucket := mcp.NewTool("delete-bucket",
		mcp.WithDescription("Delete a Planner bucket. ETag is auto-fetched if not provided."),
		mcp.WithString("bucket-id", mcp.Required(), mcp.Description("Bucket ID")),
		mcp.WithString("etag", mcp.Description("ETag (auto-fetched if omitted)")),
		mcp.WithDestructiveHintAnnotation(true),
	)
	s.AddTool(deleteBucket, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		bucketID, err := req.RequireString("bucket-id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		etag := req.GetString("etag", "")
		if etag == "" {
			etag, err = gc.GetEtag("/planner/buckets/" + bucketID)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
			}
		}
		_, err = gc.Delete("/planner/buckets/"+bucketID, etag)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		return jsonResult(map[string]any{"success": true, "message": "Bucket deleted"}), nil
	})
}
