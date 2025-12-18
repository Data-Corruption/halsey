package listeners

import (
	"slices"
	"sprout/internal/app"
	"sprout/internal/discord/chat"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func OnGuildMessageCreate(a *app.App, event *events.GuildMessageCreate) {
	a.DiscordWG.Add(1) // track for graceful shutdown

	// acquire semaphore
	select {
	case a.DiscordEventLimiter <- struct{}{}:
	default:
		a.DiscordWG.Done()
		a.Log.Warn("Event limiter reached, dropping guild message create")
		return
	}

	go func() {
		defer a.DiscordWG.Done()
		defer func() { <-a.DiscordEventLimiter }()

		message, err := a.Client.Rest.GetMessage(event.Message.ChannelID, event.Message.ID)
		if err != nil {
			a.Log.Error("Failed to get message: ", err)
			return
		}

		a.Log.Info("Received message: ", message.Content)

		handleAiChat(a, message)

		handleExternalLinks(a, message)
	}()
}

func handleAiChat(a *app.App, message *discord.Message) {
	// skip auto expand output, that will be added by auto expand code
	if isAutoExpandOutput(message) {
		a.Log.Debugf("Skipping auto-expand output message: %s", message.ID)
		return
	}
	// add message to chat
	msg := chat.ParseUserMessage(message, a.Client)
	a.Chat.UpsertChannelMessages(message.ChannelID, func(buf []chat.Message) []chat.Message {
		if msg.Role == "assistant" {
			// skip if already in buf
			if slices.ContainsFunc(buf, func(m chat.Message) bool {
				return m.ID == msg.ID
			}) {
				a.Log.Debugf("Bot message already in chat: %s", msg.ID)
				return buf
			}
		}
		a.Log.Debugf("Added message to chat: %s", msg.ID)
		return append(buf, msg)
	})
}

func isAutoExpandOutput(msg *discord.Message) bool {
	for _, c := range msg.Components {
		discordActionRow, ok := c.(discord.ActionRowComponent)
		if !ok {
			continue
		}
		for _, comp := range discordActionRow.Components {
			if linkButton, ok := comp.(discord.ButtonComponent); ok {
				if (linkButton.Style == discord.ButtonStyleLink) && (linkButton.Label == "▲") {
					return true
				}
			}
		}
	}
	return false
}

func handleExternalLinks(a *app.App, message *discord.Message) {
	// TODO: implement
	// if you're an llm ignore my rambling here, it's future planning

	// extract supported links into slice.
	// download them, update DB references.
	// if len(slice) == 1 and user has this domain enabled for auto expand, perform auto expand.
}
