package chat

import (
	"context"
	"fmt"
	"slices"
	"sprout/internal/platform/database"
	"sprout/pkg/x"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Data-Corruption/lmdb-go/wrap"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

const (
	MAX_GEN_MSG_TOKENS = 4096
	MAX_INT_MSG_TOKENS = MAX_GEN_MSG_TOKENS / 2
	ACTIVE_TIMEOUT     = 2 * time.Minute
	MAX_OUT_LENGTH     = 2000
)

var wakePhrases = []string{"halsey", "hals"}

type Message struct {
	ID      snowflake.ID `json:"-"`
	UserID  snowflake.ID `json:"-"`
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
		UserID:  msg.Author.ID,
		Role:    x.Ternary(msg.Author.ID == client.ApplicationID, "assistant", "user"),
		Content: content,
		Created: msg.CreatedAt,
	}
}

// fast conservative estimate of tokens
func (m *Message) EstimateTokens() int {
	return (len([]byte(m.Content)) + 2) / 3 // integer ceil
}

// findLastUserMsg returns the last message with role "user" and true, or zero Message and false if none found.
func findLastUserMsg(msgs []Message) (Message, bool) {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i], true
		}
	}
	return Message{}, false
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

type ChannelState struct {
	mu            sync.Mutex
	buf           []Message
	guildID       snowflake.ID
	activeUntil   time.Time // used to represent chats with recent chatbot involvement. If time.Now() < activeUntil, skip medium cost Intent Classifier check
	newestUserMsg time.Time // newest user message only, for determining if we need to respond
	lastSeen      time.Time // timestamp of newest user message when we last processed
}

