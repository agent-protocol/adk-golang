package jsonrpc2

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/a2a"
)

// TestSendTask tests the tasks/send endpoint
func TestSendTask(t *testing.T) {
	handler := NewMockTaskHandler()
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// Create a task send request
	params := a2a.TaskSendParams{
		ID: "task-123",
		Message: a2a.Message{
			Role: "user",
			Parts: []a2a.Part{
				{
					Type: "text",
					Text: stringPtr("Hello, this is a test message"),
				},
			},
		},
	}

	reqBody, err := makeRequest("tasks/send", params, 1)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Send the request
	resp, err := http.Post(testServer.URL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Read and parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var respObj a2a.SendTaskResponse
	if err := json.Unmarshal(body, &respObj); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Validate the response
	if respObj.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC version 2.0, got %s", respObj.JSONRPC)
	}

	if respObj.ID != float64(1) { // JSON numbers are parsed as float64
		t.Errorf("Expected ID 1, got %v", respObj.ID)
	}

	if respObj.Result == nil {
		t.Fatal("Expected non-nil result")
	}

	if respObj.Result.ID != "task-123" {
		t.Errorf("Expected task ID task-123, got %s", respObj.Result.ID)
	}

	if respObj.Result.Status.State != a2a.TaskStateSubmitted {
		t.Errorf("Expected task state %s, got %s", a2a.TaskStateSubmitted, respObj.Result.Status.State)
	}
}

// TestGetTask tests the tasks/get endpoint
func TestGetTask(t *testing.T) {
	handler := NewMockTaskHandler()
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// First create a task
	taskID := "task-abc"
	task := &a2a.Task{
		ID: taskID,
		Status: a2a.TaskStatus{
			State: a2a.TaskStateSubmitted,
		},
	}
	handler.tasks[taskID] = task

	// Create a task get request
	params := a2a.TaskQueryParams{
		ID: taskID,
	}

	reqBody, err := makeRequest("tasks/get", params, 2)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Send the request
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

	var respObj a2a.GetTaskResponse
	if err := json.Unmarshal(body, &respObj); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Validate the response
	if respObj.Result == nil {
		t.Fatal("Expected non-nil result")
	}

	if respObj.Result.ID != taskID {
		t.Errorf("Expected task ID %s, got %s", taskID, respObj.Result.ID)
	}

	if respObj.Result.Status.State != a2a.TaskStateSubmitted {
		t.Errorf("Expected task state %s, got %s", a2a.TaskStateSubmitted, respObj.Result.Status.State)
	}
}

// TestGetTaskNotFound tests the error handling for a non-existent task
func TestGetTaskNotFound(t *testing.T) {
	handler := NewMockTaskHandler()
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// Create a task get request with a non-existent ID
	params := a2a.TaskQueryParams{
		ID: "non-existent-task",
	}

	reqBody, err := makeRequest("tasks/get", params, 3)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Send the request
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

	var respObj a2a.GetTaskResponse
	if err := json.Unmarshal(body, &respObj); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Validate the error response
	if respObj.Error == nil {
		t.Fatal("Expected error in response")
	}

	if respObj.Error.Code != -32001 {
		t.Errorf("Expected error code -32001, got %d", respObj.Error.Code)
	}

	if respObj.Error.Message != "Task not found" {
		t.Errorf("Expected error message 'Task not found', got %s", respObj.Error.Message)
	}
}

// TestCancelTask tests the tasks/cancel endpoint
func TestCancelTask(t *testing.T) {
	handler := NewMockTaskHandler()
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// First create a task
	taskID := "task-to-cancel"
	task := &a2a.Task{
		ID: taskID,
		Status: a2a.TaskStatus{
			State: a2a.TaskStateWorking,
		},
	}
	handler.tasks[taskID] = task

	// Create a cancel task request
	params := a2a.TaskIdParams{
		ID: taskID,
	}

	reqBody, err := makeRequest("tasks/cancel", params, 4)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Send the request
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

	var respObj a2a.CancelTaskResponse
	if err := json.Unmarshal(body, &respObj); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Validate the response
	if respObj.Result == nil {
		t.Fatal("Expected non-nil result")
	}

	if respObj.Result.ID != taskID {
		t.Errorf("Expected task ID %s, got %s", taskID, respObj.Result.ID)
	}

	if respObj.Result.Status.State != a2a.TaskStateCanceled {
		t.Errorf("Expected task state %s, got %s", a2a.TaskStateCanceled, respObj.Result.Status.State)
	}
}

