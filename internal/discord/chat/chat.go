package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sprout/pkg/x"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

const (
	MAX_GEN_MSG_TOKENS = 4096
	MAX_INT_MSG_TOKENS = MAX_GEN_MSG_TOKENS / 2
	ACTIVE_TIMEOUT     = 1 * time.Minute
	OLLAMA_CHAT_URL    = "http://Peridot:11434/api/chat" // e.g., "http://127.0.0.1:11434/api/chat"
	MAX_OUT_LENGTH     = 2000
)

var WAKE_PHRASES = []string{
	"halsey",
	"Halsey",
	"hals",
	"Hals",
}

type Message struct {
	ID      snowflake.ID `json:"-"`
	Role    string       `json:"role"`
	Content string       `json:"content"` // processed, for ollama
	Created time.Time    `json:"-"`       // required, derived from discord message
}

func ParseUserMessage(msg *discord.Message, client *bot.Client) Message {
	content := fmt.Sprintf("[id=%s]", msg.ID)
	if msg.ReferencedMessage != nil {
		content += fmt.Sprintf("[reply_to=%s]", msg.ReferencedMessage.ID)
	}
	if len(msg.Attachments) > 0 {
		content += fmt.Sprintf("[attachments=%d]", len(msg.Attachments))
	}
	if msg.Poll != nil {
		content += "[poll]"
	}
	content += fmt.Sprintf(" %s: %s", msg.Author.Username, msg.Content)
	return Message{
		ID:      msg.ID,
		Role:    x.Ternary(msg.Author.ID == client.ApplicationID, "assistant", "user"),
		Content: content,
		Created: msg.CreatedAt,
	}
}

type ChannelState struct {
	mu          sync.Mutex
	buf         []Message
	activeUntil time.Time // used to represent chats with recent chatbot involvement. If time.Now() < activeUntil, skip medium cost Intent Classifier check
	newestMsg   time.Time
	lastSeen    time.Time // used to determine if there have been new messages since last response / rejection
}

type ChatManager struct {
	mu       sync.RWMutex
	channels map[snowflake.ID]*ChannelState
	client   *bot.Client
	botName  string // cached bot name
	log      *xlog.Logger
	ctx      context.Context
}

func NewChatManager(ctx context.Context, closeWG *sync.WaitGroup, log *xlog.Logger) *ChatManager {
	cm := &ChatManager{
		channels: make(map[snowflake.ID]*ChannelState),
		log:      log,
		ctx:      ctx,
	}

	closeWG.Add(1)
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer closeWG.Done()
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cm.tick()
			}
		}
	}()

	return cm
}

func (cm *ChatManager) SetClient(client *bot.Client) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.client = client
	app, err := cm.client.Rest.GetBotApplicationInfo()
	if err != nil {
		cm.log.Errorf("Failed to get bot application info: %v", err)
		return
	}
	cm.botName = app.Name
}

func (cm *ChatManager) UpsertChannelMessages(channelID snowflake.ID, fn func([]Message) []Message) {
	cm.mu.Lock()
	if _, ok := cm.channels[channelID]; !ok {
		cm.channels[channelID] = &ChannelState{buf: make([]Message, 0)}
	}
	channel := cm.channels[channelID]
	cm.mu.Unlock()

	channel.mu.Lock()
	defer channel.mu.Unlock()

	// update messages, evict old messages
	channel.buf = evictToBudget(fn(channel.buf), MAX_GEN_MSG_TOKENS)

	// update newest message time
	if len(channel.buf) > 0 {
		channel.newestMsg = channel.buf[len(channel.buf)-1].Created
	}
}

func (cm *ChatManager) tick() {
	cm.mu.RLock()

	if cm.client == nil {
		cm.mu.RUnlock()
		return
	}

	for channelID, channel := range cm.channels {
		// get snapshot of messages
		msgs := channel.shouldRespond()
		if msgs == nil {
			continue
		}

		cm.log.Debugf("Escalating channel %s with %d messages", channelID, len(msgs))

		// intent classification, trim messages to MAX_INT_MSG_TOKENS
		intentMsgs := evictToBudget(msgs, MAX_INT_MSG_TOKENS)

		intentResp, err := cm.classifyIntent(intentMsgs)
		if err != nil {
			cm.log.Errorf("Failed to classify intent for channel %s: %v", channelID, err)
			continue
		}

		cm.log.Debugf("Intent decision for channel %s, Respond: %v, Confidence: %f, Reason: %s",
			channelID, intentResp.Respond, intentResp.Confidence, intentResp.Reason)

		if !intentResp.Respond {
			continue
		}

		// update activeUntil
		channel.mu.Lock()
		channel.activeUntil = channel.newestMsg.Add(ACTIVE_TIMEOUT)
		channel.mu.Unlock()

		// send typing indicator
		if err := cm.client.Rest.SendTyping(channelID); err != nil {
			cm.log.Errorf("Failed to send typing indicator for channel %s: %v", channelID, err)
		}

		// response generation
		response, err := cm.generateResponse(msgs)
		if err != nil {
			cm.log.Errorf("Failed to generate response for channel %s: %v", channelID, err)
			continue
		}
		response = SanitizeResponse(response, cm.botName)
		if len(response) == 0 {
			cm.log.Debugf("Response for channel %s was empty", channelID)
			continue
		}
		if len(response) > MAX_OUT_LENGTH {
			cm.log.Debugf("Response for channel %s was too long, truncating", channelID)
			response = response[:MAX_OUT_LENGTH]
		}
		msgBuild := discord.NewMessageCreateBuilder().SetContent(response)

		// if there were new msgs during response generation, send as a reply to latest snapshot msg, else send raw
		channel.mu.Lock()
		snapshotTime := msgs[len(msgs)-1].Created
		newMsgs := channel.newestMsg.After(snapshotTime)
		channel.mu.Unlock()
		if newMsgs {
			msgBuild.SetMessageReference(&discord.MessageReference{MessageID: &msgs[len(msgs)-1].ID})
		}

		resMsg, err := cm.client.Rest.CreateMessage(channelID, msgBuild.Build())
		if err != nil {
			cm.log.Errorf("Failed to send message to channel %s: %v", channelID, err)
			continue
		}
		// insert res immediately into channel buf
		cm.mu.RUnlock()
		cm.UpsertChannelMessages(channelID, func(buf []Message) []Message {
			if !slices.ContainsFunc(buf, func(m Message) bool {
				return m.ID == resMsg.ID
			}) {
				return append(buf, ParseUserMessage(resMsg, cm.client))
			}
			return buf
		})
		cm.mu.RLock()
	}
	cm.mu.RUnlock()
}

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

