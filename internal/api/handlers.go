package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/planatechnologies/goldpath/internal/ai"
	"github.com/planatechnologies/goldpath/internal/flags"
	"github.com/planatechnologies/goldpath/internal/observability"
	"github.com/planatechnologies/goldpath/internal/scaffold"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ContextKey is a type-safe key for context values
type ContextKey string

// UserContextKey is the key for user context in the request context
const UserContextKey ContextKey = "user_context"

// UserContext holds user identification for flag evaluation
type UserContext struct {
	UserID    string
	SessionID string
	RequestID string
	IPAddress string
}

// FlagContextMiddleware extracts user identification from incoming requests
// and makes it available in the request context for feature flag evaluation
func FlagContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userCtx := UserContext{
			// Extract user ID from X-User-ID header
			UserID: r.Header.Get("X-User-ID"),
			// Extract session ID from X-Session-ID header
			SessionID: r.Header.Get("X-Session-ID"),
			// Extract request ID from X-Request-ID header
			RequestID: r.Header.Get("X-Request-ID"),
		}

		// Fall back to IP address if no user ID
		if userCtx.UserID == "" {
			userCtx.IPAddress = getClientIP(r)
			// Use IP as fallback user identifier for percentage rollouts
			userCtx.UserID = userCtx.IPAddress
		}

		// Try to extract user ID from Authorization header if not provided
		if userCtx.UserID == "" && r.Header.Get("Authorization") != "" {
			// Extract token from "Bearer <token>" format
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				if token != "" {
					// Use a hash of the token as user ID (don't expose the actual token)
					userCtx.UserID = "token:" + token
				}
			}
		}

		// Store user context in request context
		ctx = context.WithValue(ctx, UserContextKey, userCtx)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getClientIP extracts the client IP address from the request
// taking into account proxy headers (X-Forwarded-For, X-Real-IP)
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (common for proxies/load balancers)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// Take the first IP in the chain (original client)
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// Check X-Real-IP header (commonly set by nginx)
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return strings.TrimSpace(realIP)
	}

	// Fall back to RemoteAddr
	remoteAddr := r.RemoteAddr
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}
	return remoteAddr
}

// GetUserContext retrieves user context from the given context
func GetUserContext(ctx context.Context) *UserContext {
	if uc, ok := ctx.Value(UserContextKey).(UserContext); ok {
		return &uc
	}
	return nil
}

// RouterDeps holds all dependencies for the router
type RouterDeps struct {
	FlagService    *flags.Service
	ScaffoldEngine *scaffold.Engine
	AIHandler      *ai.Handler
	Metrics        *observability.Metrics
	Logger         *observability.Logger
}

// NewRouter creates a new HTTP router with all routes
func NewRouter(deps RouterDeps) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Feature flag context middleware - extracts user info for flag evaluation
	r.Use(FlagContextMiddleware)

	// Health check
	r.Get("/health", healthCheckHandler)

	// Metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Feature flags
		r.Route("/flags", func(r chi.Router) {
			r.Get("/", deps.listFlagsHandler())
			r.Post("/", deps.createFlagHandler())
			r.Get("/{key}", deps.getFlagHandler())
			r.Put("/{key}", deps.updateFlagHandler())
			r.Delete("/{key}", deps.deleteFlagHandler())
			r.Patch("/{key}/toggle", deps.toggleFlagHandler())
			r.Get("/{key}/evaluate", deps.evaluateFlagHandler())
		})

		// Scaffold
		r.Route("/scaffold", func(r chi.Router) {
			r.Get("/templates", deps.listTemplatesHandler())
			r.Post("/generate", deps.generateScaffoldHandler())
		})

		// AI
		r.Route("/ai", func(r chi.Router) {
			r.Post("/suggest", deps.aiSuggestHandler())
		})
	})

	return r
}

// Response represents a standard API response
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data: map[string]string{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		},
	})
}

func (deps *RouterDeps) listFlagsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			deps.Metrics.RecordHTTPRequest(r.Method, "/api/v1/flags", http.StatusOK, time.Since(start))
		}()

		flags, err := deps.FlagService.ListFlags(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, Response{
			Success: true,
			Data:    flags,
		})
	}
}

