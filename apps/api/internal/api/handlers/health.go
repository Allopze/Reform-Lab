package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
)

// PublicHealth returns a minimal public health check.
func PublicHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status": "ok",
		})
	}
}

// DetailedHealth returns service policy and operational snapshots for admins.
func DetailedHealth(artifactTTLHours int, artifactTTLByFamily map[string]int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status": "ok",
			"retention": map[string]interface{}{
				"artifactTTLHours":         artifactTTLHours,
				"artifactTTLHoursByFamily": artifactTTLByFamily,
			},
			"featureFlags": capabilities.DefaultFlags.Snapshot(),
		})
	}
}

// respondJSON writes a JSON response.
func respondJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// respondError writes a JSON error response.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
