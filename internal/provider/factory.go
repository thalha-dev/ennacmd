package provider

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/thalha-dev/ennacmd/internal/ai"
	"github.com/thalha-dev/ennacmd/internal/config"
)

func New(cfg config.Config) (ai.Provider, error) {
	httpClient := &http.Client{Timeout: 90 * time.Second}
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "openai", "openrouter":
		return &OpenAICompatible{Config: cfg, Client: httpClient}, nil
	case "ollama":
		return &Ollama{Config: cfg, Client: httpClient}, nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}