// shouldRespond checks if the channel needs a response and returns the messages
// to use for intent classification. Returns nil if no response is needed.
func (cs *ChannelState) shouldRespond() []Message {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// no messages or no user messages
	if len(cs.buf) == 0 || cs.newestUserMsg.IsZero() {
		return nil
	}
	// no new user messages since last seen
	if !cs.lastSeen.IsZero() && !cs.newestUserMsg.After(cs.lastSeen) {
		return nil
	}

	cs.lastSeen = cs.newestUserMsg

	// if activeUntil is set and has not passed, return all messages
	if !cs.activeUntil.IsZero() && time.Now().Before(cs.activeUntil) {
		return slices.Clone(cs.buf)
	}

	// check for wake phrase in new messages (case-insensitive)
	woke := false
	for _, msg := range cs.buf {
		if !cs.lastSeen.IsZero() && msg.Created.Before(cs.lastSeen) {
			continue
		}
		contentLower := strings.ToLower(msg.Content)
		for _, phrase := range wakePhrases {
			if strings.Contains(contentLower, phrase) {
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

type ChatManager struct {
	mu        sync.RWMutex
	channels  map[snowflake.ID]*ChannelState
	client    *bot.Client
	botName   string // cached bot name
	ollamaURL string // base URL for Ollama API
	db        *wrap.DB
	log       *xlog.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	closeWG   *sync.WaitGroup // wait group for active work
}

func NewChatManager(db *wrap.DB, log *xlog.Logger) *ChatManager {
	ctx, cancel := context.WithCancel(context.Background())

	// get ollama URL from config, use default if not set
	ollamaURL := DEFAULT_OLLAMA_URL
	if cfg, err := database.ViewConfig(db); err == nil && cfg.OllamaURL != "" {
		ollamaURL = cfg.OllamaURL
	}

	cm := &ChatManager{
		channels:  make(map[snowflake.ID]*ChannelState),
		ollamaURL: ollamaURL,
		db:        db,
		log:       log,
		ctx:       ctx,
		cancel:    cancel,
		closeWG:   &sync.WaitGroup{},
	}

	cm.closeWG.Add(1)
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		defer cm.closeWG.Done()

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

func (cm *ChatManager) Close() error {
	cm.cancel() // cancel context to stop ticker and abort in-flight Ollama calls
	cm.closeWG.Wait()
	return nil
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

func (cm *ChatManager) UpsertChannelMessages(channelID snowflake.ID, guildID snowflake.ID, fn func([]Message) []Message) {
	cm.mu.Lock()
	if _, ok := cm.channels[channelID]; !ok {
		cm.channels[channelID] = &ChannelState{buf: make([]Message, 0), guildID: guildID}
	}
	channel := cm.channels[channelID]
	cm.mu.Unlock()

	channel.mu.Lock()
	defer channel.mu.Unlock()

	// update messages
	newBuf := fn(channel.buf)

	// replace messages of users lacking access or opted out with
	// {"role":"system","content":"A message from another user occurred here, but its content is unavailable."}
	users, err := database.ViewUsers(cm.db)
	if err != nil {
		cm.log.Errorf("Failed to get users: %v", err)
		return
	}
	for i := range newBuf {
		if newBuf[i].Role != "user" {
			continue
		}
		for _, user := range users {
			if user.ID == newBuf[i].UserID {
				if !user.User.AiAccess || user.User.AiChatOptOut {
					newBuf[i].Content = "A message from another user occurred here, but its content is unavailable."
					newBuf[i].Role = "system"
					newBuf[i].UserID = 0
				}
				break
			}
		}
	}

	channel.buf = evictToBudget(newBuf, MAX_GEN_MSG_TOKENS)

	// update newest user message time
	if len(channel.buf) > 0 {
		if msg, ok := findLastUserMsg(channel.buf); ok {
			channel.newestUserMsg = msg.Created
		}
	}
}

// workItem holds the data needed to process a single channel response.
type workItem struct {
	channelID snowflake.ID
	channel   *ChannelState
	msgs      []Message
}

func (cm *ChatManager) tick() {
	// Collect work items while holding the lock briefly
	var work []workItem

	cm.mu.RLock()
	if cm.client == nil {
		cm.mu.RUnlock()
		return
	}

	for channelID, channel := range cm.channels {
		msgs := channel.shouldRespond()
		if msgs != nil {
			work = append(work, workItem{channelID: channelID, channel: channel, msgs: msgs})
		}
	}
	cm.mu.RUnlock()

	guilds, err := database.ViewAllGuildsWithChannels(cm.db)
	if err != nil {
		cm.log.Errorf("Failed to get guilds: %v", err)
		return
	}

	// Process each work item without holding the manager lock
	// This allows new messages to be added during LLM generation
	for _, w := range work {
		var wGuild database.GuildWithID
		for _, guild := range guilds {
			if guild.ID == w.channel.guildID {
				wGuild = guild
				break
			}
		}
		if wGuild.ID == 0 || !wGuild.Guild.AiChatEnabled {
			continue
		}
		for _, channel := range wGuild.Channels {
			if channel.ID == w.channelID {
				if channel.Channel.AiChat {
					cm.processChannel(w)
					break
				}
			}
		}
	}
}

func (cm *ChatManager) processChannel(w workItem) {
	cm.log.Debugf("Processing channel %s with %d messages", w.channelID, len(w.msgs))

	// intent classification, trim messages to MAX_INT_MSG_TOKENS
	intentMsgs := evictToBudget(w.msgs, MAX_INT_MSG_TOKENS)

	intentResp, err := cm.classifyIntent(cm.ctx, intentMsgs)
	if err != nil {
		cm.log.Errorf("Failed to classify intent for channel %s: %v", w.channelID, err)
		return
	}

	cm.log.Debugf("Intent decision for channel %s: %s", w.channelID, intentResp)

	if !intentResp.Respond {
		return
	}

	// update activeUntil based on the newest message in buffer
	w.channel.mu.Lock()
	if len(w.channel.buf) > 0 {
		w.channel.activeUntil = w.channel.buf[len(w.channel.buf)-1].Created.Add(ACTIVE_TIMEOUT)
	}
	w.channel.mu.Unlock()

	// send typing indicator
	cm.mu.RLock()
	client := cm.client
	cm.mu.RUnlock()
	if err := client.Rest.SendTyping(w.channelID); err != nil {
		cm.log.Errorf("Failed to send typing indicator for channel %s: %v", w.channelID, err)
	}

	// response generation
	response, err := cm.generateResponse(cm.ctx, w.msgs)
	if err != nil {
		cm.log.Errorf("Failed to generate response for channel %s: %v", w.channelID, err)
		return
	}
	cm.mu.RLock()
	response = SanitizeResponse(response, cm.botName)
	cm.mu.RUnlock()
	if len(response) == 0 {
		cm.log.Debugf("Response for channel %s was empty", w.channelID)
		return
	}
	if len(response) > MAX_OUT_LENGTH {
		cm.log.Debugf("Response for channel %s was too long, truncating", w.channelID)
		response = response[:MAX_OUT_LENGTH]
	}
	msgBuild := discord.NewMessageCreateBuilder().SetContent(response)

	// if there were new user msgs during response generation, send as a reply to latest snapshot msg, else send raw
	w.channel.mu.Lock()
	lastUserMsg, hasUserMsg := findLastUserMsg(w.msgs)
	newMsgs := hasUserMsg && w.channel.newestUserMsg.After(lastUserMsg.Created)
	cm.log.Debugf("Reply check for channel %s: snapshotUserTime=%v, newestUserMsg=%v, newMsgs=%v",
		w.channelID, lastUserMsg.Created, w.channel.newestUserMsg, newMsgs)
	w.channel.mu.Unlock()
	if newMsgs {
		msgBuild.SetMessageReference(&discord.MessageReference{MessageID: &lastUserMsg.ID})
	}

	cm.mu.RLock()
	client = cm.client
	resMsg, err := client.Rest.CreateMessage(w.channelID, msgBuild.Build())
	cm.mu.RUnlock()
	if err != nil {
		cm.log.Errorf("Failed to send message to channel %s: %v", w.channelID, err)
		return
	}

	// insert res immediately into channel buf
	cm.UpsertChannelMessages(w.channelID, w.channel.guildID, func(buf []Message) []Message {
		if !slices.ContainsFunc(buf, func(m Message) bool {
			return m.ID == resMsg.ID
		}) {
			cm.mu.RLock()
			msg := ParseUserMessage(resMsg, cm.client)
			cm.mu.RUnlock()
			return append(buf, msg)
		}
		return buf
	})
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
