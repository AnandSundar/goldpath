package observability

import (
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// LogLevel represents the logging level
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// Logger provides a structured JSON logger using slog (Go 1.21+)
type Logger struct {
	logger *slog.Logger
	level  LogLevel
}

// NewLogger creates a new structured logger instance
func NewLogger() *Logger {
	levelStr := getEnv("GOLDPATH_LOG_LEVEL", "info")
	logLevel := parseLogLevel(levelStr)

	// Create JSON handler with options
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Keep all attributes as-is, including time, level, and message
			return a
		},
	})

	return &Logger{
		logger: slog.New(handler),
		level:  logLevel,
	}
}

// parseLogLevel converts string to slog.Level
func parseLogLevel(levelStr string) LogLevel {
	switch levelStr {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// toSlogLevel converts our LogLevel to slog.Level
func (l LogLevel) toSlogLevel() slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Level implements slog.Leveler interface
func (l LogLevel) Level() slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Info logs an info message
func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Info(msg, keysAndValues...)
}

// Error logs an error message
func (l *Logger) Error(msg string, keysAndValues ...interface{}) {
	l.logger.Error(msg, keysAndValues...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, keysAndValues ...interface{}) {
	l.logger.Warn(msg, keysAndValues...)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	l.logger.Debug(msg, keysAndValues...)
}

// With returns a new logger with the given attributes added
func (l *Logger) With(keysAndValues ...interface{}) *Logger {
	return &Logger{
		logger: l.logger.With(keysAndValues...),
		level:  l.level,
	}
}

// Metrics holds all application metrics
type Metrics struct {
	// HTTP metrics
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPRequestsInFlight *prometheus.GaugeVec

	// Feature flag metrics
	FlagEvaluationsTotal        *prometheus.CounterVec
	FlagRolloutEvaluationsTotal *prometheus.CounterVec
	FlagErrorsTotal             *prometheus.CounterVec

	// SLO metrics
	SLOChecksTotal   *prometheus.CounterVec
	SLOBreachesTotal *prometheus.CounterVec

	// SLO tracking (rolling window)
	SLOTracker *SLOTacker

	// AI metrics
	AIClientRequestsTotal   *prometheus.CounterVec
	AIClientRequestDuration *prometheus.HistogramVec

	// Scaffold metrics
	ScaffoldGenerationsTotal *prometheus.CounterVec
	ScaffoldErrorsTotal      *prometheus.CounterVec
}

// SLOTacker tracks SLO metrics with a rolling window
type SLOTacker struct {
	mu sync.RWMutex

	// SLO configuration
	sloTarget float64 // e.g., 0.995 for 99.5%
	window    time.Duration

	// Bucket-based tracking (5-minute buckets for 7 days = 2016 buckets)
	numBuckets      int
	bucketDuration  time.Duration
	requestsBuckets []int64 // total requests per bucket
	errorsBuckets   []int64 // errors per bucket
	bucketTimes     []time.Time

	// Prometheus gauges
	errorBudgetRemaining *prometheus.GaugeVec
	burnRate             *prometheus.GaugeVec
}

// NewSLOTacker creates a new SLO tracker with rolling window
func NewSLOTacker(sloTarget float64, window time.Duration) *SLOTacker {
	numBuckets := int(window.Minutes() / 5)
	if numBuckets < 1 {
		numBuckets = 1
	}

	tracker := &SLOTacker{
		sloTarget:       sloTarget,
		window:          window,
		numBuckets:      numBuckets,
		bucketDuration:  5 * time.Minute,
		requestsBuckets: make([]int64, numBuckets),
		errorsBuckets:   make([]int64, numBuckets),
		bucketTimes:     make([]time.Time, numBuckets),
	}

	// Initialize bucket times
	now := time.Now()
	for i := range tracker.bucketTimes {
		tracker.bucketTimes[i] = now
	}

	// Create Prometheus gauges
	errorBudgetRemaining := promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "goldpath_slo_error_budget_remaining",
			Help: "Remaining error budget percentage (0-100)",
		},
		[]string{"endpoint"},
	)

	burnRate := promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "goldpath_slo_burn_rate",
			Help: "SLO burn rate (1.0 = at budget pace, >1.0 = burning faster)",
		},
		[]string{"endpoint"},
	)

	tracker.errorBudgetRemaining = errorBudgetRemaining
	tracker.burnRate = burnRate

	return tracker
}

