package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Email struct {
	ID       string
	Subject  string
	From     string
	Preview  string
	Received time.Time
}

func (g *GraphClient) UnreadEmails() ([]Email, error) {
	query := url.Values{
		"$filter":  {"isRead eq false"},
		"$orderby": {"receivedDateTime desc"},
		"$top":     {"10"},
		"$select":  {"id,subject,from,bodyPreview,receivedDateTime"},
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
			Subject          string `json:"subject"`
			BodyPreview      string `json:"bodyPreview"`
			ReceivedDateTime string `json:"receivedDateTime"`
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
		}
	}

	return emails, nil
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

func (g *GraphClient) ReplyToEmail(id, comment string) error {
	body := map[string]interface{}{
		"comment": comment + "<br><br>",
	}
	_, err := g.post(fmt.Sprintf("/me/messages/%s/reply", id), body)
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
