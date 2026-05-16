package ai

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type Prompt struct {
	Messages    []Message
	Temperature float64
	Model       string
	Stream      bool
}

type Response struct {
	Content string
}

type StreamEvent struct {
	Delta string
	Done  bool
	Err   error
}

type Provider interface {
	Complete(ctx context.Context, prompt Prompt) (*Response, error)
	Stream(ctx context.Context, prompt Prompt) (<-chan StreamEvent, error)
}
