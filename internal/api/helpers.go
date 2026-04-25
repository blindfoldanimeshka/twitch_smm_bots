package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Bot status constants
const (
	BotStatusRunning = "running"
	BotStatusStopped = "stopped"
)

// withTimeout creates a context with timeout from parent context
func withTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

// respondError writes an error response in JSON format
func respondError(w http.ResponseWriter, message string, status int) {
	http.Error(w, fmt.Sprintf(`{"error":"%s"}`, message), status)
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(data)
}

// handleStoreError handles common storage errors and returns true if error was handled
func handleStoreError(w http.ResponseWriter, err error, notFoundMsg string) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "not found") {
		respondError(w, notFoundMsg, http.StatusNotFound)
	} else {
		respondError(w, "operation failed", http.StatusInternalServerError)
	}
	return true
}
