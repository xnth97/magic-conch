package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	mapset "github.com/deckarep/golang-set/v2"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	client              *azopenai.Client
	bot                 *tgbotapi.BotAPI
	allowedChatIds      []int64
	config              Config
	conversationManager *ConversationManager
	ctx                 context.Context
}

type Config struct {
	Debug                bool              `json:"is_debug"`
	BaseUrl              string            `json:"base_url"`
	DeploymentId         string            `json:"deployment_id"`
	Deployments          map[string]string `json:"deployments"`
	ApiVersion           string            `json:"api_version"`
	ApiKey               string            `json:"api_key"`
	TelegramApiKey       string            `json:"telegram_api_key"`
	AllowedChatIds       []int64           `json:"allowed_chat_ids"`
	PastMessagesIncluded int               `json:"past_messages_included"`
	MaxTokens            int32             `json:"max_tokens"`
	Temperature          float32           `json:"temperature"`
}

func NewBot(c Config) *Bot {
	keyCredential := azcore.NewKeyCredential(c.ApiKey)
	client, err := azopenai.NewClientWithKeyCredential(c.BaseUrl, keyCredential, nil)
	if err != nil {
		panic(err)
	}

	bot, err := tgbotapi.NewBotAPI(c.TelegramApiKey)
	if err != nil {
		panic(err)
	}

	bot.Debug = c.Debug

	return &Bot{
		client:              client,
		bot:                 bot,
		allowedChatIds:      c.AllowedChatIds,
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
			if b.config.Debug {
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
		} else if strings.HasPrefix(text, "/draw") {
			prompt := strings.Trim(strings.ReplaceAll(text, "/draw", ""), " ")
			b.DrawImage(chatId, prompt)
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

	req := azopenai.ChatCompletionsOptions{
		MaxTokens:      &b.config.MaxTokens,
		Messages:       conv.Messages,
		Temperature:    &b.config.Temperature,
		DeploymentName: &b.config.DeploymentId,
	}

	resp, err := b.client.GetChatCompletionsStream(b.ctx, req, nil)
	if err != nil {
		return err
	}

	stream := resp.ChatCompletionsStream
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
		resp, err := stream.Read()
		if errors.Is(err, io.EOF) {
			sendOrUpdateMessage()
			break
		}
		if err != nil {
			continue
		}
		if len(resp.Choices) == 0 {
			continue
		}

		delta := ""
		for _, choice := range resp.Choices {
			if choice.Delta.Content != nil {
				delta += *choice.Delta.Content
			}
		}

		if b.config.Debug {
			fmt.Print(delta)
		}
		response += delta

		if count == 0 && response != "" {
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

func (b *Bot) DrawImage(chatId int64, prompt string) {
	req := azopenai.ImageGenerationOptions{
		Prompt:         &prompt,
		Size:           to.Ptr(azopenai.ImageSizeSize1024X1024),
		ResponseFormat: to.Ptr(azopenai.ImageGenerationResponseFormatURL),
		N:              to.Ptr(int32(1)),
		DeploymentName: to.Ptr("dall-e-3"),
	}

	resp, err := b.client.GetImageGenerations(b.ctx, req, nil)
	if err != nil {
		b.processError(chatId, err)
	}

	for _, generatedImage := range resp.Data {
		b.bot.Send(tgbotapi.NewPhoto(chatId, tgbotapi.FileURL(*generatedImage.URL)))
	}
}

func (b *Bot) processError(chatId int64, err error) {
	if b.config.Debug && err != nil {
		b.bot.Send(tgbotapi.NewMessage(chatId, err.Error()))
	}
}
