package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

type ChatMessage struct {
	ChatID  string
	Topic   string
	From    string
	Preview string
	Sent    time.Time
}

func (g *GraphClient) PendingChats() ([]ChatMessage, error) {
	// Fetch recent chats and filter to those where someone else sent the last
	// message (i.e., waiting on your reply). This avoids relying on unreadCount
	// which the Graph API doesn't reliably return.
	query := url.Values{
		"$select": {"id,topic,chatType,lastMessagePreview"},
		"$expand": {"lastMessagePreview"},
		"$top":    {"50"},
	}

	data, err := g.get("/me/chats", query)
	if err != nil {
		return nil, err
	}

	if debug {
		fmt.Printf("[DEBUG] /me/chats response (%d bytes):\n%s\n\n", len(data), string(data))
	}

	var result struct {
		Value []struct {
			ID       string `json:"id"`
			Topic    string `json:"topic"`
			ChatType string `json:"chatType"`
			LastMessagePreview *struct {
				Body struct {
					Content string `json:"content"`
				} `json:"body"`
				From *struct {
					User *struct {
						ID          string `json:"id"`
						DisplayName string `json:"displayName"`
					} `json:"user"`
				} `json:"from"`
				CreatedDateTime string `json:"createdDateTime"`
				MessageType     string `json:"messageType"`
			} `json:"lastMessagePreview"`
		} `json:"value"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-24 * time.Hour)

	var messages []ChatMessage
	for _, chat := range result.Value {
		if chat.LastMessagePreview == nil {
			continue
		}

		// Skip system messages (meeting recordings, etc.)
		if chat.LastMessagePreview.MessageType != "" && chat.LastMessagePreview.MessageType != "message" {
			continue
		}

		// Skip if sender is unknown
		if chat.LastMessagePreview.From == nil || chat.LastMessagePreview.From.User == nil {
			continue
		}

		// Skip if you sent the last message — not pending
		if chat.LastMessagePreview.From.User.ID == g.userID {
			continue
		}

		sent, _ := time.Parse(time.RFC3339Nano, chat.LastMessagePreview.CreatedDateTime)

		// Only show messages from the last 24 hours
		if sent.Before(cutoff) {
			continue
		}

		msg := ChatMessage{
			ChatID:  chat.ID,
			From:    chat.LastMessagePreview.From.User.DisplayName,
			Preview: stripHTML(chat.LastMessagePreview.Body.Content),
			Sent:    sent,
		}

		if chat.Topic != "" {
			msg.Topic = chat.Topic
		} else if msg.From != "" {
			msg.Topic = msg.From
		} else {
			msg.Topic = "Chat"
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

func (g *GraphClient) GetChatMessages(chatID string, count int) ([]ChatMessage, error) {
	query := url.Values{
		"$top": {fmt.Sprintf("%d", count)},
	}

	data, err := g.get(fmt.Sprintf("/me/chats/%s/messages", chatID), query)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []struct {
			ID   string `json:"id"`
			Body struct {
				Content string `json:"content"`
			} `json:"body"`
			From *struct {
				User *struct {
					DisplayName string `json:"displayName"`
				} `json:"user"`
			} `json:"from"`
			CreatedDateTime string `json:"createdDateTime"`
			MessageType     string `json:"messageType"`
		} `json:"value"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	var messages []ChatMessage
	for _, m := range result.Value {
		if m.MessageType != "message" {
			continue
		}
		msg := ChatMessage{
			ChatID:  chatID,
			Preview: stripHTML(m.Body.Content),
		}
		if m.From != nil && m.From.User != nil {
			msg.From = m.From.User.DisplayName
		}
		msg.Sent, _ = time.Parse(time.RFC3339Nano, m.CreatedDateTime)
		messages = append(messages, msg)
	}

	return messages, nil
}

func (g *GraphClient) SendChatMessage(chatID, text string) error {
	body := map[string]interface{}{
		"body": map[string]string{
			"content": text,
		},
	}
	_, err := g.post(fmt.Sprintf("/me/chats/%s/messages", chatID), body)
	return err
}