// RecordRequest records a request for SLO tracking
func (s *SLOTacker) RecordRequest(endpoint string, isError bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	bucketIndex := s.getBucketIndex(now)

	// Update bucket
	s.requestsBuckets[bucketIndex]++
	if isError {
		s.errorsBuckets[bucketIndex]++
	}
	s.bucketTimes[bucketIndex] = now

	// Update Prometheus metrics
	s.updateMetrics(endpoint)
}

// getBucketIndex returns the bucket index for a given time
func (s *SLOTacker) getBucketIndex(t time.Time) int {
	// Find the bucket that contains time t
	// Buckets are aligned to 5-minute boundaries
	elapsed := t.Sub(s.bucketTimes[0])
	bucket := int(elapsed / s.bucketDuration)

	// Wrap around if we've passed all buckets
	if bucket >= s.numBuckets {
		bucket = bucket % s.numBuckets
		// Reset the old bucket
		s.requestsBuckets[bucket] = 0
		s.errorsBuckets[bucket] = 0
	}

	return bucket
}

// getWindowStats returns total requests and errors in the rolling window
func (s *SLOTacker) getWindowStats() (totalRequests int64, totalErrors int64) {
	now := time.Now()
	cutoff := now.Add(-s.window)

	for i := 0; i < s.numBuckets; i++ {
		if s.bucketTimes[i].After(cutoff) {
			totalRequests += s.requestsBuckets[i]
			totalErrors += s.errorsBuckets[i]
		}
	}

	return totalRequests, totalErrors
}

// updateMetrics updates Prometheus gauges for an endpoint
func (s *SLOTacker) updateMetrics(endpoint string) {
	totalRequests, totalErrors := s.getWindowStats()

	if totalRequests == 0 {
		// No data yet, set default values
		s.errorBudgetRemaining.WithLabelValues(endpoint).Set(100.0)
		s.burnRate.WithLabelValues(endpoint).Set(0.0)
		return
	}

	// Calculate error rate
	errorRate := float64(totalErrors) / float64(totalRequests)

	// Calculate error budget remaining
	// Error budget = (sloTarget - actualSuccessRate) / (1 - sloTarget) * 100
	// Or simpler: 100 - (errorRate / (1-sloTarget) * 100)
	allowedErrorRate := 1.0 - s.sloTarget
	errorBudgetUsed := errorRate / allowedErrorRate
	errorBudgetRemaining := 100.0 * (1.0 - errorBudgetUsed)

	// Clamp to 0-100
	if errorBudgetRemaining < 0 {
		errorBudgetRemaining = 0
	} else if errorBudgetRemaining > 100 {
		errorBudgetRemaining = 100
	}

	// Calculate burn rate
	// Burn rate = actual error rate / allowed error rate
	burnRate := errorRate / allowedErrorRate

	s.errorBudgetRemaining.WithLabelValues(endpoint).Set(errorBudgetRemaining)
	s.burnRate.WithLabelValues(endpoint).Set(burnRate)
}

// GetErrorBudgetRemaining returns the current error budget remaining percentage
func (s *SLOTacker) GetErrorBudgetRemaining(endpoint string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalRequests, totalErrors := s.getWindowStats()
	if totalRequests == 0 {
		return 100.0
	}

	errorRate := float64(totalErrors) / float64(totalRequests)
	allowedErrorRate := 1.0 - s.sloTarget
	errorBudgetUsed := errorRate / allowedErrorRate
	errorBudgetRemaining := 100.0 * (1.0 - errorBudgetUsed)

	if errorBudgetRemaining < 0 {
		return 0
	}
	return errorBudgetRemaining
}

// GetBurnRate returns the current burn rate
func (s *SLOTacker) GetBurnRate(endpoint string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalRequests, totalErrors := s.getWindowStats()
	if totalRequests == 0 {
		return 0.0
	}

	errorRate := float64(totalErrors) / float64(totalRequests)
	allowedErrorRate := 1.0 - s.sloTarget

	if allowedErrorRate <= 0 {
		return 0.0
	}

	return errorRate / allowedErrorRate
}

