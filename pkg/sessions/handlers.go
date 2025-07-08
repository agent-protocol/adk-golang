// Package sessions provides event handler implementations.
package sessions

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
)

// LoggingEventHandler logs session lifecycle events.
type LoggingEventHandler struct {
	logger Logger
}

// Logger interface for structured logging.
type Logger interface {
	Info(msg string, fields ...any)
	Error(msg string, err error, fields ...any)
	Debug(msg string, fields ...any)
}

// DefaultLogger provides a simple logging implementation.
type DefaultLogger struct{}

// Info logs an info message.
func (l *DefaultLogger) Info(msg string, fields ...any) {
	log.Printf("[INFO] %s %v", msg, fields)
}

// Error logs an error message.
func (l *DefaultLogger) Error(msg string, err error, fields ...any) {
	log.Printf("[ERROR] %s: %v %v", msg, err, fields)
}

// Debug logs a debug message.
func (l *DefaultLogger) Debug(msg string, fields ...any) {
	log.Printf("[DEBUG] %s %v", msg, fields)
}

// NewLoggingEventHandler creates a new logging event handler.
func NewLoggingEventHandler(logger Logger) *LoggingEventHandler {
	if logger == nil {
		logger = &DefaultLogger{}
	}
	return &LoggingEventHandler{logger: logger}
}

// OnSessionCreated logs when a session is created.
func (h *LoggingEventHandler) OnSessionCreated(ctx context.Context, session *core.Session) error {
	h.logger.Info("Session created",
		"session_id", session.ID,
		"app_name", session.AppName,
		"user_id", session.UserID,
		"state_keys", len(session.State),
	)
	return nil
}

// OnSessionUpdated logs when a session is updated.
func (h *LoggingEventHandler) OnSessionUpdated(ctx context.Context, session *core.Session, oldState map[string]any) error {
	h.logger.Info("Session updated",
		"session_id", session.ID,
		"app_name", session.AppName,
		"user_id", session.UserID,
		"old_state_keys", len(oldState),
		"new_state_keys", len(session.State),
		"event_count", len(session.Events),
	)
	return nil
}

// OnSessionDeleted logs when a session is deleted.
func (h *LoggingEventHandler) OnSessionDeleted(ctx context.Context, appName, userID, sessionID string) error {
	h.logger.Info("Session deleted",
		"session_id", sessionID,
		"app_name", appName,
		"user_id", userID,
	)
	return nil
}

// OnEventAdded logs when an event is added to a session.
func (h *LoggingEventHandler) OnEventAdded(ctx context.Context, session *core.Session, event *core.Event) error {
	eventType := "unknown"
	if event.Content != nil {
		for _, part := range event.Content.Parts {
			if part.FunctionCall != nil {
				eventType = "function_call"
				break
			}
			if part.FunctionResponse != nil {
				eventType = "function_response"
				break
			}
			if part.Text != nil {
				eventType = "text"
				break
			}
		}
	}

	h.logger.Debug("Event added",
		"session_id", session.ID,
		"event_id", event.ID,
		"author", event.Author,
		"event_type", eventType,
		"has_error", event.ErrorMessage != nil,
	)
	return nil
}

// MetricsEventHandler collects metrics about session usage.
type MetricsEventHandler struct {
	metrics MetricsCollector
}

// MetricsCollector interface for collecting session metrics.
type MetricsCollector interface {
	IncrementCounter(name string, tags map[string]string)
	SetGauge(name string, value float64, tags map[string]string)
	RecordHistogram(name string, value float64, tags map[string]string)
}

// DefaultMetricsCollector provides a simple metrics implementation.
type DefaultMetricsCollector struct{}

// IncrementCounter increments a counter metric.
func (m *DefaultMetricsCollector) IncrementCounter(name string, tags map[string]string) {
	log.Printf("[METRIC] Counter %s incremented, tags: %v", name, tags)
}

