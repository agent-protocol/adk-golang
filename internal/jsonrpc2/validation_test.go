package jsonrpc2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agent-protocol/adk-golang/pkg/a2a"
)

// TestWrongRequestMethod tests that non-POST requests are rejected
func TestWrongRequestMethod(t *testing.T) {
	handler := NewValidationMockTaskHandler()
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// Test different HTTP methods that should be rejected
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodOptions, http.MethodHead}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req, err := http.NewRequest(method, testServer.URL, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			// Should get a 405 Method Not Allowed
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("Expected status code %d (Method Not Allowed), got %d", http.StatusMethodNotAllowed, resp.StatusCode)
			}

			// Verify the response body
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			// Check for the right error message
			// Skip body content check for HEAD requests since they don't return a body
			if method != http.MethodHead && !strings.Contains(string(body), "Method not allowed") {
				t.Errorf("Expected error message about method not being allowed, got: %s", string(body))
			}
		})
	}
}

// ErrorReader is a mock reader that returns an error when Read is called
type ErrorReader struct{}

func (r ErrorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("forced read error")
}

func (r ErrorReader) Close() error {
	return nil
}

// TestErrorReadingRequestBody tests error handling for request body reading errors
func TestErrorReadingRequestBody(t *testing.T) {
	handler := NewValidationMockTaskHandler()
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// Create a custom HTTP client that will use the error reader
	client := &http.Client{
		Transport: &errorBodyTransport{},
	}

	// Attempt to make a request that will have a read error
	req, err := http.NewRequest(http.MethodPost, testServer.URL, &ErrorReader{})
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err == nil {
		// We expect the request to fail, but if it doesn't, check the response
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected error reading body, got response: %s", string(body))
	}
}

// TestUnmarshalErrors tests error handling for unmarshal errors
func TestUnmarshalErrors(t *testing.T) {
	handler := NewValidationMockTaskHandler()
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	testCases := []struct {
		name         string
		requestBody  string
		expectedCode int
		expectedMsg  string
	}{
		{
			name:         "Invalid JSON structure",
			requestBody:  `{"jsonrpc":"2.0",,"method":"tasks/send","params":{},"id":1}`,
			expectedCode: -32700,
			expectedMsg:  "Invalid JSON payload",
		},
		{
			name:         "Invalid params structure",
			requestBody:  `{"jsonrpc":"2.0","method":"tasks/send","params":{"id":123},"id":1}`,
			expectedCode: -32602,
			expectedMsg:  "Invalid parameters",
		},
		{
			name:         "Missing required params",
			requestBody:  `{"jsonrpc":"2.0","method":"tasks/get","params":{"id":""},"id":1}`,
			expectedCode: -32602,
			expectedMsg:  "Invalid parameters",
		},
		{
			name:         "Wrong param type",
			requestBody:  `{"jsonrpc":"2.0","method":"tasks/send","params":{"id":123,"message":"not-an-object"},"id":1}`,
			expectedCode: -32602,
			expectedMsg:  "Invalid parameters",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Post(testServer.URL, "application/json", strings.NewReader(tc.requestBody))
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			// Read and parse the response
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			var respObj a2a.JSONRPCResponse
			if err := json.Unmarshal(body, &respObj); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			// Check for the expected error
			if respObj.Error == nil {
				t.Fatal("Expected error in response")
			}

			if respObj.Error.Code != tc.expectedCode {
				t.Errorf("Expected error code %d, got %d", tc.expectedCode, respObj.Error.Code)
			}

			if respObj.Error.Message != tc.expectedMsg {
				t.Errorf("Expected error message '%s', got '%s'", tc.expectedMsg, respObj.Error.Message)
			}
		})
	}
}

// TestEmptyBatchArray tests error handling for empty batch arrays
func TestEmptyBatchArray(t *testing.T) {
	handler := NewValidationMockTaskHandler()
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// Send an empty batch array
	resp, err := http.Post(testServer.URL, "application/json", strings.NewReader("[]"))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read and parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var respObj a2a.JSONRPCResponse
	if err := json.Unmarshal(body, &respObj); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check for the expected error
	if respObj.Error == nil {
		t.Fatal("Expected error in response")
	}

	if respObj.Error.Code != -32600 {
		t.Errorf("Expected error code -32600, got %d", respObj.Error.Code)
	}

	if respObj.Error.Message != "Request payload validation error" {
		t.Errorf("Expected error message 'Request payload validation error', got '%s'", respObj.Error.Message)
	}

	if respObj.Error.Data != "Batch request cannot be empty" {
		t.Errorf("Expected error data 'Batch request cannot be empty', got '%s'", respObj.Error.Data)
	}
}

