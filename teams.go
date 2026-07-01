package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// chatPageSize caps how many chats /me/chats returns in one page. With the
// recency $orderby it's the N most-recently-active chats; the inbox view notes
// when the window may have been truncated by this cap.
const chatPageSize = 50

type ChatMessage struct {
	ChatID  string
	Topic   string
	From    string
	Preview string
	Sent    time.Time
}

// UnreadChats returns chats with an unread last message in the past 24h —
// the dashboard's chat panel.
//
// "Unread" uses Graph's per-user viewpoint.lastMessageReadDateTime: a chat is
// unread when the last message's createdDateTime is newer than the user's
// last-read mark. Replaces the older "last message wasn't from me" workaround
// from when Graph didn't expose per-user chat read state. The 24h cutoff
// stays — an unread message from last week shouldn't linger in the panel.
func (g *GraphClient) UnreadChats() ([]ChatMessage, error) {
	return g.chatsFrom(time.Now().Add(-24*time.Hour), true)
}

// ChatsSince returns chats whose most recent message falls at or after
// `since`, read included — the chat half of the inbox history view. Same
// source and filters as UnreadChats (hidden chats and non-message events
// dropped) minus the unread gate.
func (g *GraphClient) ChatsSince(since time.Time) ([]ChatMessage, error) {
	return g.chatsFrom(since, false)
}

// chatsFrom fetches /me/chats with the expanded last-message preview and
// returns one ChatMessage per surviving chat. cutoff drops chats whose last
// message predates it. unreadOnly additionally drops chats already read —
// last message not newer than viewpoint.lastMessageReadDateTime. Honors
// viewpoint.isHidden so chats the user hid in Teams stay hidden either way.
func (g *GraphClient) chatsFrom(cutoff time.Time, unreadOnly bool) ([]ChatMessage, error) {
	query := url.Values{
		"$select":  {"id,topic,lastMessagePreview,viewpoint"},
		"$expand":  {"lastMessagePreview"},
		"$orderby": {"lastMessagePreview/createdDateTime desc"},
		"$top":     {strconv.Itoa(chatPageSize)},
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

		// The real read-state check, dashboard-only. lastMessageReadDateTime
		// can be "0001-01-01T00:00:00Z" for chats the user has never opened;
		// the comparison still works because that's older than any real sent
		// time. We do NOT skip chats where the last message is from the
		// signed-in user — Teams advances lastMessageReadDateTime on send,
		// so the viewpoint check already handles that. The inbox view passes
		// unreadOnly=false to keep read chats too.
		if unreadOnly {
			var lastRead time.Time
			if chat.Viewpoint != nil {
				lastRead, _ = time.Parse(time.RFC3339Nano, chat.Viewpoint.LastMessageReadDateTime)
			}
			if !sent.After(lastRead) {
				continue
			}
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

// LookupUserID resolves a Graph user's object ID from their primary email
// or UPN. Required for composing a new 1:1 chat — POST /chats wants the
// recipient's object ID, not their address.
//
// Uses /me/people (covered by the People.Read scope we already consent
// to) rather than /users/{email} (which would need User.ReadBasic.All
// for any user other than self). Person.id is the AAD user object ID
// for in-tenant users — the same value EnsureOneOnOneChat needs for
// the @odata.bind URI. Works for anyone in the signed-in user's
// relevance graph; users with no prior interaction won't be found.
func (g *GraphClient) LookupUserID(email string) (string, error) {
	query := url.Values{
		"$search": {fmt.Sprintf("%q", email)},
		"$select": {"id,scoredEmailAddresses"},
		"$top":    {"10"},
	}
	data, err := g.get("/me/people", query)
	if err != nil {
		return "", err
	}
	var result struct {
		Value []struct {
			ID                   string `json:"id"`
			ScoredEmailAddresses []struct {
				Address string `json:"address"`
			} `json:"scoredEmailAddresses"`
		} `json:"value"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	for _, p := range result.Value {
		if p.ID == "" {
			continue
		}
		for _, a := range p.ScoredEmailAddresses {
			if strings.EqualFold(a.Address, email) {
				return p.ID, nil
			}
		}
	}
	return "", fmt.Errorf("no match for %s in your frequent contacts (/me/people) — email or chat them once via Outlook/Teams so they appear in the relevance graph, then try again", email)
}

// EnsureOneOnOneChat creates a 1:1 chat between the signed-in user and
// the recipient. Graph treats POST /chats as idempotent for oneOnOne —
// if a chat already exists between the two users, the existing chat ID
// comes back. Caller should cache the returned ID on the recipient's
// contact entry so subsequent sends skip the create round-trip.
func (g *GraphClient) EnsureOneOnOneChat(recipientUserID string) (string, error) {
	body := map[string]interface{}{
		"chatType": "oneOnOne",
		"members": []map[string]interface{}{
			{
				"@odata.type":     "#microsoft.graph.aadUserConversationMember",
				"roles":           []string{"owner"},
				"user@odata.bind": fmt.Sprintf("https://graph.microsoft.com/v1.0/users('%s')", g.userID),
			},
			{
				"@odata.type":     "#microsoft.graph.aadUserConversationMember",
				"roles":           []string{"owner"},
				"user@odata.bind": fmt.Sprintf("https://graph.microsoft.com/v1.0/users('%s')", recipientUserID),
			},
		},
	}
	data, err := g.post("/chats", body)
	if err != nil {
		return "", err
	}
	var c struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return "", err
	}
	if c.ID == "" {
		return "", fmt.Errorf("no chat ID returned by /chats")
	}
	return c.ID, nil
}

// CreateGroupChat creates a Teams group chat with the signed-in user plus
// the supplied recipients. topic is optional — when empty, Teams shows a
// comma-joined participant list as the chat title. Unlike /chats for
// oneOnOne (which is idempotent and returns the existing chat ID),
// chatType: group always creates a fresh chat — there's no natural
// participant key to dedupe against, since the same set of people can
// have any number of distinct group threads. So we don't cache the
// chat ID on any contact entry; each `chat alice bob` opens a new
// thread, matching how Teams itself behaves when you start a new chat
// from the recipient picker.
func (g *GraphClient) CreateGroupChat(recipientUserIDs []string, topic string) (string, error) {
	members := []map[string]interface{}{
		{
			"@odata.type":     "#microsoft.graph.aadUserConversationMember",
			"roles":           []string{"owner"},
			"user@odata.bind": fmt.Sprintf("https://graph.microsoft.com/v1.0/users('%s')", g.userID),
		},
	}
	for _, id := range recipientUserIDs {
		members = append(members, map[string]interface{}{
			"@odata.type":     "#microsoft.graph.aadUserConversationMember",
			"roles":           []string{"owner"},
			"user@odata.bind": fmt.Sprintf("https://graph.microsoft.com/v1.0/users('%s')", id),
		})
	}
	body := map[string]interface{}{
		"chatType": "group",
		"members":  members,
	}
	if topic != "" {
		body["topic"] = topic
	}
	data, err := g.post("/chats", body)
	if err != nil {
		return "", err
	}
	var c struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return "", err
	}
	if c.ID == "" {
		return "", fmt.Errorf("no chat ID returned by /chats")
	}
	return c.ID, nil
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