// TestPushNotificationMethods tests the tasks/pushNotification/set and tasks/pushNotification/get endpoints
func TestPushNotificationMethods(t *testing.T) {
	handler := NewMockTaskHandler()
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// Create a task for push notifications
	taskID := "task-with-notifications"
	task := &a2a.Task{
		ID: taskID,
		Status: a2a.TaskStatus{
			State: a2a.TaskStateSubmitted,
		},
	}
	handler.tasks[taskID] = task

	// Set push notification request
	setParams := a2a.TaskPushNotificationConfig{
		ID: taskID,
		PushNotificationConfig: a2a.PushNotificationConfig{
			URL: "https://example.com/webhook",
		},
	}

	reqBody, err := makeRequest("tasks/pushNotification/set", setParams, 5)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Send the request
	resp, err := http.Post(testServer.URL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Validate SET response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var setResp a2a.SetTaskPushNotificationResponse
	if err := json.Unmarshal(body, &setResp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if setResp.Result == nil {
		t.Fatal("Expected non-nil result")
	}

	if setResp.Result.ID != taskID {
		t.Errorf("Expected task ID %s, got %s", taskID, setResp.Result.ID)
	}

	if setResp.Result.PushNotificationConfig.URL != "https://example.com/webhook" {
		t.Errorf("Expected URL https://example.com/webhook, got %s", setResp.Result.PushNotificationConfig.URL)
	}

	// Now test GET request
	getParams := a2a.TaskIdParams{
		ID: taskID,
	}

	reqBody, err = makeRequest("tasks/pushNotification/get", getParams, 6)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Send the request
	resp, err = http.Post(testServer.URL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Validate GET response
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var getResp a2a.GetTaskPushNotificationResponse
	if err := json.Unmarshal(body, &getResp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if getResp.Result == nil {
		t.Fatal("Expected non-nil result")
	}

	if getResp.Result.ID != taskID {
		t.Errorf("Expected task ID %s, got %s", taskID, getResp.Result.ID)
	}

	if getResp.Result.PushNotificationConfig.URL != "https://example.com/webhook" {
		t.Errorf("Expected URL https://example.com/webhook, got %s", getResp.Result.PushNotificationConfig.URL)
	}
}

// TestInvalidJSONRPC tests error handling for invalid JSON-RPC requests
func TestInvalidJSONRPC(t *testing.T) {
	handler := NewMockTaskHandler()
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// Test cases with different invalid requests
	testCases := []struct {
		name           string
		requestBody    string
		expectedStatus int
		expectedCode   int
		expectedMsg    string
	}{
		{
			name:           "Invalid JSON",
			requestBody:    `{this is not valid json`,
			expectedStatus: http.StatusOK,
			expectedCode:   -32700,
			expectedMsg:    "Invalid JSON payload",
		},
		{
			name:           "Missing JSONRPC Version",
			requestBody:    `{"method":"tasks/get","params":{"id":"task-123"},"id":1}`,
			expectedStatus: http.StatusOK,
			expectedCode:   -32600,
			expectedMsg:    "Request payload validation error",
		},
		{
			name:           "Wrong JSONRPC Version",
			requestBody:    `{"jsonrpc":"1.0","method":"tasks/get","params":{"id":"task-123"},"id":1}`,
			expectedStatus: http.StatusOK,
			expectedCode:   -32600,
			expectedMsg:    "Request payload validation error",
		},
		{
			name:           "Unknown Method",
			requestBody:    `{"jsonrpc":"2.0","method":"tasks/unknown","params":{},"id":1}`,
			expectedStatus: http.StatusOK,
			expectedCode:   -32601,
			expectedMsg:    "Method not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Post(testServer.URL, "application/json", strings.NewReader(tc.requestBody))
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			var respObj a2a.JSONRPCResponse
			if err := json.Unmarshal(body, &respObj); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

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

// TestBatchRequests tests handling of batch JSON-RPC requests
func TestBatchRequests(t *testing.T) {
	handler := NewMockTaskHandler()
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// Create two tasks for the batch test
	taskID1 := "batch-task-1"
	task1 := &a2a.Task{
		ID: taskID1,
		Status: a2a.TaskStatus{
			State: a2a.TaskStateSubmitted,
		},
	}
	handler.tasks[taskID1] = task1

	taskID2 := "batch-task-2"
	task2 := &a2a.Task{
		ID: taskID2,
		Status: a2a.TaskStatus{
			State: a2a.TaskStateWorking,
		},
	}
	handler.tasks[taskID2] = task2

	// Create a batch request with two get requests
	batchRequest := fmt.Sprintf(`[
		{"jsonrpc":"2.0","method":"tasks/get","params":{"id":"%s"},"id":10},
		{"jsonrpc":"2.0","method":"tasks/get","params":{"id":"%s"},"id":11}
	]`, taskID1, taskID2)

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

	// Validate it's a JSON array
	if !strings.HasPrefix(strings.TrimSpace(string(body)), "[") {
		t.Fatalf("Expected JSON array response, got: %s", string(body))
	}

	var responses []json.RawMessage
	if err := json.Unmarshal(body, &responses); err != nil {
		t.Fatalf("Failed to parse batch response: %v", err)
	}

	// Should have 2 responses
	if len(responses) != 2 {
		t.Fatalf("Expected 2 responses, got %d", len(responses))
	}

	// Parse and validate each response
	var resp1 a2a.GetTaskResponse
	if err := json.Unmarshal(responses[0], &resp1); err != nil {
		t.Fatalf("Failed to parse first response: %v", err)
	}

	if resp1.ID != float64(10) {
		t.Errorf("Expected ID 10, got %v", resp1.ID)
	}

	if resp1.Result == nil || resp1.Result.ID != taskID1 {
		t.Errorf("First response task ID mismatch")
	}

	var resp2 a2a.GetTaskResponse
	if err := json.Unmarshal(responses[1], &resp2); err != nil {
		t.Fatalf("Failed to parse second response: %v", err)
	}

	if resp2.ID != float64(11) {
		t.Errorf("Expected ID 11, got %v", resp2.ID)
	}

	if resp2.Result == nil || resp2.Result.ID != taskID2 {
		t.Errorf("Second response task ID mismatch")
	}
}

// TestNotifications tests handling of JSON-RPC notifications (no ID)
func TestNotifications(t *testing.T) {
	handler := NewMockTaskHandler()
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// Create a notification request (no ID)
	notification := `{"jsonrpc":"2.0","method":"tasks/send","params":{"id":"notif-task","message":{"role":"user","parts":[{"type":"text","text":"Notification test"}]}}}`

	// Send the notification
	resp, err := http.Post(testServer.URL, "application/json", strings.NewReader(notification))
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}
	defer resp.Body.Close()

	// For notifications, we should still get a status 200
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// According to JSON-RPC spec, notifications should not produce a response
	// But some implementations still return empty JSON object or null
	trimmedBody := strings.TrimSpace(string(body))
	if trimmedBody != "" && trimmedBody != "{}" && trimmedBody != "null" {
		t.Errorf("Expected empty response for notification, got: %s", trimmedBody)
	}

	// Verify the task was still created in the handler
	if _, exists := handler.tasks["notif-task"]; !exists {
		t.Error("Expected notification to create a task")
	}
}

// TestMethodErrors tests error handling for various methods
func TestMethodErrors(t *testing.T) {
	handler := NewMockTaskHandler()
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// Set a specific method to fail
	handler.SetTaskToFail("tasks/send")

	// Create a task send request that should fail
	params := a2a.TaskSendParams{
		ID: "fail-task",
		Message: a2a.Message{
			Role: "user",
			Parts: []a2a.Part{
				{
					Type: "text",
					Text: stringPtr("This should fail"),
				},
			},
		},
	}

	reqBody, err := makeRequest("tasks/send", params, 99)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Send the request
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

	var respObj a2a.SendTaskResponse
	if err := json.Unmarshal(body, &respObj); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Should have an error with code -32602 (Invalid params)
	if respObj.Error == nil {
		t.Fatal("Expected error in response")
	}

	if respObj.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", respObj.Error.Code)
	}

	if respObj.Error.Message != "Invalid parameters" {
		t.Errorf("Expected error message 'Invalid parameters', got %s", respObj.Error.Message)
	}
}

// TestStreamingEndpoints tests the streaming endpoints
func TestStreamingEndpoints(t *testing.T) {
	handler := NewMockTaskHandler()
	handler.SetStreamingDelay(50 * time.Millisecond) // Short delay for tests
	server := NewServer(handler)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// Test tasks/sendSubscribe
	t.Run("SendSubscribe", func(t *testing.T) {
		params := a2a.TaskSendParams{
			ID: "stream-task",
			Message: a2a.Message{
				Role: "user",
				Parts: []a2a.Part{
					{
						Type: "text",
						Text: stringPtr("Streaming test message"),
					},
				},
			},
		}

		reqBody, err := makeRequest("tasks/sendSubscribe", params, 100)
		if err != nil {
			t.Fatalf("Failed to create streaming request: %v", err)
		}

		// Send the request
		resp, err := http.Post(testServer.URL, "application/json", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("Failed to send streaming request: %v", err)
		}
		defer resp.Body.Close()

		// Content type should be application/x-ndjson
		if !strings.Contains(resp.Header.Get("Content-Type"), "application/x-ndjson") {
			t.Errorf("Expected Content-Type to contain application/x-ndjson, got %s", resp.Header.Get("Content-Type"))
		}

		// Read all streaming responses
		responses := readStreamingResponses(t, resp)

		// Should have at least 4 responses (submitted, working, artifact, completed)
		if len(responses) < 4 {
			t.Fatalf("Expected at least 4 streaming responses, got %d", len(responses))
		}

		// Validate the responses follow the expected sequence
		var foundSubmitted, foundWorking, foundArtifact, foundCompleted bool

		for _, resp := range responses {
			if resp.Result == nil {
				t.Errorf("Expected non-nil result in streaming response")
				continue
			}

			// Check for status update events
			if statusUpdate, ok := resp.Result.(map[string]interface{}); ok && statusUpdate["status"] != nil {
				status := statusUpdate["status"].(map[string]interface{})
				state := status["state"].(string)

				if state == "submitted" {
					foundSubmitted = true
				} else if state == "working" {
					foundWorking = true
				} else if state == "completed" {
					foundCompleted = true
					final := statusUpdate["final"].(bool)
					if !final {
						t.Errorf("Expected completed state to have final=true")
					}
				}
			}

			// Check for artifact events
			if artifactUpdate, ok := resp.Result.(map[string]interface{}); ok && artifactUpdate["artifact"] != nil {
				foundArtifact = true
			}
		}

		if !foundSubmitted {
			t.Error("Missing 'submitted' status update in streaming responses")
		}
		if !foundWorking {
			t.Error("Missing 'working' status update in streaming responses")
		}
		if !foundArtifact {
			t.Error("Missing artifact update in streaming responses")
		}
		if !foundCompleted {
			t.Error("Missing 'completed' status update in streaming responses")
		}
	})

	// Test tasks/resubscribe
	t.Run("Resubscribe", func(t *testing.T) {
		// First create a task
		taskID := "resubscribe-task"
		task := &a2a.Task{
			ID: taskID,
			Status: a2a.TaskStatus{
				State: a2a.TaskStateWorking,
			},
		}
		handler.tasks[taskID] = task

		// Create resubscribe request
		params := a2a.TaskQueryParams{
			ID: taskID,
		}

		reqBody, err := makeRequest("tasks/resubscribe", params, 101)
		if err != nil {
			t.Fatalf("Failed to create resubscribe request: %v", err)
		}

		// Send the request
		resp, err := http.Post(testServer.URL, "application/json", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("Failed to send resubscribe request: %v", err)
		}
		defer resp.Body.Close()

		// Read all streaming responses
		responses := readStreamingResponses(t, resp)

		// Should have at least 2 responses (working status and artifact)
		if len(responses) < 2 {
			t.Fatalf("Expected at least 2 streaming responses, got %d", len(responses))
		}

		var foundWorking, foundArtifact bool

		for _, resp := range responses {
			if resp.Result == nil {
				t.Errorf("Expected non-nil result in streaming response")
				continue
			}

			// Check for status updates
			if statusUpdate, ok := resp.Result.(map[string]interface{}); ok && statusUpdate["status"] != nil {
				status := statusUpdate["status"].(map[string]interface{})
				state := status["state"].(string)

				if state == "working" {
					foundWorking = true
				}
			}

			// Check for artifact updates
			if artifactUpdate, ok := resp.Result.(map[string]interface{}); ok && artifactUpdate["artifact"] != nil {
				foundArtifact = true
				artifact := artifactUpdate["artifact"].(map[string]interface{})
				name := artifact["name"].(string)
				if name != "Resubscribed Artifact" {
					t.Errorf("Expected artifact name 'Resubscribed Artifact', got %s", name)
				}
			}
		}

		if !foundWorking {
			t.Error("Missing 'working' status update in resubscribe responses")
		}
		if !foundArtifact {
			t.Error("Missing artifact update in resubscribe responses")
		}
	})
}

// Helper for testing streaming responses
func readStreamingResponses(t *testing.T, resp *http.Response) []a2a.JSONRPCResponse {
	t.Helper()
	scanner := bufio.NewScanner(resp.Body)
	var responses []a2a.JSONRPCResponse

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var response a2a.JSONRPCResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			t.Fatalf("Failed to parse streaming response: %v", err)
			continue
		}

		responses = append(responses, response)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading response stream: %v", err)
	}

	return responses
}
