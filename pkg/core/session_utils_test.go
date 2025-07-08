package core

import (
	"testing"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

func TestSessionStateHelper_String(t *testing.T) {
	session := NewSession("session1", "testapp", "user1")
	helper := NewSessionStateHelper(session)

	// Test setting and getting string
	helper.SetString("key1", "value1")
	value, err := helper.GetString("key1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != "value1" {
		t.Errorf("Expected 'value1', got '%s'", value)
	}

	// Test default value
	defaultValue := helper.GetStringWithDefault("nonexistent", "default")
	if defaultValue != "default" {
		t.Errorf("Expected 'default', got '%s'", defaultValue)
	}

	// Test non-string conversion
	session.SetState("intkey", 42)
	value, err = helper.GetString("intkey")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != "42" {
		t.Errorf("Expected '42', got '%s'", value)
	}
}

func TestSessionStateHelper_Int(t *testing.T) {
	session := NewSession("session1", "testapp", "user1")
	helper := NewSessionStateHelper(session)

	// Test setting and getting int
	helper.SetInt("key1", 42)
	value, err := helper.GetInt("key1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != 42 {
		t.Errorf("Expected 42, got %d", value)
	}

	// Test default value
	defaultValue := helper.GetIntWithDefault("nonexistent", 100)
	if defaultValue != 100 {
		t.Errorf("Expected 100, got %d", defaultValue)
	}

	// Test increment
	newValue, err := helper.Increment("key1", 5)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if newValue != 47 {
		t.Errorf("Expected 47, got %d", newValue)
	}

	// Test decrement
	newValue, err = helper.Decrement("key1", 3)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if newValue != 44 {
		t.Errorf("Expected 44, got %d", newValue)
	}

	// Test string to int conversion
	session.SetState("strkey", "123")
	value, err = helper.GetInt("strkey")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != 123 {
		t.Errorf("Expected 123, got %d", value)
	}
}

func TestSessionStateHelper_Bool(t *testing.T) {
	session := NewSession("session1", "testapp", "user1")
	helper := NewSessionStateHelper(session)

	// Test setting and getting bool
	helper.SetBool("key1", true)
	value, err := helper.GetBool("key1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !value {
		t.Errorf("Expected true, got %v", value)
	}

	// Test default value
	defaultValue := helper.GetBoolWithDefault("nonexistent", false)
	if defaultValue {
		t.Errorf("Expected false, got %v", defaultValue)
	}

	// Test toggle
	newValue := helper.Toggle("key1")
	if newValue {
		t.Errorf("Expected false after toggle, got %v", newValue)
	}

	// Test string to bool conversion
	session.SetState("strkey", "true")
	value, err = helper.GetBool("strkey")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !value {
		t.Errorf("Expected true, got %v", value)
	}
}

func TestSessionStateHelper_Float(t *testing.T) {
	session := NewSession("session1", "testapp", "user1")
	helper := NewSessionStateHelper(session)

	// Test setting and getting float
	helper.SetFloat("key1", 3.14)
	value, err := helper.GetFloat("key1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != 3.14 {
		t.Errorf("Expected 3.14, got %f", value)
	}

	// Test default value
	defaultValue := helper.GetFloatWithDefault("nonexistent", 2.71)
	if defaultValue != 2.71 {
		t.Errorf("Expected 2.71, got %f", defaultValue)
	}

	// Test int to float conversion
	session.SetState("intkey", 42)
	value, err = helper.GetFloat("intkey")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != 42.0 {
		t.Errorf("Expected 42.0, got %f", value)
	}
}

func TestSessionStateHelper_Time(t *testing.T) {
	session := NewSession("session1", "testapp", "user1")
	helper := NewSessionStateHelper(session)

	now := time.Now()

	// Test setting and getting time
	helper.SetTime("key1", now)
	value, err := helper.GetTime("key1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !value.Equal(now) {
		t.Errorf("Expected %v, got %v", now, value)
	}

	// Test default value
	defaultTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	defaultValue := helper.GetTimeWithDefault("nonexistent", defaultTime)
	if !defaultValue.Equal(defaultTime) {
		t.Errorf("Expected %v, got %v", defaultTime, defaultValue)
	}

	// Test string to time conversion
	timeStr := now.Format(time.RFC3339)
	session.SetState("strkey", timeStr)
	value, err = helper.GetTime("strkey")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value.Format(time.RFC3339) != timeStr {
		t.Errorf("Expected %s, got %s", timeStr, value.Format(time.RFC3339))
	}
}

func TestSessionStateHelper_Slice(t *testing.T) {
	session := NewSession("session1", "testapp", "user1")
	helper := NewSessionStateHelper(session)

	// Test setting and getting slice
	slice := []any{"a", "b", "c"}
	helper.SetSlice("key1", slice)
	value, err := helper.GetSlice("key1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(value) != 3 || value[0] != "a" || value[1] != "b" || value[2] != "c" {
		t.Errorf("Expected [a b c], got %v", value)
	}

	// Test append
	err = helper.AppendToSlice("key1", "d")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	value, _ = helper.GetSlice("key1")
	if len(value) != 4 || value[3] != "d" {
		t.Errorf("Expected 4 items with 'd' at end, got %v", value)
	}

	// Test prepend
	err = helper.PrependToSlice("key1", "0")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	value, _ = helper.GetSlice("key1")
	if len(value) != 5 || value[0] != "0" {
		t.Errorf("Expected 5 items with '0' at start, got %v", value)
	}

	// Test remove
	err = helper.RemoveFromSlice("key1", 2) // Remove middle item
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	value, _ = helper.GetSlice("key1")
	if len(value) != 4 {
		t.Errorf("Expected 4 items after removal, got %d", len(value))
	}

	// Test pop
	item, err := helper.PopFromSlice("key1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if item != "d" {
		t.Errorf("Expected 'd', got %v", item)
	}
	value, _ = helper.GetSlice("key1")
	if len(value) != 3 {
		t.Errorf("Expected 3 items after pop, got %d", len(value))
	}
}

func TestSessionStateHelper_Map(t *testing.T) {
	session := NewSession("session1", "testapp", "user1")
	helper := NewSessionStateHelper(session)

	// Test setting and getting map
	m := map[string]any{"key1": "value1", "key2": 42}
	helper.SetMap("mapkey", m)
	value, err := helper.GetMap("mapkey")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(value) != 2 || value["key1"] != "value1" || value["key2"] != 42 {
		t.Errorf("Expected map with 2 items, got %v", value)
	}

	// Test setting map key
	err = helper.SetMapKey("mapkey", "key3", "value3")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	value, _ = helper.GetMap("mapkey")
	if len(value) != 3 || value["key3"] != "value3" {
		t.Errorf("Expected map with 3 items including key3, got %v", value)
	}

	// Test getting map key
	keyValue, err := helper.GetMapKey("mapkey", "key1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if keyValue != "value1" {
		t.Errorf("Expected 'value1', got %v", keyValue)
	}

	// Test deleting map key
	err = helper.DeleteMapKey("mapkey", "key2")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	value, _ = helper.GetMap("mapkey")
	if len(value) != 2 {
		t.Errorf("Expected map with 2 items after deletion, got %d", len(value))
	}
	if _, exists := value["key2"]; exists {
		t.Errorf("Expected key2 to be deleted")
	}
}

func TestSessionStateHelper_JSON(t *testing.T) {
	session := NewSession("session1", "testapp", "user1")
	helper := NewSessionStateHelper(session)

	// Test setting and getting JSON
	type TestStruct struct {
		Name  string `json:"name"`
		Age   int    `json:"age"`
		Email string `json:"email"`
	}

	original := TestStruct{Name: "John", Age: 30, Email: "john@example.com"}
	err := helper.SetJSON("jsonkey", original)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var retrieved TestStruct
	err = helper.GetJSON("jsonkey", &retrieved)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if retrieved.Name != original.Name || retrieved.Age != original.Age || retrieved.Email != original.Email {
		t.Errorf("Expected %+v, got %+v", original, retrieved)
	}
}

func TestSession_GetMetrics(t *testing.T) {
	session := NewSession("session1", "testapp", "user1")

	// Add some events
	event1 := &Event{
		ID:        "event1",
		Author:    "user",
		Timestamp: time.Now(),
		Content: &Content{
			Parts: []Part{
				{Text: ptr.Ptr("Hello")},
			},
		},
	}
	session.AddEvent(event1)

	event2 := &Event{
		ID:           "event2",
		Author:       "agent",
		Timestamp:    time.Now(),
		ErrorMessage: ptr.Ptr("Test error"),
	}
	session.AddEvent(event2)

	// Add some state
	session.SetState("key1", "value1")
	session.SetState("key2", 42)

	metrics := session.GetMetrics()

	if metrics.EventCount != 2 {
		t.Errorf("Expected 2 events, got %d", metrics.EventCount)
	}

	if metrics.StateSize != 2 {
		t.Errorf("Expected 2 state keys, got %d", metrics.StateSize)
	}

	if metrics.ErrorCount != 1 {
		t.Errorf("Expected 1 error, got %d", metrics.ErrorCount)
	}

	if metrics.AuthorEventCounts["user"] != 1 {
		t.Errorf("Expected 1 user event, got %d", metrics.AuthorEventCounts["user"])
	}

	if metrics.AuthorEventCounts["agent"] != 1 {
		t.Errorf("Expected 1 agent event, got %d", metrics.AuthorEventCounts["agent"])
	}
}

func TestSession_Snapshot(t *testing.T) {
	session := NewSession("session1", "testapp", "user1")

	// Add some state and events
	session.SetState("key1", "value1")
	session.SetState("key2", 42)

	event := &Event{
		ID:        "event1",
		Author:    "user",
		Timestamp: time.Now(),
	}
	session.AddEvent(event)

	// Create snapshot
	snapshot := session.CreateSnapshot()

	if snapshot.SessionID != session.ID {
		t.Errorf("Expected session ID %s, got %s", session.ID, snapshot.SessionID)
	}

	if len(snapshot.State) != 2 {
		t.Errorf("Expected 2 state keys in snapshot, got %d", len(snapshot.State))
	}

	if snapshot.EventCount != 1 {
		t.Errorf("Expected 1 event in snapshot, got %d", snapshot.EventCount)
	}

	if snapshot.LastEventID != "event1" {
		t.Errorf("Expected last event ID 'event1', got %s", snapshot.LastEventID)
	}

	// Modify session state
	session.SetState("key3", "value3")
	session.DeleteState("key1")

	// Restore from snapshot
	err := session.RestoreFromSnapshot(snapshot)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(session.State) != 2 {
		t.Errorf("Expected 2 state keys after restore, got %d", len(session.State))
	}

	if value, exists := session.GetState("key1"); !exists || value != "value1" {
		t.Errorf("Expected key1 to be restored to 'value1', got %v", value)
	}

	if _, exists := session.GetState("key3"); exists {
		t.Errorf("Expected key3 to be removed after restore")
	}
}

func TestSession_DiffState(t *testing.T) {
	session := NewSession("session1", "testapp", "user1")

	// Set initial state
	session.SetState("key1", "value1")
	session.SetState("key2", 42)
	session.SetState("key3", true)

	// Create other state to compare against
	otherState := map[string]any{
		"key1": "value1",   // Same
		"key2": 99,         // Modified
		"key4": "newvalue", // Removed from current
	}
	// key3 is missing from other (added to current)

	diff := session.DiffState(otherState)

	if len(diff.Added) != 1 || diff.Added["key3"] != true {
		t.Errorf("Expected key3 to be added, got %v", diff.Added)
	}

	if len(diff.Modified) != 1 || diff.Modified["key2"] != 42 {
		t.Errorf("Expected key2 to be modified, got %v", diff.Modified)
	}

	if len(diff.Removed) != 1 || diff.Removed[0] != "key4" {
		t.Errorf("Expected key4 to be removed, got %v", diff.Removed)
	}
}
