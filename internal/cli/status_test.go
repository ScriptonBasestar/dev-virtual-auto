package cli

import (
	"testing"
)

func TestParseComposePS_JSONArray(t *testing.T) {
	input := `[{"Name":"web","State":"running"},{"Name":"db","State":"running"}]`
	result := parseComposePS([]byte(input))
	arr, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(arr) != 2 {
		t.Errorf("len = %d, want 2", len(arr))
	}
}

func TestParseComposePS_JSONLines(t *testing.T) {
	input := "{\"Name\":\"web\",\"State\":\"running\"}\n{\"Name\":\"db\",\"State\":\"running\"}"
	result := parseComposePS([]byte(input))
	arr, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(arr) != 2 {
		t.Errorf("len = %d, want 2", len(arr))
	}
}

func TestParseComposePS_EmptyArray(t *testing.T) {
	input := `[]`
	result := parseComposePS([]byte(input))
	arr, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(arr) != 0 {
		t.Errorf("len = %d, want 0", len(arr))
	}
}

func TestParseComposePS_SingleLine(t *testing.T) {
	input := `{"Name":"redis","State":"running"}`
	result := parseComposePS([]byte(input))
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if m["Name"] != "redis" {
		t.Errorf("Name = %v, want redis", m["Name"])
	}
}

func TestParseComposePS_EmptyInput(t *testing.T) {
	result := parseComposePS([]byte(""))
	// Empty string falls through to JSON lines parser which returns empty slice
	arr, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(arr) != 0 {
		t.Errorf("len = %d, want 0", len(arr))
	}
}

func TestParseComposePS_JSONLinesWithBlanks(t *testing.T) {
	input := "{\"Name\":\"web\"}\n\n{\"Name\":\"db\"}\n"
	result := parseComposePS([]byte(input))
	arr, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(arr) != 2 {
		t.Errorf("len = %d, want 2", len(arr))
	}
}
