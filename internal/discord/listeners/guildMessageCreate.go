package listeners

import (
	"slices"
	"sprout/internal/app"
	"sprout/internal/discord/chat"
	"sprout/internal/platform/database"

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

		guild, err := database.ViewGuild(a.DB, event.GuildID)
		if err != nil {
			a.Log.Errorf("Failed to view guild %s: %v", event.GuildID, err)
			return
		}

		channel, err := database.ViewChannel(a.DB, event.Message.ChannelID)
		if err != nil {
			a.Log.Errorf("Failed to view channel %s: %v", event.Message.ChannelID, err)
			return
		}

		var user *database.User
		isSelf := event.Message.Author.ID == a.Client.ApplicationID
		if !isSelf {
			user, err = database.ViewUser(a.DB, event.Message.Author.ID)
			if err != nil {
				a.Log.Errorf("Failed to view user %s: %v", event.Message.Author.ID, err)
				return
			}
		}

		handleAiChat(a, guild, channel, isSelf, user, message)

		// TODO: Listener does per link, msg revive cmd new msg response to og with link to downloaded asset. If solo link attempt expand.
		// if you're an llm ignore my rambling here, it's future planning

		/* old code
		if err := expand.Expand(ctx, message, false); err != nil {
			xlog.Error(ctx, "Failed to expand message: ", err)
		}
		*/
	}()
}

func handleAiChat(
	a *app.App,
	guild *database.Guild,
	channel *database.Channel,
	isSelf bool,
	user *database.User,
	message *discord.Message,
) {
	if !guild.AiChatEnabled {
		a.Log.Debugf("Skipping message %s: AI chat disabled for guild %s", message.ID, *message.GuildID)
		return
	}
	if !channel.AiChat {
		a.Log.Debugf("Skipping message %s: AI chat disabled for channel %s", message.ID, message.ChannelID)
		return
	}
	// skip if ai chat is disabled via user settings
	if !isSelf {
		if !user.AiAccess || user.AiChatOptOut {
			a.Log.Debugf("Skipping message %s: User AiAccess=%v, AiChatOptOut=%v", message.ID, user.AiAccess, user.AiChatOptOut)
			return
		}
	}
	// skip auto expand output, that will be added by auto expand code
	if isAutoExpandOutput(message) {
		a.Log.Debugf("Skipping auto-expand output message: %s", message.ID)
		return
	}
	// add message to chat
	msg := chat.ParseUserMessage(message, a.Client)
	a.Chat.UpsertChannelMessages(message.ChannelID, func(buf []chat.Message) []chat.Message {
		if isSelf {
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