// TestIncorrectJSONRPCVersion tests error handling for incorrect JSON-RPC versions
func TestIncorrectJSONRPCVersion(t *testing.T) {
	handler := NewValidationMockTaskHandler()
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	testCases := []struct {
		name           string
		jsonrpcVersion string
	}{
		{"Missing Version", ""},
		{"Version 1.0", "1.0"},
		{"Version 1.2", "1.2"},
		{"Version 3.0", "3.0"},
		{"Non-Numeric Version", "2.0a"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a request with the invalid JSONRPC version
			requestBody := fmt.Sprintf(`{"jsonrpc":"%s","method":"tasks/get","params":{"id":"task-123"},"id":1}`, tc.jsonrpcVersion)

			resp, err := http.Post(testServer.URL, "application/json", strings.NewReader(requestBody))
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			// Read and parse the response
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			var respObj a2a.JSONRPCResponse
			if err := json.Unmarshal(body, &respObj); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			// Check for the expected error
			if respObj.Error == nil {
				t.Fatal("Expected error in response")
			}

			if respObj.Error.Code != -32600 {
				t.Errorf("Expected error code -32600, got %d", respObj.Error.Code)
			}

			if respObj.Error.Message != "Request payload validation error" {
				t.Errorf("Expected error message 'Request payload validation error', got '%s'", respObj.Error.Message)
			}

			if respObj.Error.Data != "jsonrpc must be '2.0'" {
				t.Errorf("Expected error data \"jsonrpc must be '2.0'\", got '%s'", respObj.Error.Data)
			}
		})
	}
}

// TestBatchWithInvalidRequests tests batch requests with some invalid items
func TestBatchWithInvalidRequests(t *testing.T) {
	handler := NewValidationMockTaskHandler()
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// Create a task for valid requests
	taskID := "batch-mix-task"
	task := &a2a.Task{
		ID: taskID,
		Status: a2a.TaskStatus{
			State: a2a.TaskStateSubmitted,
		},
	}
	handler.tasks[taskID] = task

	// Create a batch with a mix of valid and invalid requests
	batchRequest := fmt.Sprintf(`[
		{"jsonrpc":"2.0","method":"tasks/get","params":{"id":"%s"},"id":1},
		{"jsonrpc":"1.0","method":"tasks/get","params":{"id":"%s"},"id":2},
		{"jsonrpc":"2.0","method":"unknown_method","params":{},"id":3},
		{"jsonrpc":"2.0","method":"tasks/get","params":{"id":"not-found"},"id":4}
	]`, taskID, taskID)

	// Send the batch request
	resp, err := http.Post(testServer.URL, "application/json", strings.NewReader(batchRequest))
	if err != nil {
		t.Fatalf("Failed to send batch request: %v", err)
	}
	defer resp.Body.Close()

	// Read and parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var responses []json.RawMessage
	if err := json.Unmarshal(body, &responses); err != nil {
		t.Fatalf("Failed to parse batch response: %v", err)
	}

	// Should have 4 responses
	if len(responses) != 4 {
		t.Fatalf("Expected 4 responses, got %d", len(responses))
	}

	// Parse and validate each response
	var resp1, resp2, resp3, resp4 a2a.JSONRPCResponse

	// First response should be successful
	if err := json.Unmarshal(responses[0], &resp1); err != nil {
		t.Fatalf("Failed to parse first response: %v", err)
	}
	if resp1.Error != nil {
		t.Errorf("Expected first response to succeed, got error: %v", resp1.Error)
	}

	// Second response should have invalid request error
	if err := json.Unmarshal(responses[1], &resp2); err != nil {
		t.Fatalf("Failed to parse second response: %v", err)
	}
	if resp2.Error == nil || resp2.Error.Code != -32600 {
		t.Errorf("Expected second response to have error code -32600, got: %v", resp2.Error)
	}

	// Third response should have method not found error
	if err := json.Unmarshal(responses[2], &resp3); err != nil {
		t.Fatalf("Failed to parse third response: %v", err)
	}
	if resp3.Error == nil || resp3.Error.Code != -32601 {
		t.Errorf("Expected third response to have error code -32601, got: %v", resp3.Error)
	}

	// Fourth response should have task not found error
	if err := json.Unmarshal(responses[3], &resp4); err != nil {
		t.Fatalf("Failed to parse fourth response: %v", err)
	}
	if resp4.Error == nil || resp4.Error.Code != -32001 {
		t.Errorf("Expected fourth response to have error code -32001, got: %v", resp4.Error)
	}
}

