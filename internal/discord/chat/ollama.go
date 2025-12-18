package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	OLLAMA_CHAT_URL = "http://Peridot:11434/api/chat" // e.g., "http://127.0.0.1:11434/api/chat"
	MODEL_NAME      = "gpt-oss:20b"
)

type OllamaRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Format   string          `json:"format,omitempty"`
}

type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type IntentResponse struct {
	Respond    bool    `json:"respond"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

func (ir IntentResponse) String() string {
	return fmt.Sprintf("Respond=%v, Confidence=%.2f, Reason=%s", ir.Respond, ir.Confidence, ir.Reason)
}

func callOllama(ctx context.Context, req OllamaRequest) ([]byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", OLLAMA_CHAT_URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama API returned status: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func classifyIntent(ctx context.Context, msgs []Message) (*IntentResponse, error) {
	ollamaMsgs := make([]OllamaMessage, 0, len(msgs)+1)
	ollamaMsgs = append(ollamaMsgs, OllamaMessage{
		Role:    "system",
		Content: PromptIntentClassifier,
	})
	for _, m := range msgs {
		ollamaMsgs = append(ollamaMsgs, OllamaMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	reqBody := OllamaRequest{
		Model:    MODEL_NAME,
		Messages: ollamaMsgs,
		Stream:   false,
		Format:   "json",
	}

	respBody, err := callOllama(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Message OllamaMessage `json:"message"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("%w, raw response: %s", err, string(respBody))
	}

	var intent IntentResponse
	if err := json.Unmarshal([]byte(parsed.Message.Content), &intent); err != nil {
		return nil, fmt.Errorf("%w, llm output: %s", err, parsed.Message.Content)
	}

	return &intent, nil
}

func generateResponse(ctx context.Context, msgs []Message) (string, error) {
	ollamaMsgs := make([]OllamaMessage, 0, len(msgs)+2)
	ollamaMsgs = append(ollamaMsgs, OllamaMessage{
		Role:    "system",
		Content: PromptResponseGen,
	})
	ollamaMsgs = append(ollamaMsgs, OllamaMessage{
		Role:    "system",
		Content: PromptRuntime,
	})
	for _, m := range msgs {
		ollamaMsgs = append(ollamaMsgs, OllamaMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	reqBody := OllamaRequest{
		Model:    MODEL_NAME,
		Messages: ollamaMsgs,
		Stream:   false,
	}

	respBody, err := callOllama(ctx, reqBody)
	if err != nil {
		return "", err
	}

	var parsed struct {
		Message OllamaMessage `json:"message"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}

	return parsed.Message.Content, nil
}