// SetGauge sets a gauge metric.
func (m *DefaultMetricsCollector) SetGauge(name string, value float64, tags map[string]string) {
	log.Printf("[METRIC] Gauge %s set to %f, tags: %v", name, value, tags)
}

// RecordHistogram records a histogram metric.
func (m *DefaultMetricsCollector) RecordHistogram(name string, value float64, tags map[string]string) {
	log.Printf("[METRIC] Histogram %s recorded %f, tags: %v", name, value, tags)
}

// NewMetricsEventHandler creates a new metrics event handler.
func NewMetricsEventHandler(metrics MetricsCollector) *MetricsEventHandler {
	if metrics == nil {
		metrics = &DefaultMetricsCollector{}
	}
	return &MetricsEventHandler{metrics: metrics}
}

// OnSessionCreated records session creation metrics.
func (h *MetricsEventHandler) OnSessionCreated(ctx context.Context, session *core.Session) error {
	tags := map[string]string{
		"app_name": session.AppName,
	}
	h.metrics.IncrementCounter("sessions.created", tags)
	h.metrics.SetGauge("session.state_size", float64(len(session.State)), tags)
	return nil
}

// OnSessionUpdated records session update metrics.
func (h *MetricsEventHandler) OnSessionUpdated(ctx context.Context, session *core.Session, oldState map[string]any) error {
	tags := map[string]string{
		"app_name": session.AppName,
	}
	h.metrics.IncrementCounter("sessions.updated", tags)
	h.metrics.SetGauge("session.state_size", float64(len(session.State)), tags)
	h.metrics.SetGauge("session.event_count", float64(len(session.Events)), tags)
	return nil
}

// OnSessionDeleted records session deletion metrics.
func (h *MetricsEventHandler) OnSessionDeleted(ctx context.Context, appName, userID, sessionID string) error {
	tags := map[string]string{
		"app_name": appName,
	}
	h.metrics.IncrementCounter("sessions.deleted", tags)
	return nil
}

// OnEventAdded records event addition metrics.
func (h *MetricsEventHandler) OnEventAdded(ctx context.Context, session *core.Session, event *core.Event) error {
	tags := map[string]string{
		"app_name": session.AppName,
		"author":   event.Author,
	}

	h.metrics.IncrementCounter("events.added", tags)

	if event.ErrorMessage != nil {
		h.metrics.IncrementCounter("events.errors", tags)
	}

	// Record processing time if available (would need to be added to event metadata)
	if event.CustomMetadata != nil {
		if processingTime, ok := event.CustomMetadata["processing_time_ms"].(float64); ok {
			h.metrics.RecordHistogram("event.processing_time", processingTime, tags)
		}
	}

	return nil
}

// ValidationEventHandler validates session data and prevents invalid operations.
type ValidationEventHandler struct {
	maxStateSize     int
	maxEventSize     int
	allowedAuthors   map[string]bool
	forbiddenKeys    map[string]bool
	customValidators []SessionValidator
}

// SessionValidator provides custom validation logic.
type SessionValidator interface {
	ValidateSession(ctx context.Context, session *core.Session) error
	ValidateEvent(ctx context.Context, session *core.Session, event *core.Event) error
}

// ValidationConfig contains configuration for the validation handler.
type ValidationConfig struct {
	MaxStateSize     int                `json:"max_state_size"`
	MaxEventSize     int                `json:"max_event_size"`
	AllowedAuthors   []string           `json:"allowed_authors"`
	ForbiddenKeys    []string           `json:"forbidden_keys"`
	CustomValidators []SessionValidator `json:"-"`
}

