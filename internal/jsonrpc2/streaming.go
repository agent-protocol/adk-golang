package jsonrpc2

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/agent-protocol/adk-golang/pkg/a2a"
)

// StreamWriter is a helper for writing streaming JSON-RPC responses
type StreamWriter struct {
	w      http.ResponseWriter
	id     any
	mu     sync.Mutex
	first  bool
	closed bool
}

// NewStreamWriter creates a new StreamWriter
func NewStreamWriter(w http.ResponseWriter, id any) *StreamWriter {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")
	return &StreamWriter{
		w:     w,
		id:    id,
		first: true,
	}
}

// WriteStatusUpdate sends a task status update to the client
func (sw *StreamWriter) WriteStatusUpdate(update *a2a.TaskStatusUpdateEvent) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.closed {
		return nil
	}

	response := &a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      sw.id,
		Result:  update,
	}

	if flusher, ok := sw.w.(http.Flusher); ok {
		err := json.NewEncoder(sw.w).Encode(response)
		if err == nil {
			flusher.Flush()
		}
		return err
	}

	return json.NewEncoder(sw.w).Encode(response)
}

// WriteArtifactUpdate sends a task artifact update to the client
func (sw *StreamWriter) WriteArtifactUpdate(update *a2a.TaskArtifactUpdateEvent) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.closed {
		return nil
	}

	response := &a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      sw.id,
		Result:  update,
	}

	if flusher, ok := sw.w.(http.Flusher); ok {
		err := json.NewEncoder(sw.w).Encode(response)
		if err == nil {
			flusher.Flush()
		}
		return err
	}

	return json.NewEncoder(sw.w).Encode(response)
}

// WriteError sends an error response to the client
func (sw *StreamWriter) WriteError(err *a2a.JSONRPCError) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.closed {
		return nil
	}

	sw.closed = true

	response := &a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      sw.id,
		Error:   err,
	}

	if flusher, ok := sw.w.(http.Flusher); ok {
		err := json.NewEncoder(sw.w).Encode(response)
		if err == nil {
			flusher.Flush()
		}
		return err
	}

	return json.NewEncoder(sw.w).Encode(response)
}

// Close finalizes the stream
func (sw *StreamWriter) Close() error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.closed = true
	return nil
}
