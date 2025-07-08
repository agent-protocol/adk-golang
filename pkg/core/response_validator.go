// Package core provides response validation utilities.
package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ResponseValidator validates LLM responses for proper structure.
type ResponseValidator struct {
	strictMode bool
}

// NewResponseValidator creates a new response validator.
func NewResponseValidator(strictMode bool) *ResponseValidator {
	return &ResponseValidator{
		strictMode: strictMode,
	}
}

// ValidateContent validates that content follows the expected structure.
func (v *ResponseValidator) ValidateContent(content *Content) error {
	if content == nil {
		return fmt.Errorf("content cannot be nil")
	}

	if content.Role == "" {
		return fmt.Errorf("content role is required")
	}

	validRoles := map[string]bool{
		"user":      true,
		"assistant": true,
		"agent":     true,
		"model":     true,
		"system":    true,
	}

	if !validRoles[content.Role] {
		return fmt.Errorf("invalid role: %s", content.Role)
	}

	if len(content.Parts) == 0 {
		return fmt.Errorf("content must have at least one part")
	}

	for i, part := range content.Parts {
		if err := v.validatePart(&part, i); err != nil {
			return fmt.Errorf("part %d validation failed: %w", i, err)
		}
	}

	return nil
}

// validatePart validates a single content part.
func (v *ResponseValidator) validatePart(part *Part, index int) error {
	if part.Type == "" {
		return fmt.Errorf("part type is required")
	}

	switch part.Type {
	case "text":
		if part.Text == nil {
			return fmt.Errorf("text part must have text field")
		}
		if v.strictMode && strings.TrimSpace(*part.Text) == "" {
			return fmt.Errorf("text part cannot be empty in strict mode")
		}

	case "function_call":
		if part.FunctionCall == nil {
			return fmt.Errorf("function_call part must have function_call field")
		}
		if err := v.validateFunctionCall(part.FunctionCall); err != nil {
			return fmt.Errorf("function call validation failed: %w", err)
		}

	case "function_response":
		if part.FunctionResponse == nil {
			return fmt.Errorf("function_response part must have function_response field")
		}
		if err := v.validateFunctionResponse(part.FunctionResponse); err != nil {
			return fmt.Errorf("function response validation failed: %w", err)
		}

	default:
		if v.strictMode {
			return fmt.Errorf("unknown part type: %s", part.Type)
		}
	}

	return nil
}

// validateFunctionCall validates function call structure.
func (v *ResponseValidator) validateFunctionCall(fc *FunctionCall) error {
	if fc.Name == "" {
		return fmt.Errorf("function call name is required")
	}

	if fc.Args == nil {
		fc.Args = make(map[string]any) // Initialize empty args
	}

	// Validate args is proper JSON-serializable
	if _, err := json.Marshal(fc.Args); err != nil {
		return fmt.Errorf("function call args must be JSON-serializable: %w", err)
	}

	return nil
}

// validateFunctionResponse validates function response structure.
func (v *ResponseValidator) validateFunctionResponse(fr *FunctionResponse) error {
	if fr.Name == "" {
		return fmt.Errorf("function response name is required")
	}

	if fr.Response == nil {
		return fmt.Errorf("function response must have response field")
	}

	// Validate response is proper JSON-serializable
	if _, err := json.Marshal(fr.Response); err != nil {
		return fmt.Errorf("function response must be JSON-serializable: %w", err)
	}

	return nil
}

// SanitizeContent cleans and validates content, fixing common issues.
func (v *ResponseValidator) SanitizeContent(content *Content) (*Content, error) {
	if content == nil {
		return nil, fmt.Errorf("content cannot be nil")
	}

	cleaned := &Content{
		Role:  content.Role,
		Parts: make([]Part, 0, len(content.Parts)),
	}

	for _, part := range content.Parts {
		cleanedPart, keep := v.sanitizePart(&part)
		if keep {
			cleaned.Parts = append(cleaned.Parts, *cleanedPart)
		}
	}

	if len(cleaned.Parts) == 0 {
		return nil, fmt.Errorf("no valid parts remaining after sanitization")
	}

	return cleaned, nil
}

// sanitizePart cleans a single part and returns whether to keep it.
func (v *ResponseValidator) sanitizePart(part *Part) (*Part, bool) {
	cleaned := &Part{
		Type:     part.Type,
		Metadata: part.Metadata,
	}

	switch part.Type {
	case "text":
		if part.Text != nil {
			text := strings.TrimSpace(*part.Text)
			// Filter out common malformed patterns
			if v.isValidText(text) {
				cleaned.Text = &text
				return cleaned, true
			}
		}
		return nil, false

	case "function_call":
		if part.FunctionCall != nil {
			cleaned.FunctionCall = part.FunctionCall
			// Ensure args is initialized
			if cleaned.FunctionCall.Args == nil {
				cleaned.FunctionCall.Args = make(map[string]any)
			}
			return cleaned, true
		}
		return nil, false

	case "function_response":
		if part.FunctionResponse != nil {
			cleaned.FunctionResponse = part.FunctionResponse
			return cleaned, true
		}
		return nil, false

	default:
		// Keep unknown types as-is if not in strict mode
		if !v.strictMode {
			*cleaned = *part
			return cleaned, true
		}
		return nil, false
	}
}

// isValidText checks if text content is valid (not malformed JSON fragments).
func (v *ResponseValidator) isValidText(text string) bool {
	if text == "" {
		return false
	}

	// Filter out common malformed JSON patterns
	malformedPatterns := []string{
		`"},"`,
		`}},`,
		`"]`,
		`"parameters"`,
	}

	for _, pattern := range malformedPatterns {
		if strings.Contains(text, pattern) && len(text) < 50 {
			return false
		}
	}

	// Check if it's just brackets or quotes
	trimmed := strings.Trim(text, `"{}[],:`)
	return len(trimmed) > 0
}