// NewMetrics creates and registers all application metrics
func NewMetrics() *Metrics {
	m := &Metrics{
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "goldpath_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "goldpath_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		HTTPRequestsInFlight: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "goldpath_http_requests_in_flight",
				Help: "Number of HTTP requests currently being processed",
			},
			[]string{"method", "path"},
		),
		FlagEvaluationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "goldpath_flag_evaluations_total",
				Help: "Total number of feature flag evaluations",
			},
			[]string{"flag_key", "enabled"},
		),
		FlagRolloutEvaluationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "goldpath_flag_rollout_evaluations_total",
				Help: "Total number of feature flag evaluations with rollout",
			},
			[]string{"flag_key", "enabled", "in_rollout", "rollout_percentage"},
		),
		FlagErrorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "goldpath_flag_errors_total",
				Help: "Total number of feature flag errors",
			},
			[]string{"flag_key", "error_type"},
		),
		SLOChecksTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "goldpath_slo_checks_total",
				Help: "Total number of SLO checks",
			},
			[]string{"slo_name"},
		),
		SLOBreachesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "goldpath_slo_breaches_total",
				Help: "Total number of SLO breaches",
			},
			[]string{"slo_name"},
		),
		AIClientRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "goldpath_ai_requests_total",
				Help: "Total number of AI client requests",
			},
			[]string{"model", "error"},
		),
		AIClientRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "goldpath_ai_request_duration_seconds",
				Help:    "AI request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"model"},
		),
		ScaffoldGenerationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "goldpath_scaffold_generations_total",
				Help: "Total number of scaffold generations",
			},
			[]string{"template"},
		),
		ScaffoldErrorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "goldpath_scaffold_errors_total",
				Help: "Total number of scaffold generation errors",
			},
			[]string{"template", "error_type"},
		),
	}

	// Initialize SLO tracker with 7-day rolling window
	// Default SLO target is 99.5% (from GOLDPATH_SLO_THRESHOLD env var, default 0.995)
	sloThreshold := getEnvFloat64("GOLDPATH_SLO_THRESHOLD", 0.995)
	m.SLOTracker = NewSLOTacker(sloThreshold, 7*24*time.Hour)

	return m
}

// RecordHTTPRequest records an HTTP request
func (m *Metrics) RecordHTTPRequest(method, path string, status int, duration time.Duration) {
	m.HTTPRequestsTotal.WithLabelValues(method, path, statusToString(status)).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())

	// Track SLO request (consider 5xx as errors)
	isError := status >= 500
	if m.SLOTracker != nil {
		m.SLOTracker.RecordRequest(path, isError)
		m.SLOTracker.RecordRequest("overall", isError)
	}
}

// RecordFlagEvaluation records a feature flag evaluation
func (m *Metrics) RecordFlagEvaluation(flagKey string, enabled bool) {
	m.FlagEvaluationsTotal.WithLabelValues(flagKey, boolToString(enabled)).Inc()
}

// RecordFlagEvaluationWithRollout records a feature flag evaluation with rollout information
func (m *Metrics) RecordFlagEvaluationWithRollout(flagKey string, enabled, inRollout bool, rolloutPercentage float64) {
	m.FlagRolloutEvaluationsTotal.WithLabelValues(flagKey, boolToString(enabled), boolToString(inRollout), floatToString(rolloutPercentage)).Inc()
}

// RecordFlagError records a feature flag error
func (m *Metrics) RecordFlagError(flagKey, errorType string) {
	m.FlagErrorsTotal.WithLabelValues(flagKey, errorType).Inc()
}

// RecordSLOCheck records an SLO check
func (m *Metrics) RecordSLOCheck(sloName string, success bool) {
	m.SLOChecksTotal.WithLabelValues(sloName).Inc()
	if !success {
		m.SLOBreachesTotal.WithLabelValues(sloName).Inc()
	}
}

// RecordSLORequest records a request for SLO tracking with rolling window
// endpoint parameter can be "overall" for aggregate tracking or a specific endpoint path
func (m *Metrics) RecordSLORequest(endpoint string, isError bool) {
	if m.SLOTracker != nil {
		m.SLOTracker.RecordRequest(endpoint, isError)
	}
}

// RecordAIRequest records an AI request
func (m *Metrics) RecordAIRequest(model string, err error, duration time.Duration) {
	errorStr := "false"
	if err != nil {
		errorStr = "true"
	}
	m.AIClientRequestsTotal.WithLabelValues(model, errorStr).Inc()
	m.AIClientRequestDuration.WithLabelValues(model).Observe(duration.Seconds())
}

// RecordScaffoldGeneration records a scaffold generation
func (m *Metrics) RecordScaffoldGeneration(template string, err error) {
	m.ScaffoldGenerationsTotal.WithLabelValues(template).Inc()
	if err != nil {
		m.ScaffoldErrorsTotal.WithLabelValues(template, "generation_error").Inc()
	}
}

func statusToString(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	case status >= 200:
		return "2xx"
	default:
		return "other"
	}
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func floatToString(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvFloat64 retrieves an environment variable as float64 or returns a default value
func getEnvFloat64(key string, defaultValue float64) float64 {
	if value, exists := os.LookupEnv(key); exists {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}
