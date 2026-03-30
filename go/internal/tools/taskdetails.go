package tools

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"

	"github.com/TestaVivaDK/plannner-connector/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// newUUID generates a random v4 UUID string.
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func registerTaskDetailTools(s *server.MCPServer, gc *graph.Client) {
	// update-task-details
	updateDetails := mcp.NewTool("update-task-details",
		mcp.WithDescription("Update task details (description, checklist, references, previewType). ETag is auto-fetched if not provided."),
		mcp.WithString("task-id", mcp.Required(), mcp.Description("Task ID")),
		mcp.WithString("description", mcp.Description("Task description (plain text)")),
		mcp.WithString("previewType", mcp.Description(`"automatic", "noPreview", "checklist", "description", or "reference"`)),
		mcp.WithObject("checklist", mcp.Description("Checklist items keyed by GUID")),
		mcp.WithObject("references", mcp.Description("References keyed by URL (with special chars encoded)")),
		mcp.WithString("etag", mcp.Description("ETag (auto-fetched if omitted)")),
		mcp.WithDestructiveHintAnnotation(true),
	)
	s.AddTool(updateDetails, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID, err := req.RequireString("task-id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		etag := req.GetString("etag", "")
		if etag == "" {
			etag, err = gc.GetEtag("/planner/tasks/" + taskID + "/details")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
			}
		}
		body := make(map[string]any)
		args := req.GetArguments()
		if v, ok := args["description"]; ok {
			body["description"] = v
		}
		if v := req.GetString("previewType", ""); v != "" {
			body["previewType"] = v
		}
		if v, ok := args["checklist"]; ok {
			body["checklist"] = v
		}
		if v, ok := args["references"]; ok {
			body["references"] = v
		}
		raw, err := gc.Patch("/planner/tasks/"+taskID+"/details", body, etag)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		return mcp.NewToolResultText(formatJSON(raw)), nil
	})

	// add-checklist-item
	addChecklist := mcp.NewTool("add-checklist-item",
		mcp.WithDescription("Add a checklist item to a Planner task. Fetches current details, generates a GUID key, and adds the item."),
		mcp.WithString("task-id", mcp.Required(), mcp.Description("Task ID")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Checklist item text")),
		mcp.WithBoolean("isChecked", mcp.Description("Initial checked state (default: false)")),
		mcp.WithDestructiveHintAnnotation(true),
	)
	s.AddTool(addChecklist, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID, err := req.RequireString("task-id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		title, err := req.RequireString("title")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		isChecked := req.GetBool("isChecked", false)

		// Fetch current details for etag.
		detailsRaw, err := gc.Get("/planner/tasks/"+taskID+"/details", nil, nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		var details map[string]any
		if err := json.Unmarshal(detailsRaw, &details); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"parse details: %s"}`, err)), nil
		}
		etag, _ := details["@odata.etag"].(string)

		guid := newUUID()
		checklist := map[string]any{
			guid: map[string]any{
				"@odata.type": "microsoft.graph.plannerChecklistItem",
				"title":       title,
				"isChecked":   isChecked,
			},
		}
		raw, err := gc.Patch("/planner/tasks/"+taskID+"/details", map[string]any{"checklist": checklist}, etag)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		// Parse result and add the generated item ID.
		var result map[string]any
		if err := json.Unmarshal(raw, &result); err != nil {
			result = make(map[string]any)
		}
		result["addedItemId"] = guid
		return jsonResult(result), nil
	})

	// toggle-checklist-item
	toggleChecklist := mcp.NewTool("toggle-checklist-item",
		mcp.WithDescription("Toggle a checklist item's checked state on a Planner task."),
		mcp.WithString("task-id", mcp.Required(), mcp.Description("Task ID")),
		mcp.WithString("itemId", mcp.Required(), mcp.Description("Checklist item GUID key")),
		mcp.WithDestructiveHintAnnotation(true),
	)
	s.AddTool(toggleChecklist, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID, err := req.RequireString("task-id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		itemID, err := req.RequireString("itemId")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Fetch details.
		detailsRaw, err := gc.Get("/planner/tasks/"+taskID+"/details", nil, nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		var details map[string]any
		if err := json.Unmarshal(detailsRaw, &details); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"parse details: %s"}`, err)), nil
		}
		etag, _ := details["@odata.etag"].(string)

		// Find existing checklist item.
		checklistMap, _ := details["checklist"].(map[string]any)
		if checklistMap == nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"Checklist item %s not found"}`, itemID)), nil
		}
		existingItem, ok := checklistMap[itemID].(map[string]any)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"Checklist item %s not found"}`, itemID)), nil
		}

		// Toggle isChecked.
		currentChecked, _ := existingItem["isChecked"].(bool)
		existingItem["isChecked"] = !currentChecked
		existingItem["@odata.type"] = "microsoft.graph.plannerChecklistItem"

		checklist := map[string]any{
			itemID: existingItem,
		}
		raw, err := gc.Patch("/planner/tasks/"+taskID+"/details", map[string]any{"checklist": checklist}, etag)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		return mcp.NewToolResultText(formatJSON(raw)), nil
	})
}
