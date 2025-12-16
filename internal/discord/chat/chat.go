package chat

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/disgoorg/snowflake/v2"
)

const (
	MAX_GEN_MSG_TOKENS    = 4096
	MAX_INTENT_MSG_TOKENS = MAX_GEN_MSG_TOKENS / 2
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
	if _, ok := cm.channels[channelID]; !ok {
		cm.channels[channelID] = &ChannelState{buf: make([]Message, 0)}
	}

	cm.channels[channelID].mu.Lock()
	defer cm.channels[channelID].mu.Unlock()

	// update messages, evict old messages
	cm.channels[channelID].buf = evictToBudget(fn(cm.channels[channelID].buf), MAX_GEN_MSG_TOKENS)

	// update newest message time
	if len(cm.channels[channelID].buf) > 0 {
		cm.channels[channelID].newestMsg = cm.channels[channelID].buf[len(cm.channels[channelID].buf)-1].Created
	}
}

func (cm *ChatManager) tick() {
	for _, channel := range cm.channels {
		channel.mu.Lock()
		defer channel.mu.Unlock()

		// no messages
		if channel.newestMsg.IsZero() {
			continue
		}
		// no new messages since last response
		if !channel.lastRes.IsZero() && channel.newestMsg.Equal(channel.lastRes) || channel.newestMsg.Before(channel.lastRes) {
			continue
		}

		// create smaller sub buf for intent classification
		inMsgs := evictToBudget(channel.buf, MAX_INTENT_MSG_TOKENS)

		// if no wake phrase detected in new messages and not active, skip
		woke := false
		for _, msg := range inMsgs {
			if msg.Created.After(channel.lastRes) {
				if slices.Contains(WAKE_PHRASES, msg.Content) {
					woke = true
					break
				}
			}
		}
		if !woke && time.Now().After(channel.activeUntil) {
			continue
		}

		// TODO: Intent classification

		// TODO: Response generation

		fmt.Println("hi") // TODO: Remove
	}
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
