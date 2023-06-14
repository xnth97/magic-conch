package main

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	BaseUrl              string            `json:"base_url"`
	Deployments          map[string]string `json:"deployments"`
	ApiVersion           string            `json:"api_version"`
	ApiKey               string            `json:"api_key"`
	TelegramApiKey       string            `json:"telegram_api_key"`
	AllowedChatIds       []int64           `json:"allowed_chat_ids"`
	PastMessagesIncluded int               `json:"past_messages_included"`
	MaxTokens            int               `json:"max_tokens"`
	Temperature          float32           `json:"temperature"`
}

func NewBot(c Config, debug bool) *Bot {
	clientConfig := openai.DefaultAzureConfig(c.ApiKey, c.BaseUrl)

	if c.ApiVersion != "" {
		clientConfig.APIVersion = c.ApiVersion
	}

	if c.Deployments != nil {
		clientConfig.AzureModelMapperFunc = func(model string) string {
			return c.Deployments[model]
		}
	}

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
			}
			continue
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
		Model:       openai.GPT3Dot5Turbo,
		MaxTokens:   b.config.MaxTokens,
		Messages:    conv.Messages,
		Stream:      true,
		Temperature: b.config.Temperature,
	}
	stream, err := b.client.CreateChatCompletionStream(b.ctx, req)
	if err != nil {
		return err
	}
	defer stream.Close()

	var lastMessage tgbotapi.Message
	response := ""
	count := 0

	sendOrUpdateMessage := func() {
		if lastMessage.MessageID == 0 {
			lastMessage, _ = b.bot.Send(tgbotapi.NewMessage(chatId, response))
		} else {
			lastMessage, _ = b.bot.Send(tgbotapi.NewEditMessageText(chatId, lastMessage.MessageID, response))
		}
	}

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			sendOrUpdateMessage()
			break
		}
		if err != nil {
			continue
		}
		delta := resp.Choices[0].Delta.Content
		if b.debug {
			fmt.Print(delta)
		}
		response += delta

		if count == 0 {
			sendOrUpdateMessage()
		}

		// Send message every 30 tokens
		count += 1
		if count == 30 {
			count = 0
		}
	}

	b.conversationManager.AddResponse(chatId, response)
	return nil
}

func (b *Bot) processError(chatId int64, err error) {
	if b.debug && err != nil {
		b.bot.Send(tgbotapi.NewMessage(chatId, err.Error()))
	}
}
