package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/thalha-dev/ennacmd/internal/ai"
	"github.com/thalha-dev/ennacmd/internal/config"
)

type Ollama struct {
	Config config.Config
	Client *http.Client
}

type ollamaRequest struct {
	Model    string        `json:"model"`
	Messages []ai.Message  `json:"messages"`
	Stream   bool          `json:"stream"`
	Options  ollamaOptions `json:"options"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature"`
}

type ollamaResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type ollamaStreamResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

func (o *Ollama) Complete(ctx context.Context, prompt ai.Prompt) (*ai.Response, error) {
	body, err := json.Marshal(ollamaRequest{
		Model:    prompt.Model,
		Messages: prompt.Messages,
		Stream:   false,
		Options:  ollamaOptions{Temperature: prompt.Temperature},
	})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(o.Config.BaseURL, "/")+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if err := ensureSuccess(resp); err != nil {
		return nil, err
	}

	var payload ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &ai.Response{Content: strings.TrimSpace(payload.Message.Content)}, nil
}

func (o *Ollama) Stream(ctx context.Context, prompt ai.Prompt) (<-chan ai.StreamEvent, error) {
	body, err := json.Marshal(ollamaRequest{
		Model:    prompt.Model,
		Messages: prompt.Messages,
		Stream:   true,
		Options:  ollamaOptions{Temperature: prompt.Temperature},
	})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(o.Config.BaseURL, "/")+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if err := ensureSuccess(resp); err != nil {
		_ = resp.Body.Close()
		return nil, err
	}

	ch := make(chan ai.StreamEvent)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		buffer := make([]byte, 0, 64*1024)
		scanner.Buffer(buffer, 1024*1024)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			var payload ollamaStreamResponse
			if err := json.Unmarshal([]byte(line), &payload); err != nil {
				ch <- ai.StreamEvent{Err: fmt.Errorf("decode stream payload: %w", err)}
				return
			}
			if payload.Message.Content != "" {
				ch <- ai.StreamEvent{Delta: payload.Message.Content}
			}
			if payload.Done {
				ch <- ai.StreamEvent{Done: true}
				return
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- ai.StreamEvent{Err: fmt.Errorf("read stream: %w", err)}
			return
		}

		ch <- ai.StreamEvent{Done: true}
	}()

	return ch, nil
}
