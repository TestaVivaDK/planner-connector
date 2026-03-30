package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/TestaVivaDK/plannner-connector/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTaskTools(s *server.MCPServer, gc *graph.Client) {
	// create-task
	createTask := mcp.NewTool("create-task",
		mcp.WithDescription("Create a new Planner task."),
		mcp.WithString("planId", mcp.Required(), mcp.Description("Plan ID")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Task title")),
		mcp.WithString("bucketId", mcp.Description("Bucket ID (task goes to default bucket if omitted)")),
		mcp.WithArray("assigneeIds", mcp.Description("Array of user IDs to assign")),
		mcp.WithNumber("priority", mcp.Description("Priority: 0=Urgent, 1=Important, 2=Medium, 3+=Low")),
		mcp.WithString("startDateTime", mcp.Description("Start date in ISO 8601 format")),
		mcp.WithString("dueDateTime", mcp.Description("Due date in ISO 8601 format")),
		mcp.WithNumber("percentComplete", mcp.Description("Completion: 0=Not started, 50=In progress, 100=Complete")),
		mcp.WithString("orderHint", mcp.Description("Order hint for positioning")),
		mcp.WithDestructiveHintAnnotation(true),
	)
	s.AddTool(createTask, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		planID, err := req.RequireString("planId")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		title, err := req.RequireString("title")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body := map[string]any{"planId": planID, "title": title}
		if v := req.GetString("bucketId", ""); v != "" {
			body["bucketId"] = v
		}
		args := req.GetArguments()
		if v, ok := args["priority"]; ok {
			body["priority"] = v
		}
		if v := req.GetString("startDateTime", ""); v != "" {
			body["startDateTime"] = v
		}
		if v := req.GetString("dueDateTime", ""); v != "" {
			body["dueDateTime"] = v
		}
		if v, ok := args["percentComplete"]; ok {
			body["percentComplete"] = v
		}
		if v := req.GetString("orderHint", ""); v != "" {
			body["orderHint"] = v
		}
		// Build assignments from assigneeIds.
		assigneeIds := req.GetStringSlice("assigneeIds", nil)
		if len(assigneeIds) > 0 {
			assignments := make(map[string]any)
			for _, uid := range assigneeIds {
				assignments[uid] = map[string]any{
					"@odata.type": "#microsoft.graph.plannerAssignment",
					"orderHint":   " !",
				}
			}
			body["assignments"] = assignments
		}
		raw, err := gc.Post("/planner/tasks", body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		return mcp.NewToolResultText(formatJSON(raw)), nil
	})

	// update-task
	updateTask := mcp.NewTool("update-task",
		mcp.WithDescription("Update a Planner task. ETag is auto-fetched if not provided."),
		mcp.WithString("task-id", mcp.Required(), mcp.Description("Task ID")),
		mcp.WithString("title", mcp.Description("New title")),
		mcp.WithString("bucketId", mcp.Description("Move to different bucket")),
		mcp.WithNumber("priority", mcp.Description("Priority: 0=Urgent, 1=Important, 2=Medium, 3+=Low")),
		mcp.WithString("startDateTime", mcp.Description("Start date ISO 8601")),
		mcp.WithString("dueDateTime", mcp.Description("Due date ISO 8601")),
		mcp.WithNumber("percentComplete", mcp.Description("0=Not started, 50=In progress, 100=Complete")),
		mcp.WithObject("appliedCategories", mcp.Description(`Categories, e.g. {"category1": true}`)),
		mcp.WithString("orderHint", mcp.Description("Order hint")),
		mcp.WithString("etag", mcp.Description("ETag (auto-fetched if omitted)")),
		mcp.WithDestructiveHintAnnotation(true),
	)
	s.AddTool(updateTask, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID, err := req.RequireString("task-id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		etag := req.GetString("etag", "")
		if etag == "" {
			etag, err = gc.GetEtag("/planner/tasks/" + taskID)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
			}
		}
		body := make(map[string]any)
		args := req.GetArguments()
		if v := req.GetString("title", ""); v != "" {
			body["title"] = v
		}
		if v := req.GetString("bucketId", ""); v != "" {
			body["bucketId"] = v
		}
		if v, ok := args["priority"]; ok {
			body["priority"] = v
		}
		if v := req.GetString("startDateTime", ""); v != "" {
			body["startDateTime"] = v
		}
		if v := req.GetString("dueDateTime", ""); v != "" {
			body["dueDateTime"] = v
		}
		if v, ok := args["percentComplete"]; ok {
			body["percentComplete"] = v
		}
		if v, ok := args["appliedCategories"]; ok {
			body["appliedCategories"] = v
		}
		if v := req.GetString("orderHint", ""); v != "" {
			body["orderHint"] = v
		}
		raw, err := gc.Patch("/planner/tasks/"+taskID, body, etag)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		return mcp.NewToolResultText(formatJSON(raw)), nil
	})

	// delete-task
	deleteTask := mcp.NewTool("delete-task",
		mcp.WithDescription("Delete a Planner task. ETag is auto-fetched if not provided."),
		mcp.WithString("task-id", mcp.Required(), mcp.Description("Task ID")),
		mcp.WithString("etag", mcp.Description("ETag (auto-fetched if omitted)")),
		mcp.WithDestructiveHintAnnotation(true),
	)
	s.AddTool(deleteTask, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID, err := req.RequireString("task-id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		etag := req.GetString("etag", "")
		if etag == "" {
			etag, err = gc.GetEtag("/planner/tasks/" + taskID)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
			}
		}
		_, err = gc.Delete("/planner/tasks/"+taskID, etag)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		return jsonResult(map[string]any{"success": true, "message": "Task deleted"}), nil
	})

	// assign-task
	assignTask := mcp.NewTool("assign-task",
		mcp.WithDescription("Assign a user to a Planner task. Fetches current task, merges assignment, and updates."),
		mcp.WithString("task-id", mcp.Required(), mcp.Description("Task ID")),
		mcp.WithString("userId", mcp.Required(), mcp.Description("User ID to assign")),
		mcp.WithDestructiveHintAnnotation(true),
	)
	s.AddTool(assignTask, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID, err := req.RequireString("task-id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		userID, err := req.RequireString("userId")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		// Fetch current task to get etag and existing assignments.
		taskRaw, err := gc.Get("/planner/tasks/"+taskID, nil, nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		var task map[string]any
		if err := json.Unmarshal(taskRaw, &task); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"parse task: %s"}`, err)), nil
		}
		etag, _ := task["@odata.etag"].(string)
		assignments, _ := task["assignments"].(map[string]any)
		if assignments == nil {
			assignments = make(map[string]any)
		}
		assignments[userID] = map[string]any{
			"@odata.type": "#microsoft.graph.plannerAssignment",
			"orderHint":   " !",
		}
		raw, err := gc.Patch("/planner/tasks/"+taskID, map[string]any{"assignments": assignments}, etag)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		return mcp.NewToolResultText(formatJSON(raw)), nil
	})

	// unassign-task
	unassignTask := mcp.NewTool("unassign-task",
		mcp.WithDescription("Remove a user assignment from a Planner task."),
		mcp.WithString("task-id", mcp.Required(), mcp.Description("Task ID")),
		mcp.WithString("userId", mcp.Required(), mcp.Description("User ID to unassign")),
		mcp.WithDestructiveHintAnnotation(true),
	)
	s.AddTool(unassignTask, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID, err := req.RequireString("task-id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		userID, err := req.RequireString("userId")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		// Fetch task for etag.
		taskRaw, err := gc.Get("/planner/tasks/"+taskID, nil, nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		var task map[string]any
		if err := json.Unmarshal(taskRaw, &task); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"parse task: %s"}`, err)), nil
		}
		etag, _ := task["@odata.etag"].(string)
		// Set the user's assignment to nil to remove it.
		assignments := map[string]any{userID: nil}
		raw, err := gc.Patch("/planner/tasks/"+taskID, map[string]any{"assignments": assignments}, etag)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		return mcp.NewToolResultText(formatJSON(raw)), nil
	})

	// move-task
	moveTask := mcp.NewTool("move-task",
		mcp.WithDescription("Move a Planner task to a different bucket."),
		mcp.WithString("task-id", mcp.Required(), mcp.Description("Task ID")),
		mcp.WithString("bucketId", mcp.Required(), mcp.Description("Target bucket ID")),
		mcp.WithDestructiveHintAnnotation(true),
	)
	s.AddTool(moveTask, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID, err := req.RequireString("task-id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		bucketID, err := req.RequireString("bucketId")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		// Fetch task for etag.
		taskRaw, err := gc.Get("/planner/tasks/"+taskID, nil, nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		var task map[string]any
		if err := json.Unmarshal(taskRaw, &task); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"parse task: %s"}`, err)), nil
		}
		etag, _ := task["@odata.etag"].(string)
		raw, err := gc.Patch("/planner/tasks/"+taskID, map[string]any{"bucketId": bucketID}, etag)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(`{"error":"%s"}`, err)), nil
		}
		return mcp.NewToolResultText(formatJSON(raw)), nil
	})
}
