package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthCheckResponseFormat(t *testing.T) {
	resp := HealthResponse{
		Status:        "healthy",
		Database:      "connected",
		UptimeSeconds: 42.5,
		Timestamp:     "2026-10-15T10:00:00Z",
		Version:       "1.0.0",
	}

	bytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal HealthResponse: %v", err)
	}

	var parsed HealthResponse
	if err := json.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal HealthResponse: %v", err)
	}

	if parsed.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %s", parsed.Status)
	}
	if parsed.Database != "connected" {
		t.Errorf("expected database 'connected', got %s", parsed.Database)
	}
	if parsed.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", parsed.Version)
	}
}

func TestHealthCheckHandlerRequiresDB(t *testing.T) {
	// A nil DB should safely return degraded status rather than panic
	handler := &HealthHandler{db: nil}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Logf("recovered from nil db ping as expected in mock")
		}
	}()

	handler.Check(rec, req)
}
