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
	MessagingProduct string       `json:"messaging_product"`
	Metadata         waMetadata   `json:"metadata"`
	Contacts         []waContact  `json:"contacts"`
	Messages         []waMessage  `json:"messages"`
	Statuses         []waStatus   `json:"statuses"`
}

type waStatus struct {
	ID        string `json:"id"`
	Status    string `json:"status"` // "sent", "delivered", "read", "failed"
	Timestamp string `json:"timestamp"`
}

type waMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type waContact struct {
	Profile     waProfile `json:"profile"`
	WaID        string    `json:"wa_id"`
	CountryCode string    `json:"country_code,omitempty"`
}

type waProfile struct {
	Name string `json:"name"`
}

type waMessage struct {
	From          string          `json:"from"`
	ID            string          `json:"id"`
	Timestamp     string          `json:"timestamp"`
	FromLogicalID string          `json:"from_logical_id,omitempty"`
	Type          string          `json:"type"`
	Text          *waText         `json:"text,omitempty"`
	Image         *waMedia        `json:"image,omitempty"`
	Video         *waMedia        `json:"video,omitempty"`
	Audio         *waMedia        `json:"audio,omitempty"`
	Document      *waMedia        `json:"document,omitempty"`
	Location      *waLocation     `json:"location,omitempty"`
	Unsupported   *waUnsupported  `json:"unsupported,omitempty"`
	Errors        []waError       `json:"errors,omitempty"`
}

type waLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
}

type waUnsupported struct {
	Type string `json:"type"`
}

type waError struct {
	Code      int            `json:"code"`
	Title     string         `json:"title"`
	Message   string         `json:"message"`
	ErrorData *waErrorData   `json:"error_data,omitempty"`
}

type waErrorData struct {
	Details string `json:"details"`
}

type waText struct {
	Body string `json:"body"`
}

type waMedia struct {
	ID        string `json:"id"`
	MimeType  string `json:"mime_type"`
	SHA256    string `json:"sha256,omitempty"`
	Caption   string `json:"caption,omitempty"`
	URL       string `json:"url,omitempty"`
	StaticURL string `json:"static_url,omitempty"`
	Voice     bool   `json:"voice,omitempty"`
	Filename  string `json:"filename,omitempty"`
}

// WhatsAppParser parses WhatsApp Cloud API webhook payloads.
type WhatsAppParser struct{}

func (p *WhatsAppParser) Parse(raw json.RawMessage) ([]WebhookEvent, error) {
	var payload waWebhookPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("invalid WhatsApp webhook payload: %w", err)
	}

	var events []WebhookEvent

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			switch change.Field {
			case "messages":
				events = append(events, p.parseMessages(change.Value)...)
			case "statuses":
				// Statuses may also arrive under field "messages" in the same value block,
				// but some webhook configurations send them under field "statuses".
				events = append(events, p.parseStatuses(change.Value.Statuses)...)
			default:
				// Unknown field — skip silently.
			}

			// WhatsApp often sends statuses alongside messages under field "messages".
			if change.Field == "messages" && len(change.Value.Statuses) > 0 {
				events = append(events, p.parseStatuses(change.Value.Statuses)...)
			}
		}
	}

	return events, nil
}

func (p *WhatsAppParser) parseMessages(value waValue) []WebhookEvent {
	contactNames := make(map[string]string)
	contactCountryCodes := make(map[string]string)
	for _, c := range value.Contacts {
		contactNames[c.WaID] = c.Profile.Name
		if c.CountryCode != "" {
			contactCountryCodes[c.WaID] = c.CountryCode
		}
	}

	var events []WebhookEvent
	for _, m := range value.Messages {
		msg := &IncomingMessage{
			ExternalSenderID:  m.From,
			ExternalMessageID: m.ID,
			ContactName:       contactNames[m.From],
			ContactPhone:      m.From,
			Timestamp:         parseUnixString(m.Timestamp),
		}

		switch m.Type {
		case "text":
			msg.MediaType = MediaTypeText
			msg.Content = "[Text]"
			msg.Payload = mustMarshal(m.Text)
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
		case "location":
			msg.MediaType = MediaTypeLocation
			msg.Content = "[Location]"
			msg.Payload = mustMarshal(m.Location)
		case "unsupported":
			msg.MediaType = MediaTypeUnsupported
			subType := "unknown"
			if m.Unsupported != nil && m.Unsupported.Type != "" {
				subType = m.Unsupported.Type
			}
			msg.Content = fmt.Sprintf("[Unsupported: %s]", subType)
			if m.Unsupported != nil {
				msg.Payload = mustMarshal(m.Unsupported)
			}
		default:
			msg.MediaType = MediaType(m.Type)
			msg.Content = fmt.Sprintf("[%s]", m.Type)
		}

		// Build metadata from platform-specific diagnostic fields.
		msg.Metadata = p.buildMessageMetadata(m, contactCountryCodes[m.From])

		events = append(events, WebhookEvent{Kind: EventKindMessage, Message: msg})
	}
	return events
}

// buildMessageMetadata constructs metadata JSON from diagnostic fields.
func (p *WhatsAppParser) buildMessageMetadata(m waMessage, countryCode string) json.RawMessage {
	meta := make(map[string]interface{})

	if m.FromLogicalID != "" {
		meta["from_logical_id"] = m.FromLogicalID
	}
	if countryCode != "" {
		meta["country_code"] = countryCode
	}
	if len(m.Errors) > 0 {
		meta["errors"] = m.Errors
	}

	if len(meta) == 0 {
		return nil
	}
	return mustMarshal(meta)
}

func (p *WhatsAppParser) parseStatuses(statuses []waStatus) []WebhookEvent {
	var events []WebhookEvent
	for _, s := range statuses {
		events = append(events, WebhookEvent{
			Kind: EventKindStatus,
			Status: &StatusUpdate{
				ExternalMessageID: s.ID,
				Status:            s.Status,
				Timestamp:         parseUnixString(s.Timestamp),
			},
		})
	}
	return events
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