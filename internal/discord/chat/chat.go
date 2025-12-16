package chat

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/disgoorg/snowflake/v2"
)

const (
	MAX_GEN_MSG_TOKENS = 4096
	MAX_INT_MSG_TOKENS = MAX_GEN_MSG_TOKENS / 2
	ACTIVE_TIMEOUT     = 2 * time.Minute
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

type ChannelState struct {
	mu          sync.Mutex
	buf         []Message
	activeUntil time.Time // used to represent chats with recent chatbot involvement. If time.Now() < activeUntil, skip medium cost Intent Classifier check
	newestMsg   time.Time
	lastRes     time.Time // used to determine if there have been new messages since last response
}

type ChatManager struct {
	mu       sync.RWMutex
	channels map[snowflake.ID]*ChannelState
}

func NewChatManager(ctx context.Context, closeWG *sync.WaitGroup) *ChatManager {
	cm := &ChatManager{channels: make(map[snowflake.ID]*ChannelState)}

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
	defer cm.mu.RUnlock()

	for _, channel := range cm.channels {
		msgs := channel.shouldRespond()
		if msgs == nil {
			continue
		}

		// TODO: Intent classification
		// trim messages to MAX_INT_MSG_TOKENS
		// generate intent classification
		// if respond is true
		// lock channel mu
		// set lastRes and activeUntil
		// unlock channel mu

		// TODO: Response generation
		// generate response
		// if new messages since lastRes, send response raw, else send as a reply to lastRes msg
	}
}

// shouldRespond checks if the channel needs a response and returns the messages
// to use for intent classification. Returns nil if no response is needed.
func (cs *ChannelState) shouldRespond() []Message {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// no messages
	if cs.newestMsg.IsZero() {
		return nil
	}
	// no new messages since last response
	if !cs.lastRes.IsZero() && (cs.newestMsg.Equal(cs.lastRes) || cs.newestMsg.Before(cs.lastRes)) {
		return nil
	}

	// check for wake phrase in new messages
	woke := false
	for _, msg := range cs.buf {
		if msg.Created.After(cs.lastRes) {
			if slices.Contains(WAKE_PHRASES, msg.Content) {
				woke = true
				break
			}
		}
	}

	if !woke && time.Now().After(cs.activeUntil) {
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

// Fast conservative estimate of tokens
func (m *Message) EstimateTokens() int {
	return (len([]byte(m.Content)) + 2) / 3 // integer ceil
}
