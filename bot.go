package main

import (
	"fmt"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	client         *Client
	bot            *tgbotapi.BotAPI
	allowedChatIds []int64
	debug          bool
}

type Config struct {
	BaseUrl              string  `json:"base_url"`
	Model                string  `json:"model"`
	ApiVersion           string  `json:"api_version"`
	ApiKey               string  `json:"api_key"`
	TelegramApiKey       string  `json:"telegram_api_key"`
	AllowedChatIds       []int64 `json:"allowed_chat_ids"`
	PastMessagesIncluded int     `json:"past_messages_included"`
	MaxTokens            int     `json:"max_tokens"`
}

func NewBot(c Config, debug bool) *Bot {
	clientConfig := DefaultConfig()
	if c.PastMessagesIncluded != 0 {
		clientConfig.PastMessagesIncluded = c.PastMessagesIncluded
	}
	if c.MaxTokens != 0 {
		clientConfig.MaxTokens = c.MaxTokens
	}

	client := NewClientWithConfig(
		c.BaseUrl,
		c.Model,
		c.ApiVersion,
		c.ApiKey,
		clientConfig,
	)

	bot, err := tgbotapi.NewBotAPI(c.TelegramApiKey)
	if err != nil {
		panic(err)
	}

	bot.Debug = debug

	return &Bot{
		client:         client,
		bot:            bot,
		allowedChatIds: c.AllowedChatIds,
		debug:          debug,
	}
}

func (b *Bot) Start() {
	conversationManager := NewConversationManager()

	allowedChatIds := mapset.NewSet[int64]()
	for _, id := range b.allowedChatIds {
		allowedChatIds.Add(id)
	}

	fmt.Println("Why don't you ask the magic conch?")

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	updates := b.bot.GetUpdatesChan(updateConfig)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		chatId := update.Message.Chat.ID
		if allowedChatIds.Cardinality() > 0 && !allowedChatIds.Contains(chatId) {
			msg := "Unauthorized Access"
			if b.debug {
				msg += fmt.Sprintf(" (chatId: %d)", chatId)
			}
			b.bot.Send(tgbotapi.NewMessage(chatId, msg))
			continue
		}

		text := update.Message.Text

		// Reset conversation
		if strings.HasPrefix(text, "/resetall") {
			conversationManager.ResetAll(chatId)
			b.bot.Send(tgbotapi.NewMessage(chatId, "Alright! Conversation and role are reset"))
			continue
		} else if strings.HasPrefix(text, "/reset") {
			conversationManager.Reset(chatId)
			b.bot.Send(tgbotapi.NewMessage(chatId, "Alright! Conversation is reset"))
			continue
		} else if strings.HasPrefix(text, "/role") {
			role := strings.Trim(strings.ReplaceAll(text, "/role", ""), " ")
			conversationManager.SetRole(chatId, role)
			b.bot.Send(tgbotapi.NewMessage(chatId, "Alright! System prompt updated to: "+role))
			continue
		}

		// Group: only responds to /chat
		if update.Message.Chat.IsGroup() {
			if strings.HasPrefix(text, "/chat") {
				conversation := conversationManager.GetConversation(chatId)
				query := strings.Trim(strings.ReplaceAll(text, "/chat", ""), " ")
				ans, err := b.client.Chat(conversation, query)
				if err != nil {
					b.bot.Send(tgbotapi.NewMessage(chatId, "Error: "+err.Error()))
					continue
				}
				b.bot.Send(tgbotapi.NewMessage(chatId, ans))
				continue
			}
		}

		// Normal messages
		conversation := conversationManager.GetConversation(chatId)
		ans, err := b.client.Chat(conversation, text)
		if err != nil {
			b.bot.Send(tgbotapi.NewMessage(chatId, "Error: "+err.Error()))
			continue
		}
		b.bot.Send(tgbotapi.NewMessage(chatId, ans))
	}
}
