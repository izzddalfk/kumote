package handlers

import "time"

// This structure follow Telegram webhook response
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"` // this property not available for Telegram
	Message   string      `json:"message,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

func NewSuccessResponse(data interface{}) *APIResponse {
	return &APIResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}
}

func NewErrorResponse(message string) *APIResponse {
	return &APIResponse{
		Success:   false,
		Message:   message,
		Timestamp: time.Now().Unix(),
	}
}
