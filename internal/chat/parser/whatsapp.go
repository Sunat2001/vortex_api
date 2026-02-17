package parser

import (
	"encoding/json"
	"fmt"
	"time"
)

// WhatsApp Cloud API webhook payload structures.

type waWebhookPayload struct {
	Entry []waEntry `json:"entry"`
}

type waEntry struct {
	ID      string     `json:"id"`
	Changes []waChange `json:"changes"`
}

type waChange struct {
	Value waValue `json:"value"`
	Field string  `json:"field"`
}

type waValue struct {
	MessagingProduct string      `json:"messaging_product"`
	Metadata         waMetadata  `json:"metadata"`
	Contacts         []waContact `json:"contacts"`
	Messages         []waMessage `json:"messages"`
}

type waMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type waContact struct {
	Profile waProfile `json:"profile"`
	WaID    string    `json:"wa_id"`
}

type waProfile struct {
	Name string `json:"name"`
}

type waMessage struct {
	From      string    `json:"from"`
	ID        string    `json:"id"`
	Timestamp string    `json:"timestamp"`
	Type      string    `json:"type"`
	Text      *waText   `json:"text,omitempty"`
	Image     *waMedia  `json:"image,omitempty"`
	Video     *waMedia  `json:"video,omitempty"`
	Audio     *waMedia  `json:"audio,omitempty"`
	Document  *waMedia  `json:"document,omitempty"`
}

type waText struct {
	Body string `json:"body"`
}

type waMedia struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	Caption  string `json:"caption"`
}

// WhatsAppParser parses WhatsApp Cloud API webhook payloads.
type WhatsAppParser struct{}

func (p *WhatsAppParser) Parse(raw json.RawMessage) ([]*IncomingMessage, error) {
	var payload waWebhookPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("invalid WhatsApp webhook payload: %w", err)
	}

	// Build a lookup of wa_id → contact name.
	contactNames := make(map[string]string)

	var messages []*IncomingMessage

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			for _, c := range change.Value.Contacts {
				contactNames[c.WaID] = c.Profile.Name
			}

			for _, m := range change.Value.Messages {
				msg := &IncomingMessage{
					ExternalSenderID:  m.From,
					ExternalMessageID: m.ID,
					ContactName:       contactNames[m.From],
					ContactPhone:      m.From,
					Timestamp:         parseUnixString(m.Timestamp),
				}

				switch m.Type {
				case "text":
					if m.Text != nil {
						msg.Content = m.Text.Body
					}
					msg.MediaType = MediaTypeText
				case "image":
					msg.MediaType = MediaTypeImage
					msg.Content = captionOrPlaceholder(m.Image, "[Image]")
					msg.Payload = mustMarshal(m.Image)
				case "video":
					msg.MediaType = MediaTypeVideo
					msg.Content = captionOrPlaceholder(m.Video, "[Video]")
					msg.Payload = mustMarshal(m.Video)
				case "audio":
					msg.MediaType = MediaTypeAudio
					msg.Content = "[Audio]"
					msg.Payload = mustMarshal(m.Audio)
				case "document":
					msg.MediaType = MediaTypeDocument
					msg.Content = captionOrPlaceholder(m.Document, "[Document]")
					msg.Payload = mustMarshal(m.Document)
				default:
					msg.MediaType = MediaType(m.Type)
					msg.Content = fmt.Sprintf("[%s]", m.Type)
				}

				messages = append(messages, msg)
			}
		}
	}

	return messages, nil
}

func captionOrPlaceholder(media *waMedia, placeholder string) string {
	if media != nil && media.Caption != "" {
		return media.Caption
	}
	return placeholder
}

func parseUnixString(s string) time.Time {
	var ts int64
	fmt.Sscanf(s, "%d", &ts)
	if ts == 0 {
		return time.Now()
	}
	return time.Unix(ts, 0)
}