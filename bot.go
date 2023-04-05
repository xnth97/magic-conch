package main

import (
	"context"
	"fmt"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sashabaranov/go-openai"
)

type Bot struct {
	client              *openai.Client
	bot                 *tgbotapi.BotAPI
	allowedChatIds      []int64
	debug               bool
	config              Config
	conversationManager *ConversationManager
	ctx                 context.Context
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
	clientConfig := openai.DefaultAzureConfig(c.ApiKey, c.BaseUrl, c.Model)
	clientConfig.APIVersion = c.ApiVersion
	client := openai.NewClientWithConfig(clientConfig)

	bot, err := tgbotapi.NewBotAPI(c.TelegramApiKey)
	if err != nil {
		panic(err)
	}

	bot.Debug = debug

	return &Bot{
		client:              client,
		bot:                 bot,
		allowedChatIds:      c.AllowedChatIds,
		debug:               debug,
		config:              c,
		conversationManager: NewConversationManager(c.PastMessagesIncluded),
		ctx:                 context.Background(),
	}
}

func (b *Bot) Start() {
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

		var err error

		// Reset conversation
		if strings.HasPrefix(text, "/resetall") {
			b.conversationManager.ResetAll(chatId)
			b.bot.Send(tgbotapi.NewMessage(chatId, "Alright! Conversation and role are reset"))
			continue
		} else if strings.HasPrefix(text, "/reset") {
			b.conversationManager.Reset(chatId)
			b.bot.Send(tgbotapi.NewMessage(chatId, "Alright! Conversation is reset"))
			continue
		} else if strings.HasPrefix(text, "/role") {
			role := strings.Trim(strings.ReplaceAll(text, "/role", ""), " ")
			b.conversationManager.SetSystemMessage(chatId, role)
			b.bot.Send(tgbotapi.NewMessage(chatId, "Alright! System prompt updated to: "+role))
			continue
		}

		// Group: only responds to /chat
		if update.Message.Chat.IsGroup() {
			if strings.HasPrefix(text, "/chat") {
				query := strings.Trim(strings.ReplaceAll(text, "/chat", ""), " ")
				err = b.Respond(chatId, query)
				b.processError(chatId, err)
				continue
			}
		}

		// Normal messages
		err = b.Respond(chatId, text)
		b.processError(chatId, err)
	}
}

func (b *Bot) Respond(chatId int64, query string) error {
	var err error

	conv := b.conversationManager.AddUserMessage(chatId, query)
	req := openai.ChatCompletionRequest{
		Model:     openai.GPT3Dot5Turbo,
		MaxTokens: b.config.MaxTokens,
		Messages:  conv.Messages,
		Stream:    false,
	}
	resp, err := b.client.CreateChatCompletion(b.ctx, req)
	if err != nil {
		return err
	}

	response := resp.Choices[0].Message.Content
	b.bot.Send(tgbotapi.NewMessage(chatId, response))

	b.conversationManager.AddResponse(chatId, response)
	return nil
}

func (b *Bot) processError(chatId int64, err error) {
	if b.debug && err != nil {
		b.bot.Send(tgbotapi.NewMessage(chatId, err.Error()))
	}
}