// NewValidationEventHandler creates a new validation event handler.
func NewValidationEventHandler(config *ValidationConfig) *ValidationEventHandler {
	if config == nil {
		config = &ValidationConfig{
			MaxStateSize: 1024 * 1024, // 1MB
			MaxEventSize: 512 * 1024,  // 512KB
		}
	}

	handler := &ValidationEventHandler{
		maxStateSize:     config.MaxStateSize,
		maxEventSize:     config.MaxEventSize,
		allowedAuthors:   make(map[string]bool),
		forbiddenKeys:    make(map[string]bool),
		customValidators: config.CustomValidators,
	}

	for _, author := range config.AllowedAuthors {
		handler.allowedAuthors[author] = true
	}

	for _, key := range config.ForbiddenKeys {
		handler.forbiddenKeys[key] = true
	}

	return handler
}

// OnSessionCreated validates a new session.
func (h *ValidationEventHandler) OnSessionCreated(ctx context.Context, session *core.Session) error {
	return h.validateSession(ctx, session)
}

// OnSessionUpdated validates session updates.
func (h *ValidationEventHandler) OnSessionUpdated(ctx context.Context, session *core.Session, oldState map[string]any) error {
	return h.validateSession(ctx, session)
}

// OnSessionDeleted validates session deletion (placeholder).
func (h *ValidationEventHandler) OnSessionDeleted(ctx context.Context, appName, userID, sessionID string) error {
	// No validation needed for deletion
	return nil
}

// OnEventAdded validates new events.
func (h *ValidationEventHandler) OnEventAdded(ctx context.Context, session *core.Session, event *core.Event) error {
	// Validate event author
	if len(h.allowedAuthors) > 0 && !h.allowedAuthors[event.Author] {
		return fmt.Errorf("author not allowed: %s", event.Author)
	}

	// Validate event size
	eventSize := h.calculateEventSize(event)
	if h.maxEventSize > 0 && eventSize > h.maxEventSize {
		return fmt.Errorf("event size %d exceeds maximum %d", eventSize, h.maxEventSize)
	}

	// Run custom validators
	for _, validator := range h.customValidators {
		if err := validator.ValidateEvent(ctx, session, event); err != nil {
			return fmt.Errorf("custom validation failed: %w", err)
		}
	}

	return nil
}

// validateSession validates session data.
func (h *ValidationEventHandler) validateSession(ctx context.Context, session *core.Session) error {
	// Validate state size
	stateSize := h.calculateStateSize(session.State)
	if h.maxStateSize > 0 && stateSize > h.maxStateSize {
		return fmt.Errorf("session state size %d exceeds maximum %d", stateSize, h.maxStateSize)
	}

	// Validate forbidden keys
	for key := range session.State {
		if h.forbiddenKeys[key] {
			return fmt.Errorf("forbidden state key: %s", key)
		}
	}

	// Run custom validators
	for _, validator := range h.customValidators {
		if err := validator.ValidateSession(ctx, session); err != nil {
			return fmt.Errorf("custom validation failed: %w", err)
		}
	}

	return nil
}

// calculateStateSize estimates the size of session state in bytes.
func (h *ValidationEventHandler) calculateStateSize(state map[string]any) int {
	// Simple estimation - in practice, you might want to use JSON marshaling
	size := 0
	for k, v := range state {
		size += len(k)
		size += h.estimateValueSize(v)
	}
	return size
}

// calculateEventSize estimates the size of an event in bytes.
func (h *ValidationEventHandler) calculateEventSize(event *core.Event) int {
	// Simple estimation - in practice, you might want to use JSON marshaling
	size := len(event.ID) + len(event.InvocationID) + len(event.Author)

	if event.Content != nil {
		for _, part := range event.Content.Parts {
			if part.Text != nil {
				size += len(*part.Text)
			}
			// Add estimates for function calls/responses if needed
		}
	}

	return size
}

// estimateValueSize estimates the size of a value in bytes.
func (h *ValidationEventHandler) estimateValueSize(value any) int {
	switch v := value.(type) {
	case string:
		return len(v)
	case int, int32, int64, float32, float64, bool:
		return 8 // Rough estimate
	case map[string]any:
		size := 0
		for k, val := range v {
			size += len(k) + h.estimateValueSize(val)
		}
		return size
	case []any:
		size := 0
		for _, val := range v {
			size += h.estimateValueSize(val)
		}
		return size
	default:
		return 100 // Default estimate for unknown types
	}
}

