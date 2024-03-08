package main

import (
	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

type Conversation struct {
	Messages      []azopenai.ChatRequestMessageClassification
	SystemMessage string
}

type ConversationManager struct {
	conversations        map[int64]*Conversation
	pastMessagesIncluded int
}

const defaultSystemMessage = "You are a helpful assistant."

func NewConversationManager(pastMessagesIncluded int) *ConversationManager {
	return &ConversationManager{
		conversations:        make(map[int64]*Conversation),
		pastMessagesIncluded: pastMessagesIncluded,
	}
}

func (c *ConversationManager) GetConversation(id int64) *Conversation {
	if conversation, ok := c.conversations[id]; ok {
		return conversation
	}

	return c.ResetAll(id)
}

func (c *ConversationManager) Reset(id int64) *Conversation {
	conv := startConversation(c.getSystemMessage(id))
	c.conversations[id] = &conv
	return &conv
}

func (c *ConversationManager) ResetAll(id int64) *Conversation {
	delete(c.conversations, id)
	return c.Reset(id)
}

func (c *ConversationManager) SetSystemMessage(id int64, systemMessage string) *Conversation {
	c.conversations[id].SystemMessage = systemMessage
	return c.Reset(id)
}

func (c *ConversationManager) getSystemMessage(id int64) string {
	if conv, ok := c.conversations[id]; ok {
		return conv.SystemMessage
	}

	return defaultSystemMessage
}

func startConversation(systemMessage string) Conversation {
	return Conversation{
		Messages: []azopenai.ChatRequestMessageClassification{
			&azopenai.ChatRequestSystemMessage{
				Content: to.Ptr(systemMessage),
			},
		},
		SystemMessage: systemMessage,
	}
}

func (c *ConversationManager) AddUserMessage(id int64, userInput string) *Conversation {
	conv := c.GetConversation(id)
	conv.Messages = append(
		conv.Messages,
		&azopenai.ChatRequestUserMessage{
			Content: azopenai.NewChatRequestUserMessageContent(userInput),
		},
	)
	return conv
}

func (c *ConversationManager) AddResponse(id int64, response string) {
	conv := c.GetConversation(id)
	conv.Messages = append(
		conv.Messages,
		&azopenai.ChatRequestAssistantMessage{
			Content: to.Ptr(response),
		},
	)

	if len(conv.Messages) > c.pastMessagesIncluded && len(conv.Messages) > 3 {
		// keep the system message, remove 2nd (user message) and 3rd (assistant response)
		conv.Messages = append([]azopenai.ChatRequestMessageClassification{conv.Messages[0]}, conv.Messages[3:]...)
	}
}
