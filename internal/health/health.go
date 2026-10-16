package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
)

var startTime = time.Now()

type HealthHandler struct {
	db *sqlx.DB
}

func NewHealthHandler(db *sqlx.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

type HealthResponse struct {
	Status        string  `json:"status"`
	Database      string  `json:"database"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	Timestamp     string  `json:"timestamp"`
	Version       string  `json:"version"`
	Error         string  `json:"error,omitempty"`
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	w.Header().Set("Content-Type", "application/json")

	uptime := time.Since(startTime).Seconds()
	resp := HealthResponse{
		Status:        "healthy",
		Database:      "connected",
		UptimeSeconds: uptime,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Version:       "1.0.0",
	}

	if err := h.db.PingContext(ctx); err != nil {
		resp.Status = "degraded"
		resp.Database = "disconnected"
		resp.Error = err.Error()
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(resp)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
