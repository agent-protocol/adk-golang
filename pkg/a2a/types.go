package a2a

// SendMessageRequest represents a request to send a message to an agent.
type SendMessageRequest struct {
	AgentName string   `json:"agent_name"`
	Message   *Message `json:"message"`
}

// SendMessageResponse represents the response from sending a message.
type SendMessageResponse struct {
	TaskID string `json:"task_id"`
}

// GetAgentCardRequest represents a request to get an agent card.
type GetAgentCardRequest struct {
	AgentName string `json:"agent_name"`
}

// GetAgentCardResponse represents the response containing an agent card.
type GetAgentCardResponse struct {
	AgentCard AgentCard `json:"agent_card"`
}