// TestCommonErrors tests error handling for common error types
func TestCommonErrors(t *testing.T) {
	handler := NewValidationMockTaskHandler()
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// Create a set of error instances to test
	errorTypes := []struct {
		name          string
		createRequest func() ([]byte, interface{})
		expectedCode  int
		expectedMsg   string
		expectedData  interface{}
	}{
		{
			name: "TaskNotFoundError",
			createRequest: func() ([]byte, interface{}) {
				// Create a request for a task that doesn't exist
				params := a2a.TaskQueryParams{ID: "non-existent-task"}
				reqBody, _ := makeRequest("tasks/get", params, 10)
				return reqBody, nil
			},
			expectedCode: -32001,
			expectedMsg:  "Task not found",
		},
		{
			name: "InvalidParamsError",
			createRequest: func() ([]byte, interface{}) {
				// Create a handler that will return InvalidParamsError
				handler.SetErrorToReturn("tasks/send", &a2a.InvalidParamsError{
					Code:    -32602,
					Message: "Invalid parameters",
					Data:    "Missing required field",
				})
				params := a2a.TaskSendParams{ID: "test-task"}
				reqBody, _ := makeRequest("tasks/send", params, 20)
				return reqBody, "Missing required field"
			},
			expectedCode: -32602,
			expectedMsg:  "Invalid parameters",
		},
		{
			name: "MethodNotFoundError",
			createRequest: func() ([]byte, interface{}) {
				// Create a request with a non-existent method
				params := map[string]string{"key": "value"}
				reqBody, _ := makeRequest("invalid/method", params, 30)
				return reqBody, nil
			},
			expectedCode: -32601,
			expectedMsg:  "Method not found",
		},
		{
			name: "InvalidRequestError",
			createRequest: func() ([]byte, interface{}) {
				// Create a request with wrong JSON-RPC version
				reqBody := []byte(`{"jsonrpc":"1.0","method":"tasks/get","params":{"id":"task-123"},"id":40}`)
				return reqBody, "jsonrpc must be '2.0'"
			},
			expectedCode: -32600,
			expectedMsg:  "Request payload validation error",
		},
		{
			name: "JSONParseError",
			createRequest: func() ([]byte, interface{}) {
				// Create a request with invalid JSON
				reqBody := []byte(`{"jsonrpc":"2.0","method":tasks/get,"params":{"id":"task-123"},"id":50}`)
				return reqBody, nil
			},
			expectedCode: -32700,
			expectedMsg:  "Invalid JSON payload",
		},
		{
			name: "InternalError",
			createRequest: func() ([]byte, interface{}) {
				// Create a handler that will return InternalError
				handler.SetErrorToReturn("tasks/get", &a2a.InternalError{
					Code:    -32603,
					Message: "Internal error",
					Data:    "Database connection failure",
				})
				params := a2a.TaskQueryParams{ID: "existing-task"}
				// Actually add the task so the error comes from the handler
				handler.MockTaskHandler.tasks["existing-task"] = &a2a.Task{ID: "existing-task"}
				reqBody, _ := makeRequest("tasks/get", params, 60)
				return reqBody, "Database connection failure"
			},
			expectedCode: -32603,
			expectedMsg:  "Internal error",
		},
		{
			name: "TaskNotCancelableError",
			createRequest: func() ([]byte, interface{}) {
				// Create a handler that will return TaskNotCancelableError
				handler.SetErrorToReturn("tasks/cancel", &a2a.TaskNotCancelableError{
					Code:    -32002,
					Message: "Task cannot be canceled",
					Data:    "Task already completed",
				})
				params := a2a.TaskIdParams{ID: "completed-task"}
				// Add a completed task
				handler.MockTaskHandler.tasks["completed-task"] = &a2a.Task{
					ID: "completed-task",
					Status: a2a.TaskStatus{
						State: a2a.TaskStateCompleted,
					},
				}
				reqBody, _ := makeRequest("tasks/cancel", params, 70)
				return reqBody, "Task already completed"
			},
			expectedCode: -32002,
			expectedMsg:  "Task cannot be canceled",
		},
	}

	for _, et := range errorTypes {
		t.Run(et.name, func(t *testing.T) {
			reqBody, expectedData := et.createRequest()

			resp, err := http.Post(testServer.URL, "application/json", bytes.NewReader(reqBody))
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			// Read and parse the response
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			var respObj a2a.JSONRPCResponse
			if err := json.Unmarshal(body, &respObj); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			// Check for the expected error
			if respObj.Error == nil {
				t.Fatalf("Expected error in response, got nil")
			}

			if respObj.Error.Code != et.expectedCode {
				t.Errorf("Expected error code %d, got %d", et.expectedCode, respObj.Error.Code)
			}

			if respObj.Error.Message != et.expectedMsg {
				t.Errorf("Expected error message '%s', got '%s'", et.expectedMsg, respObj.Error.Message)
			}

			// If specific expected data was provided, check that too
			if expectedData != nil && respObj.Error.Data != expectedData {
				t.Errorf("Expected error data '%v', got '%v'", expectedData, respObj.Error.Data)
			}
		})
	}
}

// Custom transport for testing read errors
type errorBodyTransport struct{}

func (t *errorBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// If the request body is an ErrorReader, simulate a read error
	if _, ok := req.Body.(*ErrorReader); ok {
		return nil, errors.New("simulated read error")
	}

	// Otherwise use the default transport
	return http.DefaultTransport.RoundTrip(req)
}
