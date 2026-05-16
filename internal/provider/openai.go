package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/thalha-dev/ennacmd/internal/ai"
	"github.com/thalha-dev/ennacmd/internal/config"
)

type OpenAICompatible struct {
	Config config.Config
	Client *http.Client
}

type openAIRequest struct {
	Model       string       `json:"model"`
	Messages    []ai.Message `json:"messages"`
	Temperature float64      `json:"temperature"`
	Stream      bool         `json:"stream"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type openAIStreamPayload struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func (o *OpenAICompatible) Complete(ctx context.Context, prompt ai.Prompt) (*ai.Response, error) {
	body, err := json.Marshal(openAIRequest{
		Model:       prompt.Model,
		Messages:    prompt.Messages,
		Temperature: prompt.Temperature,
		Stream:      false,
	})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(o.Config.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	o.applyHeaders(req)

	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if err := ensureSuccess(resp); err != nil {
		return nil, err
	}

	var payload openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(payload.Choices) == 0 {
		return &ai.Response{}, nil
	}

	return &ai.Response{Content: strings.TrimSpace(payload.Choices[0].Message.Content)}, nil
}

func (o *OpenAICompatible) Stream(ctx context.Context, prompt ai.Prompt) (<-chan ai.StreamEvent, error) {
	body, err := json.Marshal(openAIRequest{
		Model:       prompt.Model,
		Messages:    prompt.Messages,
		Temperature: prompt.Temperature,
		Stream:      true,
	})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(o.Config.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	o.applyHeaders(req)
	req.Header.Set("Accept", "text/event-stream")

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
			if line == "" || !strings.HasPrefix(line, "data:") {
				continue
			}

			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" {
				ch <- ai.StreamEvent{Done: true}
				return
			}

			var item openAIStreamPayload
			if err := json.Unmarshal([]byte(payload), &item); err != nil {
				ch <- ai.StreamEvent{Err: fmt.Errorf("decode stream payload: %w", err)}
				return
			}
			if len(item.Choices) == 0 {
				continue
			}
			if delta := item.Choices[0].Delta.Content; delta != "" {
				ch <- ai.StreamEvent{Delta: delta}
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

func (o *OpenAICompatible) applyHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.Config.APIKey)
	req.Header.Set("User-Agent", "ennacmd/0.1")
	if strings.EqualFold(o.Config.Provider, "openrouter") {
		req.Header.Set("HTTP-Referer", "https://ennacmd.local")
		req.Header.Set("X-Title", "ennacmd")
	}
}

func ensureSuccess(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	message := providerErrorMessage(body)
	if message == "" {
		return fmt.Errorf("provider request failed: %s", resp.Status)
	}
	return fmt.Errorf("provider request failed (%s): %s", resp.Status, message)
}

func providerErrorMessage(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}

	var stringEnvelope struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &stringEnvelope); err == nil && strings.TrimSpace(stringEnvelope.Error) != "" {
		return strings.TrimSpace(stringEnvelope.Error)
	}

	var nestedEnvelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &nestedEnvelope); err == nil && strings.TrimSpace(nestedEnvelope.Error.Message) != "" {
		return strings.TrimSpace(nestedEnvelope.Error.Message)
	}

	return trimmed
}
