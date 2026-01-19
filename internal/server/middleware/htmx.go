package middleware

import (
	"context"
	"net/http"
)

type contextKey string

const htmxKey contextKey = "htmx"

// HTMXRequest holds HTMX-specific request information.
type HTMXRequest struct {
	IsHTMX       bool
	IsBoosted    bool
	CurrentURL   string
	Target       string
	TriggerName  string
	TriggerID    string
	HistoryRestore bool
}

// HTMX middleware parses HTMX headers.
func HTMX(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		htmx := &HTMXRequest{
			IsHTMX:         r.Header.Get("HX-Request") == "true",
			IsBoosted:      r.Header.Get("HX-Boosted") == "true",
			CurrentURL:     r.Header.Get("HX-Current-URL"),
			Target:         r.Header.Get("HX-Target"),
			TriggerName:    r.Header.Get("HX-Trigger-Name"),
			TriggerID:      r.Header.Get("HX-Trigger"),
			HistoryRestore: r.Header.Get("HX-History-Restore-Request") == "true",
		}

		ctx := context.WithValue(r.Context(), htmxKey, htmx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetHTMX retrieves HTMX request info from context.
func GetHTMX(r *http.Request) *HTMXRequest {
	if htmx, ok := r.Context().Value(htmxKey).(*HTMXRequest); ok {
		return htmx
	}
	return &HTMXRequest{}
}

// IsHTMXRequest checks if the request is from HTMX.
func IsHTMXRequest(r *http.Request) bool {
	return GetHTMX(r).IsHTMX
}