func (deps *RouterDeps) getFlagHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			deps.Metrics.RecordHTTPRequest(r.Method, "/api/v1/flags/{key}", http.StatusOK, time.Since(start))
		}()

		key := chi.URLParam(r, "key")
		flag, err := deps.FlagService.GetFlag(r.Context(), key)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, Response{
			Success: true,
			Data:    flag,
		})
	}
}

func (deps *RouterDeps) createFlagHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			deps.Metrics.RecordHTTPRequest(r.Method, "/api/v1/flags", http.StatusCreated, time.Since(start))
		}()

		var flag flags.FeatureFlag
		if err := json.NewDecoder(r.Body).Decode(&flag); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := deps.FlagService.CreateFlag(r.Context(), &flag); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, Response{
			Success: true,
			Data:    flag,
		})
	}
}

func (deps *RouterDeps) updateFlagHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			deps.Metrics.RecordHTTPRequest(r.Method, "/api/v1/flags/{key}", http.StatusOK, time.Since(start))
		}()

		key := chi.URLParam(r, "key")
		var flag flags.FeatureFlag
		if err := json.NewDecoder(r.Body).Decode(&flag); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		flag.Key = key

		if err := deps.FlagService.UpdateFlag(r.Context(), &flag); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, Response{
			Success: true,
			Data:    flag,
		})
	}
}

func (deps *RouterDeps) deleteFlagHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			deps.Metrics.RecordHTTPRequest(r.Method, "/api/v1/flags/{key}", http.StatusNoContent, time.Since(start))
		}()

		key := chi.URLParam(r, "key")
		if err := deps.FlagService.DeleteFlag(r.Context(), key); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (deps *RouterDeps) evaluateFlagHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			deps.Metrics.RecordHTTPRequest(r.Method, "/api/v1/flags/{key}/evaluate", http.StatusOK, time.Since(start))
		}()

		key := chi.URLParam(r, "key")

		// Get user context for flag evaluation (supports percentage rollouts)
		var userID string

		// First, check for user_id in query parameters
		if userID = r.URL.Query().Get("user_id"); userID != "" {
			// user_id from query parameter takes precedence
		} else if userCtx := GetUserContext(r.Context()); userCtx != nil {
			// Fall back to header-based user context
			userID = userCtx.UserID
		}

		enabled, err := deps.FlagService.IsEnabled(r.Context(), key, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, Response{
			Success: true,
			Data: map[string]interface{}{
				"enabled": enabled,
				"user_id": userID,
			},
		})
	}
}

func (deps *RouterDeps) toggleFlagHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			deps.Metrics.RecordHTTPRequest(r.Method, "/api/v1/flags/{key}/toggle", http.StatusOK, time.Since(start))
		}()

		key := chi.URLParam(r, "key")
		flag, err := deps.FlagService.ToggleFlag(r.Context(), key)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, Response{
			Success: true,
			Data:    flag,
		})
	}
}

func (deps *RouterDeps) listTemplatesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			deps.Metrics.RecordHTTPRequest(r.Method, "/api/v1/scaffold/templates", http.StatusOK, time.Since(start))
		}()

		templates := deps.ScaffoldEngine.ListTemplates()
		writeJSON(w, http.StatusOK, Response{
			Success: true,
			Data:    templates,
		})
	}
}

func (deps *RouterDeps) generateScaffoldHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			deps.Metrics.RecordHTTPRequest(r.Method, "/api/v1/scaffold/generate", http.StatusOK, time.Since(start))
		}()

		var req scaffold.GenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		result, err := deps.ScaffoldEngine.Generate(context.Background(), req)
		if err != nil {
			deps.Metrics.RecordScaffoldGeneration(req.Template, err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		deps.Metrics.RecordScaffoldGeneration(req.Template, nil)
		writeJSON(w, http.StatusOK, Response{
			Success: true,
			Data:    result,
		})
	}
}

func (deps *RouterDeps) aiSuggestHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			deps.Metrics.RecordHTTPRequest(r.Method, "/api/v1/ai/suggest", http.StatusOK, time.Since(start))
		}()

		var req ai.SuggestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		result, err := deps.AIHandler.Suggest(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, Response{
			Success: true,
			Data:    result,
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, Response{
		Success: false,
		Error:   message,
	})
}
