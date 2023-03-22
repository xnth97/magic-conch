package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	apiKey string
	url    string
	client *http.Client
	config ClientConfig
}

type Conversation struct {
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Messages         []Message `json:"messages"`
	MaxTokens        int       `json:"max_tokens"`
	Temperature      float32   `json:"temperature"`
	TopP             float32   `json:"top_p"`
	FrequencyPenalty float32   `json:"frequency_penalty"`
	PresencePenalty  float32   `json:"presence_penalty"`
}

type ChatCompletion struct {
	Choices []struct {
		Message Message `json:"message"`
	}
}

type ClientConfig struct {
	PastMessagesIncluded int
	MaxTokens            int
	Temperature          float32
	TopP                 float32
	FrequencyPenalty     float32
	PresencePenalty      float32
}

func NewClient(baseUrl string, model string, apiVersion string, apiKey string) *Client {
	fullUrl := baseUrl + "/openai/deployments/" + model + "/chat/completions?api-version=" + apiVersion
	return &Client{
		apiKey: apiKey,
		url:    fullUrl,
		client: &http.Client{},
		config: DefaultConfig(),
	}
}

func NewClientWithConfig(baseUrl string, model string, apiVersion string, apiKey string, config ClientConfig) *Client {
	return NewClient(baseUrl, model, apiVersion, apiKey)
}

func (c *Client) Chat(conversation *Conversation, prompt string) (string, error) {
	conversation.Messages = append(conversation.Messages, Message{
		Role:    "user",
		Content: prompt,
	})

	jsonData, err := json.Marshal(c.createRequest(conversation))
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("api-key", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error: %d", resp.StatusCode)
	}

	var chatCompletion ChatCompletion
	err = json.NewDecoder(resp.Body).Decode(&chatCompletion)

	if err != nil {
		return "", err
	}

	ans := chatCompletion.Choices[0].Message.Content
	conversation.Messages = append(conversation.Messages, Message{
		Role:    "assistant",
		Content: ans,
	})

	if len(conversation.Messages) > c.config.PastMessagesIncluded && len(conversation.Messages) > 4 {
		// keep the system message, remove 2nd (user message) and 3rd (assistant response)
		conversation.Messages = append([]Message{conversation.Messages[0]}, conversation.Messages[3:]...)
	}

	return ans, nil
}

func (c *Client) createRequest(conv *Conversation) ChatRequest {
	return ChatRequest{
		Messages:         conv.Messages,
		MaxTokens:        c.config.MaxTokens,
		Temperature:      c.config.Temperature,
		TopP:             c.config.TopP,
		FrequencyPenalty: c.config.FrequencyPenalty,
		PresencePenalty:  c.config.PresencePenalty,
	}
}

func DefaultConfig() ClientConfig {
	return ClientConfig{
		PastMessagesIncluded: 10,
		MaxTokens:            800,
		Temperature:          0.7,
		TopP:                 0.95,
		FrequencyPenalty:     0,
		PresencePenalty:      0,
	}
}
