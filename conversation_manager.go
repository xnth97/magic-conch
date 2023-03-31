package main

type ConversationManager struct {
	conversations map[int64]*Conversation
	roles         map[int64]string
}

const defaultRole = "You are a helpful assistant."

func NewConversationManager() *ConversationManager {
	return &ConversationManager{
		conversations: make(map[int64]*Conversation),
		roles:         make(map[int64]string),
	}
}

func (c *ConversationManager) GetConversation(id int64) *Conversation {
	if conversation, ok := c.conversations[id]; ok {
		return conversation
	}

	return c.Reset(id)
}

func (c *ConversationManager) Reset(id int64) *Conversation {
	conv := startConversation(c.getRole(id))
	c.conversations[id] = &conv
	return &conv
}

func (c *ConversationManager) ResetAll(id int64) *Conversation {
	delete(c.roles, id)
	return c.Reset(id)
}

func (c *ConversationManager) SetRole(id int64, systemMessage string) *Conversation {
	c.roles[id] = systemMessage
	return c.Reset(id)
}

func (c *ConversationManager) getRole(id int64) string {
	if role, ok := c.roles[id]; ok {
		return role
	}

	return defaultRole
}

func startConversation(systemMessage string) Conversation {
	return Conversation{Messages: []Message{
		{
			Role:    MessageRoleSystem,
			Content: systemMessage,
		},
	}}
}
