package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Recipient struct {
	Name    string
	Address string
}

// Display returns the visible label for a recipient: prefer Name, fall
// back to Address when the name is missing or duplicates the address.
func (r Recipient) Display() string {
	if r.Name != "" && r.Name != r.Address {
		return r.Name
	}
	return r.Address
}

type Email struct {
	ID       string
	Subject  string
	From     string
	Preview  string
	Received time.Time
	To       []Recipient
	Cc       []Recipient
}

func (g *GraphClient) UnreadEmails() ([]Email, error) {
	query := url.Values{
		"$filter":  {"isRead eq false"},
		"$orderby": {"receivedDateTime desc"},
		"$top":     {"10"},
		"$select":  {"id,subject,from,toRecipients,ccRecipients,bodyPreview,receivedDateTime"},
	}

	data, err := g.get("/me/messages", query)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []struct {
			ID   string `json:"id"`
			From struct {
				EmailAddress struct {
					Name string `json:"name"`
				} `json:"emailAddress"`
			} `json:"from"`
			ToRecipients     []graphRecipient `json:"toRecipients"`
			CcRecipients     []graphRecipient `json:"ccRecipients"`
			Subject          string           `json:"subject"`
			BodyPreview      string           `json:"bodyPreview"`
			ReceivedDateTime string           `json:"receivedDateTime"`
		} `json:"value"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	emails := make([]Email, len(result.Value))
	for i, e := range result.Value {
		received, _ := time.Parse(time.RFC3339Nano, e.ReceivedDateTime)
		emails[i] = Email{
			ID:       e.ID,
			Subject:  e.Subject,
			From:     e.From.EmailAddress.Name,
			Preview:  e.BodyPreview,
			Received: received,
			To:       toRecipients(e.ToRecipients),
			Cc:       toRecipients(e.CcRecipients),
		}
	}

	return emails, nil
}

type graphRecipient struct {
	EmailAddress struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	} `json:"emailAddress"`
}

func toRecipients(in []graphRecipient) []Recipient {
	out := make([]Recipient, 0, len(in))
	for _, r := range in {
		out = append(out, Recipient{
			Name:    r.EmailAddress.Name,
			Address: r.EmailAddress.Address,
		})
	}
	return out
}

// withoutAddress returns recipients with any entry matching addr removed
// (case-insensitive on the SMTP address). Used to filter the signed-in
// user out of reply-all displays so the lists reflect who actually gets
// the reply.
func withoutAddress(rs []Recipient, addr string) []Recipient {
	if addr == "" {
		return rs
	}
	out := make([]Recipient, 0, len(rs))
	for _, r := range rs {
		if !strings.EqualFold(r.Address, addr) {
			out = append(out, r)
		}
	}
	return out
}

func (g *GraphClient) GetEmailBody(id string) (string, error) {
	query := url.Values{
		"$select": {"body"},
	}

	data, err := g.get(fmt.Sprintf("/me/messages/%s", id), query)
	if err != nil {
		return "", err
	}

	var result struct {
		Body struct {
			ContentType string `json:"contentType"`
			Content     string `json:"content"`
		} `json:"body"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}

	if result.Body.ContentType == "html" {
		return stripHTML(result.Body.Content), nil
	}
	return result.Body.Content, nil
}

func (g *GraphClient) MarkEmailRead(id string) error {
	return g.patch(fmt.Sprintf("/me/messages/%s", id), map[string]bool{"isRead": true})
}

// ReplyAllToEmail posts a reply-all to the message. Graph's /replyAll
// self-degrades to reply-to-sender when the original has no other
// recipients, so this is safe to use unconditionally — matches the iOS
// Blick app's "reply defaults to reply-all" behavior.
func (g *GraphClient) ReplyAllToEmail(id, comment string) error {
	html := strings.ReplaceAll(comment, "\n", "<br>")
	body := map[string]interface{}{
		"comment": html + "<br><br>",
	}
	_, err := g.post(fmt.Sprintf("/me/messages/%s/replyAll", id), body)
	return err
}

// SendMail composes and sends a new message in one shot via /me/sendMail
// (saveToSentItems defaults to true so the message lands in Sent like any
// other Outlook send). Content type is Text — the keyboard-first compose
// flow doesn't deal in HTML.
func (g *GraphClient) SendMail(to []string, subject, body string) error {
	if len(to) == 0 {
		return fmt.Errorf("no recipients")
	}
	recipients := make([]map[string]interface{}, len(to))
	for i, addr := range to {
		recipients[i] = map[string]interface{}{
			"emailAddress": map[string]string{"address": addr},
		}
	}
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"subject":      subject,
			"body":         map[string]string{"contentType": "Text", "content": body},
			"toRecipients": recipients,
		},
		"saveToSentItems": true,
	}
	_, err := g.post("/me/sendMail", payload)
	return err
}

var (
	htmlTagRe    = regexp.MustCompile(`<[^>]*>`)
	htmlEntityRe = regexp.MustCompile(`&[a-zA-Z0-9#]+;`)
	whitespaceRe = regexp.MustCompile(`\n{3,}`)
)

var entityMap = map[string]string{
	"&amp;":  "&",
	"&lt;":   "<",
	"&gt;":   ">",
	"&nbsp;": " ",
	"&#39;":  "'",
	"&quot;": "\"",
	"&apos;": "'",
}

func stripHTML(s string) string {
	// Replace block elements with newlines
	for _, tag := range []string{"</p>", "</div>", "</tr>", "<br>", "<br/>", "<br />"} {
		s = strings.ReplaceAll(s, tag, "\n")
	}
	s = htmlTagRe.ReplaceAllString(s, "")
	s = htmlEntityRe.ReplaceAllStringFunc(s, func(entity string) string {
		if r, ok := entityMap[strings.ToLower(entity)]; ok {
			return r
		}
		return entity
	})
	s = whitespaceRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
