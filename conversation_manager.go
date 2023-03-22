package main

type ConversationManager struct {
	conversations map[int64]*Conversation
}

func NewConversationManager() *ConversationManager {
	return &ConversationManager{
		conversations: make(map[int64]*Conversation),
	}
}

func (c *ConversationManager) GetConversation(id int64) *Conversation {
	if conversation, ok := c.conversations[id]; ok {
		return conversation
	}

	return c.Reset(id)
}

func (c *ConversationManager) Reset(id int64) *Conversation {
	conv := startConversation("You are a helpful assistant.")
	c.conversations[id] = &conv
	return &conv
}

func startConversation(systemMessage string) Conversation {
	return Conversation{Messages: []Message{
		{
			Role:    "system",
			Content: systemMessage,
		},
	}}
}