// CompositeEventHandler combines multiple event handlers.
type CompositeEventHandler struct {
	handlers []SessionEventHandler
}

// NewCompositeEventHandler creates a composite event handler.
func NewCompositeEventHandler(handlers ...SessionEventHandler) *CompositeEventHandler {
	return &CompositeEventHandler{handlers: handlers}
}

// AddHandler adds a new event handler.
func (h *CompositeEventHandler) AddHandler(handler SessionEventHandler) {
	h.handlers = append(h.handlers, handler)
}

// OnSessionCreated calls all handlers for session creation.
func (h *CompositeEventHandler) OnSessionCreated(ctx context.Context, session *core.Session) error {
	for _, handler := range h.handlers {
		if err := handler.OnSessionCreated(ctx, session); err != nil {
			return err
		}
	}
	return nil
}

// OnSessionUpdated calls all handlers for session updates.
func (h *CompositeEventHandler) OnSessionUpdated(ctx context.Context, session *core.Session, oldState map[string]any) error {
	for _, handler := range h.handlers {
		if err := handler.OnSessionUpdated(ctx, session, oldState); err != nil {
			return err
		}
	}
	return nil
}

// OnSessionDeleted calls all handlers for session deletion.
func (h *CompositeEventHandler) OnSessionDeleted(ctx context.Context, appName, userID, sessionID string) error {
	for _, handler := range h.handlers {
		if err := handler.OnSessionDeleted(ctx, appName, userID, sessionID); err != nil {
			return err
		}
	}
	return nil
}

// OnEventAdded calls all handlers for event addition.
func (h *CompositeEventHandler) OnEventAdded(ctx context.Context, session *core.Session, event *core.Event) error {
	for _, handler := range h.handlers {
		if err := handler.OnEventAdded(ctx, session, event); err != nil {
			return err
		}
	}
	return nil
}

// AsyncEventHandler wraps another handler to execute asynchronously.
type AsyncEventHandler struct {
	handler SessionEventHandler
	timeout time.Duration
}

// NewAsyncEventHandler creates an async event handler wrapper.
func NewAsyncEventHandler(handler SessionEventHandler, timeout time.Duration) *AsyncEventHandler {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &AsyncEventHandler{
		handler: handler,
		timeout: timeout,
	}
}

// OnSessionCreated handles session creation asynchronously.
func (h *AsyncEventHandler) OnSessionCreated(ctx context.Context, session *core.Session) error {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
		defer cancel()

		if err := h.handler.OnSessionCreated(ctx, session); err != nil {
			log.Printf("Async handler error on session created: %v", err)
		}
	}()
	return nil
}

// OnSessionUpdated handles session updates asynchronously.
func (h *AsyncEventHandler) OnSessionUpdated(ctx context.Context, session *core.Session, oldState map[string]any) error {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
		defer cancel()

		if err := h.handler.OnSessionUpdated(ctx, session, oldState); err != nil {
			log.Printf("Async handler error on session updated: %v", err)
		}
	}()
	return nil
}

// OnSessionDeleted handles session deletion asynchronously.
func (h *AsyncEventHandler) OnSessionDeleted(ctx context.Context, appName, userID, sessionID string) error {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
		defer cancel()

		if err := h.handler.OnSessionDeleted(ctx, appName, userID, sessionID); err != nil {
			log.Printf("Async handler error on session deleted: %v", err)
		}
	}()
	return nil
}

// OnEventAdded handles event addition asynchronously.
func (h *AsyncEventHandler) OnEventAdded(ctx context.Context, session *core.Session, event *core.Event) error {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
		defer cancel()

		if err := h.handler.OnEventAdded(ctx, session, event); err != nil {
			log.Printf("Async handler error on event added: %v", err)
		}
	}()
	return nil
}
