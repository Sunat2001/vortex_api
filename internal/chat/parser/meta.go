package parser

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Meta webhook payload structures.
// Facebook uses entry[].messaging[], Instagram uses entry[].changes[].value.

type metaWebhookPayload struct {
	Object string      `json:"object"`
	Entry  []metaEntry `json:"entry"`
}

type metaEntry struct {
	ID        string               `json:"id"`
	Time      int64                `json:"time"`
	Messaging []metaMessagingEvent `json:"messaging"`
	Changes   []metaChange         `json:"changes"`
}

type metaChange struct {
	Field string             `json:"field"`
	Value metaMessagingEvent `json:"value"`
}

type metaMessagingEvent struct {
	Sender    metaParticipant `json:"sender"`
	Recipient metaParticipant `json:"recipient"`
	Timestamp json.Number     `json:"timestamp"`
	Message   *metaMessage    `json:"message,omitempty"`
}

type metaParticipant struct {
	ID string `json:"id"`
}

type metaMessage struct {
	Mid         string           `json:"mid"`
	Text        string           `json:"text"`
	Attachments []metaAttachment `json:"attachments"`
}

type metaAttachment struct {
	Type    string              `json:"type"`
	Payload metaAttachmentMedia `json:"payload"`
}

type metaAttachmentMedia struct {
	URL string `json:"url"`
}

// MetaParser parses Facebook and Instagram webhook payloads.
type MetaParser struct{}

func (p *MetaParser) Parse(raw json.RawMessage) ([]WebhookEvent, error) {
	var payload metaWebhookPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("invalid Meta webhook payload: %w", err)
	}

	var events []WebhookEvent

	for _, entry := range payload.Entry {
		// Facebook format: entry[].messaging[]
		for _, event := range entry.Messaging {
			if msg := p.parseEvent(event); msg != nil {
				events = append(events, WebhookEvent{Kind: EventKindMessage, Message: msg})
			}
		}

		// Instagram format: entry[].changes[].value
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}
			if msg := p.parseEvent(change.Value); msg != nil {
				events = append(events, WebhookEvent{Kind: EventKindMessage, Message: msg})
			}
		}
	}

	return events, nil
}

func (p *MetaParser) parseEvent(event metaMessagingEvent) *IncomingMessage {
	if event.Message == nil {
		return nil
	}

	msg := &IncomingMessage{
		ExternalSenderID:  event.Sender.ID,
		ExternalMessageID: event.Message.Mid,
		Timestamp:         parseTimestamp(event.Timestamp),
	}

	if event.Message.Text != "" {
		msg.Content = event.Message.Text
		msg.MediaType = MediaTypeText
	} else if len(event.Message.Attachments) > 0 {
		att := event.Message.Attachments[0]
		msg.MediaType = mapMetaAttachmentType(att.Type)
		msg.Content = fmt.Sprintf("[%s]", msg.MediaType)
		// Only store attachment data in payload (media URLs, types)
		msg.Payload = mustMarshal(event.Message.Attachments)
	}

	return msg
}

// parseTimestamp handles both string ("1527459824") and numeric (1527459824) timestamps.
func parseTimestamp(n json.Number) time.Time {
	ts, err := n.Int64()
	if err != nil {
		// Fallback: try parsing as string
		if v, err := strconv.ParseInt(n.String(), 10, 64); err == nil {
			ts = v
		}
	}
	if ts == 0 {
		return time.Now()
	}
	// Meta sometimes sends milliseconds, sometimes seconds
	if ts > 1e12 {
		return time.Unix(ts/1000, 0)
	}
	return time.Unix(ts, 0)
}

func mapMetaAttachmentType(t string) MediaType {
	switch t {
	case "image":
		return MediaTypeImage
	case "video":
		return MediaTypeVideo
	case "audio":
		return MediaTypeAudio
	case "file":
		return MediaTypeDocument
	default:
		return MediaType(t)
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
