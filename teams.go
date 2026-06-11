package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type ChatMessage struct {
	ChatID  string
	Topic   string
	From    string
	Preview string
	Sent    time.Time
}

func (g *GraphClient) UnreadChats() ([]ChatMessage, error) {
	// "Unread" uses Graph's per-user viewpoint.lastMessageReadDateTime: a
	// chat is unread when the last message's createdDateTime is newer than
	// the user's last-read mark. Replaces the older "last message wasn't
	// from me" workaround from when Graph didn't expose per-user chat read
	// state. Also honors viewpoint.isHidden so chats the user hid in Teams
	// stay hidden here. The 24h cutoff stays — an unread message from last
	// week shouldn't linger in the panel.
	query := url.Values{
		"$select": {"id,topic,lastMessagePreview,viewpoint"},
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
			ID                 string `json:"id"`
			Topic              string `json:"topic"`
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
			Viewpoint *struct {
				IsHidden                 bool   `json:"isHidden"`
				LastMessageReadDateTime  string `json:"lastMessageReadDateTime"`
			} `json:"viewpoint"`
		} `json:"value"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-24 * time.Hour)

	var messages []ChatMessage
	for _, chat := range result.Value {
		if chat.Viewpoint != nil && chat.Viewpoint.IsHidden {
			continue
		}
		if chat.LastMessagePreview == nil {
			continue
		}

		// Keep regular messages (and the rare empty messageType) and drop
		// the rest — joins, leaves, renames, meeting recordings, etc.
		if chat.LastMessagePreview.MessageType != "" && chat.LastMessagePreview.MessageType != "message" {
			continue
		}

		if chat.LastMessagePreview.From == nil || chat.LastMessagePreview.From.User == nil {
			continue
		}

		sent, _ := time.Parse(time.RFC3339Nano, chat.LastMessagePreview.CreatedDateTime)
		if sent.Before(cutoff) {
			continue
		}

		// The real read-state check. lastMessageReadDateTime can be
		// "0001-01-01T00:00:00Z" for chats the user has never opened; the
		// comparison still works because that's older than any real sent
		// time. We do NOT skip chats where the last message is from the
		// signed-in user — Teams advances lastMessageReadDateTime on send,
		// so the viewpoint check already handles that.
		var lastRead time.Time
		if chat.Viewpoint != nil {
			lastRead, _ = time.Parse(time.RFC3339Nano, chat.Viewpoint.LastMessageReadDateTime)
		}
		if !sent.After(lastRead) {
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

// MarkChatRead advances viewpoint.lastMessageReadDateTime for the signed-in
// user, which is what UnreadChats keys off — so the chat drops out of the
// unread list immediately. Mirrors the iOS app's markChatRead. Requires
// Chat.ReadWrite.
func (g *GraphClient) MarkChatRead(chatID string) error {
	body := map[string]interface{}{
		"user": map[string]string{
			"id":       g.userID,
			"tenantId": g.tenantID,
		},
	}
	_, err := g.post(fmt.Sprintf("/chats/%s/markChatReadForUser", chatID), body)
	return err
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
	html := strings.ReplaceAll(text, "\n", "<br>")
	body := map[string]interface{}{
		"body": map[string]string{
			"contentType": "html",
			"content":     html,
		},
	}
	_, err := g.post(fmt.Sprintf("/me/chats/%s/messages", chatID), body)
	return err
}