func (cm *ChatManager) classifyIntent(msgs []Message) (*IntentResponse, error) {
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
		Model:    "gpt-oss:20b",
		Messages: ollamaMsgs,
		Stream:   false,
		Format:   "json",
	}

	respBody, err := cm.callOllama(reqBody)
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

func (cm *ChatManager) generateResponse(msgs []Message) (string, error) {
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
		Model:    "gpt-oss:20b",
		Messages: ollamaMsgs,
		Stream:   false,
	}

	respBody, err := cm.callOllama(reqBody)
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

func (cm *ChatManager) callOllama(req OllamaRequest) ([]byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(cm.ctx, 10*time.Minute)
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

// shouldRespond checks if the channel needs a response and returns the messages
// to use for intent classification. Returns nil if no response is needed.
func (cs *ChannelState) shouldRespond() []Message {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// no messages
	if len(cs.buf) == 0 || cs.newestMsg.IsZero() {
		return nil
	}
	// no new messages since last seen
	if !cs.lastSeen.IsZero() && (cs.newestMsg.Equal(cs.lastSeen) || cs.newestMsg.Before(cs.lastSeen)) {
		return nil
	}
	// skip if the newest message is from the assistant (prevents re-classifying after bot responds)
	if len(cs.buf) > 0 && cs.buf[len(cs.buf)-1].Role == "assistant" {
		return nil
	}

	cs.lastSeen = cs.newestMsg

	// if activeUntil is set and has not passed, return all messages
	if !cs.activeUntil.IsZero() && time.Now().Before(cs.activeUntil) {
		return slices.Clone(cs.buf)
	}

	// check for wake phrase in new messages
	woke := false
	for _, msg := range cs.buf {
		if !cs.lastSeen.IsZero() && msg.Created.Before(cs.lastSeen) {
			continue
		}
		for _, phrase := range WAKE_PHRASES {
			if strings.Contains(msg.Content, phrase) {
				woke = true
				break
			}
		}
		if woke {
			break
		}
	}
	if !woke {
		return nil
	}

	return slices.Clone(cs.buf)
}

func evictToBudget(buf []Message, maxTokens int) []Message {
	total := 0
	for i := len(buf) - 1; i >= 0; i-- {
		total += buf[i].EstimateTokens()
		if total > maxTokens {
			return buf[i+1:]
		}
	}
	return buf
}

// fast conservative estimate of tokens
func (m *Message) EstimateTokens() int {
	return (len([]byte(m.Content)) + 2) / 3 // integer ceil
}

// SanitizeResponse cleans LLM hallucinations of internal protocol tags and self-identification.
func SanitizeResponse(input string, botName string) string {
	remaining := strings.TrimLeftFunc(input, unicode.IsSpace)

	// 1. Iteratively strip [tags] from the start of the string.
	// We loop because the LLM might output multiple tags: [id=123][tombstone] ...
	for strings.HasPrefix(remaining, "[") {
		end := strings.Index(remaining, "]")
		if end == -1 {
			break // Open bracket with no close? Unlikely system tag. Stop processing.
		}
		remaining = remaining[end+1:]                                // Slice off the tag
		remaining = strings.TrimLeftFunc(remaining, unicode.IsSpace) // Trim space again to catch "[tag] [tag]" vs "[tag][tag]"
	}

	// 2. Check for the specific Bot Name prefix
	namePrefix := botName + ":"
	// case-insensitive check if the LLM gets sloppy
	// if we need strict adherence, use strings.CutPrefix
	if len(remaining) >= len(namePrefix) {
		if strings.EqualFold(remaining[:len(namePrefix)], namePrefix) {
			remaining = remaining[len(namePrefix):]
		}
	}

	// 3. Final cleanup of the actual message content
	return strings.TrimSpace(remaining)
}
