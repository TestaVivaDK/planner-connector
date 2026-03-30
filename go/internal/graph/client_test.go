package graph

import (
	"testing"
)

// mockAuth implements TokenProvider for testing.
type mockAuth struct{}

func (m *mockAuth) GetToken() (string, error) {
	return "test-token", nil
}

func TestBuildURL(t *testing.T) {
	c := NewClient(&mockAuth{})
	got := c.buildURL("/me/planner/tasks", map[string]string{
		"$top":    "10",
		"$select": "id,title",
	})

	// Verify base
	if got[:len(c.baseURL)] != c.baseURL {
		t.Fatalf("URL does not start with base: %s", got)
	}
	// Verify path
	if !contains(got, "/me/planner/tasks") {
		t.Fatalf("URL missing path: %s", got)
	}
	// Verify query params are present (order may vary)
	if !contains(got, "$top=10") {
		t.Fatalf("URL missing $top param: %s", got)
	}
	if !contains(got, "$select=id%2Ctitle") {
		t.Fatalf("URL missing $select param: %s", got)
	}
	// Verify separator
	if !contains(got, "?") {
		t.Fatalf("URL missing ? separator: %s", got)
	}
}

func TestBuildURLNoParams(t *testing.T) {
	c := NewClient(&mockAuth{})
	got := c.buildURL("/me/planner/tasks", nil)
	expected := c.baseURL + "/me/planner/tasks"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
	// Ensure no trailing ? or &
	if contains(got, "?") || contains(got, "&") {
		t.Fatalf("URL should not contain query separator when no params: %s", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
