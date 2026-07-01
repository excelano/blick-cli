package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
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
	ID             string
	Subject        string
	From           string
	Preview        string
	Received       time.Time
	To             []Recipient
	Cc             []Recipient
	HasAttachments bool
}

// inboxEmailTop caps how many messages the inbox history view pulls in one
// window. The caller surfaces a note when the cap is hit so the truncation
// isn't silent.
const inboxEmailTop = 50

// messageSelect is the shared $select for the message list views — the fields
// the dashboard and inbox rows render.
const messageSelect = "id,subject,from,toRecipients,ccRecipients,bodyPreview,receivedDateTime,hasAttachments"

func (g *GraphClient) UnreadEmails() ([]Email, error) {
	query := url.Values{
		"$filter":  {"isRead eq false"},
		"$orderby": {"receivedDateTime desc"},
		"$top":     {"10"},
		"$select":  {messageSelect},
	}

	data, err := g.get("/me/messages", query)
	if err != nil {
		return nil, err
	}
	return parseEmails(data)
}

// EmailsSince returns messages received at or after `since`, read included,
// newest first — the email half of the inbox history view. Capped at
// inboxEmailTop; the caller notes when the window overflows.
func (g *GraphClient) EmailsSince(since time.Time) ([]Email, error) {
	query := url.Values{
		"$filter":  {fmt.Sprintf("receivedDateTime ge %s", since.UTC().Format(time.RFC3339))},
		"$orderby": {"receivedDateTime desc"},
		"$top":     {strconv.Itoa(inboxEmailTop)},
		"$select":  {messageSelect},
	}

	data, err := g.get("/me/messages", query)
	if err != nil {
		return nil, err
	}
	return parseEmails(data)
}

func parseEmails(data []byte) ([]Email, error) {
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
			HasAttachments   bool             `json:"hasAttachments"`
		} `json:"value"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	emails := make([]Email, len(result.Value))
	for i, e := range result.Value {
		received, _ := time.Parse(time.RFC3339Nano, e.ReceivedDateTime)
		emails[i] = Email{
			ID:             e.ID,
			Subject:        e.Subject,
			From:           e.From.EmailAddress.Name,
			Preview:        e.BodyPreview,
			Received:       received,
			To:             toRecipients(e.ToRecipients),
			Cc:             toRecipients(e.CcRecipients),
			HasAttachments: e.HasAttachments,
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
func (g *GraphClient) SendMail(to []string, subject, body string, attachments []OutgoingAttachment) error {
	if len(to) == 0 {
		return fmt.Errorf("no recipients")
	}
	recipients := make([]map[string]interface{}, len(to))
	for i, addr := range to {
		recipients[i] = map[string]interface{}{
			"emailAddress": map[string]string{"address": addr},
		}
	}
	message := map[string]interface{}{
		"subject":      subject,
		"body":         map[string]string{"contentType": "Text", "content": body},
		"toRecipients": recipients,
	}
	if len(attachments) > 0 {
		encoded := make([]map[string]interface{}, len(attachments))
		for i, a := range attachments {
			encoded[i] = map[string]interface{}{
				"@odata.type":  fileAttachmentType,
				"name":         a.Name,
				"contentType":  a.ContentType,
				"contentBytes": base64.StdEncoding.EncodeToString(a.Content),
			}
		}
		message["attachments"] = encoded
	}
	payload := map[string]interface{}{
		"message":         message,
		"saveToSentItems": true,
	}
	_, err := g.post("/me/sendMail", payload)
	return err
}

var (
	htmlTagRe    = regexp.MustCompile(`<[^>]*>`)
	htmlEntityRe = regexp.MustCompile(`&[a-zA-Z0-9#]+;`)
	whitespaceRe = regexp.MustCompile(`\n{3,}`)
	// Matches an <a href="..."> ... </a> pair (case-insensitive, dot spans
	// newlines) capturing the destination and the visible inner content.
	anchorRe = regexp.MustCompile(`(?is)<a\b[^>]*?\bhref\s*=\s*["']([^"']*)["'][^>]*>(.*?)</a>`)
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

// safeLinkSuffix is the host suffix of Outlook ATP SafeLinks wrapper URLs,
// e.g. nam12.safelinks.protection.outlook.com.
const safeLinkSuffix = ".safelinks.protection.outlook.com"

// unwrapSafeLink returns the original destination when href is an Outlook ATP
// SafeLinks wrapper (https://<x>.safelinks.protection.outlook.com/?url=...),
// otherwise href unchanged. Guarded tightly: only that host and only when the
// url param is present are touched, so every other URL passes through as-is.
// Runs before entity decoding, when the href still carries &amp; separators;
// url.Values skips those amp;… segments (they hold a semicolon) and still
// recovers the clean, percent-decoded url param.
func unwrapSafeLink(href string) string {
	u, err := url.Parse(href)
	if err != nil {
		return href
	}
	host := strings.ToLower(u.Hostname())
	if host != "safelinks.protection.outlook.com" &&
		!strings.HasSuffix(host, safeLinkSuffix) {
		return href
	}
	if orig := u.Query().Get("url"); orig != "" {
		return orig
	}
	return href
}

// rewriteAnchors turns <a href="URL">text</a> into visible text that keeps the
// destination, so the general tag strip in stripHTML doesn't discard the URL.
// Terminals that auto-linkify make the surviving URL clickable again. Named
// anchors and javascript:/# hrefs carry no destination worth keeping, so only
// their visible text survives.
func rewriteAnchors(s string) string {
	return anchorRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := anchorRe.FindStringSubmatch(m)
		href := unwrapSafeLink(strings.TrimSpace(sub[1]))
		text := strings.TrimSpace(htmlTagRe.ReplaceAllString(sub[2], ""))
		if href == "" || strings.HasPrefix(href, "#") ||
			strings.HasPrefix(strings.ToLower(href), "javascript:") {
			return text
		}
		display := href
		if strings.HasPrefix(strings.ToLower(display), "mailto:") {
			display = display[len("mailto:"):]
		}
		if text == "" || text == display || text == href {
			return display
		}
		return text + " (" + display + ")"
	})
}

func stripHTML(s string) string {
	// Preserve link destinations before the tag strip removes the anchors.
	s = rewriteAnchors(s)
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
