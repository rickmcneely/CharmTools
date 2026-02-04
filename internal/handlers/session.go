package handlers

import (
	"context"
	"net/http"
	"time"
)

const (
	sessionCookieName = "charmtool_session"
	sessionMaxAge     = 10 * 24 * 60 * 60 // 10 days in seconds
)

// contextKey is a custom type for context keys
type contextKey string

const sessionIDKey contextKey = "sessionID"

// SessionMiddleware handles session creation and validation
func (h *Handler) SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sessionID string

		// Check for existing session cookie
		cookie, err := r.Cookie(sessionCookieName)
		if err == nil && cookie.Value != "" {
			sessionID = cookie.Value
			// Verify session exists
			if !h.store.SessionExists(sessionID) {
				sessionID = ""
			}
		}

		// Create new session if needed
		if sessionID == "" {
			newID, err := h.store.CreateSession()
			if err != nil {
				http.Error(w, "Failed to create session", http.StatusInternalServerError)
				return
			}
			sessionID = newID

			// Set session cookie
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookieName,
				Value:    sessionID,
				Path:     "/",
				MaxAge:   sessionMaxAge,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
		} else {
			// Refresh cookie expiry
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookieName,
				Value:    sessionID,
				Path:     "/",
				MaxAge:   sessionMaxAge,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
			// Touch session to restart 10-day server-side expiry
			h.store.TouchSession(sessionID)
		}

		// Add session ID to context
		ctx := context.WithValue(r.Context(), sessionIDKey, sessionID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getSessionID retrieves the session ID from the request context
func getSessionID(r *http.Request) string {
	if id, ok := r.Context().Value(sessionIDKey).(string); ok {
		return id
	}
	return ""
}

// setCORSHeaders sets CORS headers for API responses
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// setJSONContentType sets the content type to JSON
func setJSONContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

// formatTime formats a time for display
func formatTime(t time.Time) string {
	return t.Format("2006/01/02 15:04:05")
}
