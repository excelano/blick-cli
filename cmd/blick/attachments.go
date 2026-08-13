package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

// maxSendAttachmentBytes caps the total attachment payload for a single
// sendMail. Graph's simple (inline base64) attachment path tops out around
// 3 MB per message; past that the large-file upload-session flow is required,
// and that only works on saved drafts, not the one-shot sendMail this tool
// uses. Bigger files get a clear error instead of a confusing Graph 413.
const maxSendAttachmentBytes = 3 * 1024 * 1024

// OutgoingAttachment is a file staged for sending: its display name, MIME
// type, and raw bytes. Marshaled into the sendMail payload as a
// fileAttachment.
type OutgoingAttachment struct {
	Name        string
	ContentType string
	Content     []byte
}

// fileAttachmentType is the @odata.type Graph stamps on regular file
// attachments. Item attachments (embedded messages, calendar invites) and
// reference attachments (cloud links) carry different types and need
// different handling, so we key on this to tell them apart.
const fileAttachmentType = "#microsoft.graph.fileAttachment"

// Attachment is the metadata for one attachment on a message. Content is
// not fetched here — ListAttachments deliberately $selects it away so the
// list stays cheap; GetAttachmentContent pulls the bytes on demand.
type Attachment struct {
	ID          string
	Name        string
	ContentType string
	Size        int
	IsInline    bool
	ODataType   string
}

// IsFile reports whether this is a plain file attachment (as opposed to an
// embedded item or a cloud reference), i.e. something we can save or open.
func (a Attachment) IsFile() bool { return a.ODataType == fileAttachmentType }

// ListAttachments returns the attachments on a message. The $select omits
// contentBytes so a message with a large attachment doesn't drag its whole
// payload down just to list names. Inline attachments (signature logos and
// the like) are kept in the raw result; callers filter them out for the
// user-facing list.
func (g *GraphClient) ListAttachments(messageID string) ([]Attachment, error) {
	query := url.Values{
		"$select": {"id,name,contentType,size,isInline"},
	}
	data, err := g.get(fmt.Sprintf("/me/messages/%s/attachments", messageID), query)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []struct {
			ODataType   string `json:"@odata.type"`
			ID          string `json:"id"`
			Name        string `json:"name"`
			ContentType string `json:"contentType"`
			Size        int    `json:"size"`
			IsInline    bool   `json:"isInline"`
		} `json:"value"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	out := make([]Attachment, 0, len(result.Value))
	for _, v := range result.Value {
		out = append(out, Attachment{
			ID:          v.ID,
			Name:        v.Name,
			ContentType: v.ContentType,
			Size:        v.Size,
			IsInline:    v.IsInline,
			ODataType:   v.ODataType,
		})
	}
	return out, nil
}

// userAttachments returns the non-inline attachments, which is the set the
// user sees and indexes against in `attach N` / `save` / `open`. Inline
// attachments are embedded content (signature logos), not files the user
// asked to receive, so they're dropped from every user-facing view to keep
// the indexing consistent across the three verbs.
func userAttachments(all []Attachment) []Attachment {
	out := make([]Attachment, 0, len(all))
	for _, a := range all {
		if !a.IsInline {
			out = append(out, a)
		}
	}
	return out
}

// GetAttachmentContent fetches a single attachment including its bytes.
// Graph returns fileAttachment content base64-encoded in contentBytes; we
// decode it here so callers deal in raw bytes. Returns the attachment's
// declared name alongside the content.
func (g *GraphClient) GetAttachmentContent(messageID, attachmentID string) (name string, content []byte, err error) {
	data, err := g.get(fmt.Sprintf("/me/messages/%s/attachments/%s", messageID, attachmentID), nil)
	if err != nil {
		return "", nil, err
	}

	var result struct {
		Name         string `json:"name"`
		ContentBytes string `json:"contentBytes"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", nil, err
	}

	decoded, err := base64.StdEncoding.DecodeString(result.ContentBytes)
	if err != nil {
		return "", nil, fmt.Errorf("decoding attachment content: %w", err)
	}
	return result.Name, decoded, nil
}

// readOutgoingAttachments loads the files named on the compose command line
// so a bad path fails before any prompt opens — the same fail-fast contract
// as an unknown recipient. It enforces the total-size cap up front so the
// user learns a file is too big before typing a message that can't be sent.
func readOutgoingAttachments(paths []string) ([]OutgoingAttachment, error) {
	out := make([]OutgoingAttachment, 0, len(paths))
	total := 0
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("attachment %s: %w", p, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("attachment %s is a directory", p)
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("attachment %s: %w", p, err)
		}
		total += len(content)
		if total > maxSendAttachmentBytes {
			return nil, fmt.Errorf("attachments exceed the %s send limit — larger files need a shared link instead", humanSize(maxSendAttachmentBytes))
		}
		out = append(out, OutgoingAttachment{
			Name:        filepath.Base(p),
			ContentType: detectContentType(p, content),
			Content:     content,
		})
	}
	return out, nil
}

// detectContentType guesses a MIME type from the file extension, falling back
// to sniffing the leading bytes, then to a generic binary type. Graph accepts
// application/octet-stream fine, so this is best-effort labeling only.
func detectContentType(path string, content []byte) string {
	if ct := mime.TypeByExtension(filepath.Ext(path)); ct != "" {
		return ct
	}
	if len(content) > 0 {
		return http.DetectContentType(content)
	}
	return "application/octet-stream"
}
