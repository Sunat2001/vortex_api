package parser

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper builds a minimal WhatsApp webhook JSON payload with a single entry,
// single change under field "messages", and the supplied value block.
func buildWAPayload(field string, value waValue) json.RawMessage {
	payload := waWebhookPayload{
		Entry: []waEntry{
			{
				ID: "entry-1",
				Changes: []waChange{
					{
						Field: field,
						Value: value,
					},
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// mustUnmarshalMap unmarshals a json.RawMessage into map[string]interface{}.
// Fails the test immediately on error.
func mustUnmarshalMap(t *testing.T, raw json.RawMessage) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &m))
	return m
}

// ---- WhatsAppParser.Parse tests -----------------------------------------------

func TestWhatsAppParser_Parse_InvalidJSON(t *testing.T) {
	p := &WhatsAppParser{}
	_, err := p.Parse(json.RawMessage(`{not valid json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid WhatsApp webhook payload")
}

func TestWhatsAppParser_Parse_EmptyPayload(t *testing.T) {
	p := &WhatsAppParser{}
	events, err := p.Parse(json.RawMessage(`{"entry":[]}`))
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestWhatsAppParser_Parse_UnknownField_SkippedSilently(t *testing.T) {
	// A change with an unknown field should produce zero events.
	value := waValue{}
	raw := buildWAPayload("unknown_field", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	assert.Empty(t, events)
}

// ---- Text message -------------------------------------------------------------

func TestWhatsAppParser_Parse_TextMessage(t *testing.T) {
	value := waValue{
		Contacts: []waContact{
			{Profile: waProfile{Name: "Alice"}, WaID: "12345"},
		},
		Messages: []waMessage{
			{
				From:      "12345",
				ID:        "msg-001",
				Timestamp: "1700000000",
				Type:      "text",
				Text:      &waText{Body: "Hello, world!"},
			},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, EventKindMessage, event.Kind)
	require.NotNil(t, event.Message)

	msg := event.Message
	assert.Equal(t, "12345", msg.ExternalSenderID)
	assert.Equal(t, "msg-001", msg.ExternalMessageID)
	assert.Equal(t, "Alice", msg.ContactName)
	assert.Equal(t, "12345", msg.ContactPhone)
	assert.Equal(t, MediaTypeText, msg.MediaType)
	assert.Equal(t, "[Text]", msg.Content)

	// Payload must contain {"body":"Hello, world!"}
	payload := mustUnmarshalMap(t, msg.Payload)
	assert.Equal(t, "Hello, world!", payload["body"])

	// Timestamp must be parsed from unix string.
	assert.Equal(t, time.Unix(1700000000, 0), msg.Timestamp)
}

// ---- Image message ------------------------------------------------------------

func TestWhatsAppParser_Parse_ImageMessage_NoCaption(t *testing.T) {
	value := waValue{
		Messages: []waMessage{
			{
				From:      "99999",
				ID:        "img-001",
				Timestamp: "1700000001",
				Type:      "image",
				Image: &waMedia{
					ID:       "media-id-1",
					MimeType: "image/jpeg",
					SHA256:   "abc123",
				},
			},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 1)

	msg := events[0].Message
	assert.Equal(t, MediaTypeImage, msg.MediaType)
	// No caption — falls back to placeholder.
	assert.Equal(t, "[Image]", msg.Content)

	payload := mustUnmarshalMap(t, msg.Payload)
	assert.Equal(t, "media-id-1", payload["id"])
	assert.Equal(t, "image/jpeg", payload["mime_type"])
}

func TestWhatsAppParser_Parse_ImageMessage_WithCaption(t *testing.T) {
	value := waValue{
		Messages: []waMessage{
			{
				From:      "99999",
				ID:        "img-002",
				Timestamp: "1700000002",
				Type:      "image",
				Image: &waMedia{
					ID:       "media-id-2",
					MimeType: "image/png",
					Caption:  "Look at this!",
				},
			},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 1)

	msg := events[0].Message
	assert.Equal(t, MediaTypeImage, msg.MediaType)
	// Caption present — content is the caption.
	assert.Equal(t, "Look at this!", msg.Content)
}

// ---- Video message ------------------------------------------------------------

func TestWhatsAppParser_Parse_VideoMessage(t *testing.T) {
	value := waValue{
		Messages: []waMessage{
			{
				From:      "77777",
				ID:        "vid-001",
				Timestamp: "1700000003",
				Type:      "video",
				Video: &waMedia{
					ID:       "vid-media-1",
					MimeType: "video/mp4",
				},
			},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 1)

	msg := events[0].Message
	assert.Equal(t, MediaTypeVideo, msg.MediaType)
	assert.Equal(t, "[Video]", msg.Content)
	payload := mustUnmarshalMap(t, msg.Payload)
	assert.Equal(t, "vid-media-1", payload["id"])
}

// ---- Audio message ------------------------------------------------------------

func TestWhatsAppParser_Parse_AudioMessage(t *testing.T) {
	value := waValue{
		Messages: []waMessage{
			{
				From:      "66666",
				ID:        "aud-001",
				Timestamp: "1700000004",
				Type:      "audio",
				Audio: &waMedia{
					ID:       "aud-media-1",
					MimeType: "audio/ogg",
					Voice:    true,
				},
			},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 1)

	msg := events[0].Message
	assert.Equal(t, MediaTypeAudio, msg.MediaType)
	// Audio always uses placeholder, regardless of caption.
	assert.Equal(t, "[Audio]", msg.Content)
	payload := mustUnmarshalMap(t, msg.Payload)
	assert.Equal(t, "aud-media-1", payload["id"])
}

// ---- Document message ---------------------------------------------------------

func TestWhatsAppParser_Parse_DocumentMessage(t *testing.T) {
	value := waValue{
		Messages: []waMessage{
			{
				From:      "55555",
				ID:        "doc-001",
				Timestamp: "1700000005",
				Type:      "document",
				Document: &waMedia{
					ID:       "doc-media-1",
					MimeType: "application/pdf",
					Filename: "invoice.pdf",
					Caption:  "Invoice Q4",
				},
			},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 1)

	msg := events[0].Message
	assert.Equal(t, MediaTypeDocument, msg.MediaType)
	// Caption present on document — used as content.
	assert.Equal(t, "Invoice Q4", msg.Content)
	payload := mustUnmarshalMap(t, msg.Payload)
	assert.Equal(t, "invoice.pdf", payload["filename"])
}

func TestWhatsAppParser_Parse_DocumentMessage_NoCaption(t *testing.T) {
	value := waValue{
		Messages: []waMessage{
			{
				From:      "55555",
				ID:        "doc-002",
				Timestamp: "1700000006",
				Type:      "document",
				Document: &waMedia{
					ID:       "doc-media-2",
					MimeType: "application/pdf",
					Filename: "report.pdf",
				},
			},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 1)

	msg := events[0].Message
	assert.Equal(t, "[Document]", msg.Content)
}

// ---- Location message ---------------------------------------------------------

func TestWhatsAppParser_Parse_LocationMessage(t *testing.T) {
	value := waValue{
		Messages: []waMessage{
			{
				From:      "44444",
				ID:        "loc-001",
				Timestamp: "1700000007",
				Type:      "location",
				Location: &waLocation{
					Latitude:  40.7128,
					Longitude: -74.0060,
					Name:      "New York City",
					Address:   "New York, NY, USA",
				},
			},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 1)

	msg := events[0].Message
	assert.Equal(t, MediaTypeLocation, msg.MediaType)
	assert.Equal(t, "[Location]", msg.Content)

	payload := mustUnmarshalMap(t, msg.Payload)
	assert.InDelta(t, 40.7128, payload["latitude"], 0.0001)
	assert.InDelta(t, -74.0060, payload["longitude"], 0.0001)
	assert.Equal(t, "New York City", payload["name"])
	assert.Equal(t, "New York, NY, USA", payload["address"])
}

// ---- Unsupported message — with Unsupported field populated -------------------

func TestWhatsAppParser_Parse_UnsupportedMessage_WithType(t *testing.T) {
	value := waValue{
		Messages: []waMessage{
			{
				From:        "33333",
				ID:          "unsup-001",
				Timestamp:   "1700000008",
				Type:        "unsupported",
				Unsupported: &waUnsupported{Type: "video_note"},
			},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 1)

	msg := events[0].Message
	assert.Equal(t, MediaTypeUnsupported, msg.MediaType)
	assert.Equal(t, "[Unsupported: video_note]", msg.Content)

	// Payload must contain the type field.
	require.NotNil(t, msg.Payload)
	payload := mustUnmarshalMap(t, msg.Payload)
	assert.Equal(t, "video_note", payload["type"])
}

// ---- Unsupported message — with nil Unsupported field ------------------------

func TestWhatsAppParser_Parse_UnsupportedMessage_NilUnsupported(t *testing.T) {
	// When Unsupported field is nil, subType falls back to "unknown" and
	// Payload must be nil (not the JSON string "null").
	value := waValue{
		Messages: []waMessage{
			{
				From:        "33333",
				ID:          "unsup-002",
				Timestamp:   "1700000009",
				Type:        "unsupported",
				Unsupported: nil,
			},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 1)

	msg := events[0].Message
	assert.Equal(t, MediaTypeUnsupported, msg.MediaType)
	assert.Equal(t, "[Unsupported: unknown]", msg.Content)

	// Payload must be nil (not json.RawMessage("null")).
	assert.Nil(t, msg.Payload)
}

// ---- Unknown message type (default case) ------------------------------------

func TestWhatsAppParser_Parse_UnknownMessageType(t *testing.T) {
	value := waValue{
		Messages: []waMessage{
			{
				From:      "22222",
				ID:        "unk-001",
				Timestamp: "1700000010",
				Type:      "sticker",
			},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 1)

	msg := events[0].Message
	// Default case: MediaType is set to the raw type string.
	assert.Equal(t, MediaType("sticker"), msg.MediaType)
	assert.Equal(t, "[sticker]", msg.Content)
	// No payload set by the default case.
	assert.Nil(t, msg.Payload)
}

// ---- Message with errors array -----------------------------------------------

func TestWhatsAppParser_Parse_MessageWithErrors(t *testing.T) {
	value := waValue{
		Messages: []waMessage{
			{
				From:      "11111",
				ID:        "err-001",
				Timestamp: "1700000011",
				Type:      "text",
				Text:      &waText{Body: "Error msg"},
				Errors: []waError{
					{
						Code:    131051,
						Title:   "Message type unknown",
						Message: "Message type is not currently supported",
					},
				},
			},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 1)

	msg := events[0].Message
	require.NotNil(t, msg.Metadata)
	meta := mustUnmarshalMap(t, msg.Metadata)
	errors, ok := meta["errors"]
	require.True(t, ok, "metadata must contain 'errors' key")
	errSlice, ok := errors.([]interface{})
	require.True(t, ok)
	require.Len(t, errSlice, 1)
}

// ---- Message with from_logical_id --------------------------------------------

func TestWhatsAppParser_Parse_MessageWithFromLogicalID(t *testing.T) {
	value := waValue{
		Messages: []waMessage{
			{
				From:          "10000",
				ID:            "lid-001",
				Timestamp:     "1700000012",
				Type:          "text",
				Text:          &waText{Body: "Hi"},
				FromLogicalID: "logical-abc-123",
			},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 1)

	msg := events[0].Message
	require.NotNil(t, msg.Metadata)
	meta := mustUnmarshalMap(t, msg.Metadata)
	assert.Equal(t, "logical-abc-123", meta["from_logical_id"])
}

// ---- Contact with country_code -----------------------------------------------

func TestWhatsAppParser_Parse_ContactWithCountryCode(t *testing.T) {
	value := waValue{
		Contacts: []waContact{
			{
				Profile:     waProfile{Name: "Bob"},
				WaID:        "20000",
				CountryCode: "US",
			},
		},
		Messages: []waMessage{
			{
				From:      "20000",
				ID:        "cc-001",
				Timestamp: "1700000013",
				Type:      "text",
				Text:      &waText{Body: "Hey"},
			},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 1)

	msg := events[0].Message
	require.NotNil(t, msg.Metadata)
	meta := mustUnmarshalMap(t, msg.Metadata)
	assert.Equal(t, "US", meta["country_code"])
}

// ---- Status updates under field "statuses" ------------------------------------

func TestWhatsAppParser_Parse_StatusUpdates(t *testing.T) {
	payload := waWebhookPayload{
		Entry: []waEntry{
			{
				ID: "entry-s",
				Changes: []waChange{
					{
						Field: "statuses",
						Value: waValue{
							Statuses: []waStatus{
								{ID: "wamid-1", Status: "delivered", Timestamp: "1700000020"},
								{ID: "wamid-2", Status: "read", Timestamp: "1700000021"},
							},
						},
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(payload)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 2)

	assert.Equal(t, EventKindStatus, events[0].Kind)
	require.NotNil(t, events[0].Status)
	assert.Equal(t, "wamid-1", events[0].Status.ExternalMessageID)
	assert.Equal(t, "delivered", events[0].Status.Status)
	assert.Equal(t, time.Unix(1700000020, 0), events[0].Status.Timestamp)

	assert.Equal(t, EventKindStatus, events[1].Kind)
	assert.Equal(t, "wamid-2", events[1].Status.ExternalMessageID)
	assert.Equal(t, "read", events[1].Status.Status)
}

// ---- Statuses alongside messages under field "messages" ----------------------

func TestWhatsAppParser_Parse_StatusesAlongsideMessages(t *testing.T) {
	// WhatsApp often sends statuses + messages in the same value block under
	// the "messages" field.  The parser appends status events after message
	// events in a single pass.
	value := waValue{
		Messages: []waMessage{
			{
				From:      "30000",
				ID:        "msg-x1",
				Timestamp: "1700000030",
				Type:      "text",
				Text:      &waText{Body: "Mixed"},
			},
		},
		Statuses: []waStatus{
			{ID: "wamid-x", Status: "sent", Timestamp: "1700000031"},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	// 1 message event + 1 status event
	require.Len(t, events, 2)

	assert.Equal(t, EventKindMessage, events[0].Kind)
	assert.Equal(t, EventKindStatus, events[1].Kind)
	assert.Equal(t, "wamid-x", events[1].Status.ExternalMessageID)
}

// ---- buildMessageMetadata combinations ---------------------------------------

func TestBuildMessageMetadata_NoFields(t *testing.T) {
	p := &WhatsAppParser{}
	// No FromLogicalID, no countryCode, no Errors — result must be nil.
	m := waMessage{}
	result := p.buildMessageMetadata(m, "")
	assert.Nil(t, result)
}

func TestBuildMessageMetadata_OnlyFromLogicalID(t *testing.T) {
	p := &WhatsAppParser{}
	m := waMessage{FromLogicalID: "logi-1"}
	result := p.buildMessageMetadata(m, "")
	require.NotNil(t, result)
	meta := mustUnmarshalMap(t, result)
	assert.Equal(t, "logi-1", meta["from_logical_id"])
	_, hasCC := meta["country_code"]
	assert.False(t, hasCC)
}

func TestBuildMessageMetadata_OnlyCountryCode(t *testing.T) {
	p := &WhatsAppParser{}
	m := waMessage{}
	result := p.buildMessageMetadata(m, "DE")
	require.NotNil(t, result)
	meta := mustUnmarshalMap(t, result)
	assert.Equal(t, "DE", meta["country_code"])
	_, hasLID := meta["from_logical_id"]
	assert.False(t, hasLID)
}

func TestBuildMessageMetadata_OnlyErrors(t *testing.T) {
	p := &WhatsAppParser{}
	m := waMessage{
		Errors: []waError{
			{Code: 1, Title: "err", Message: "msg"},
		},
	}
	result := p.buildMessageMetadata(m, "")
	require.NotNil(t, result)
	meta := mustUnmarshalMap(t, result)
	_, hasErrors := meta["errors"]
	assert.True(t, hasErrors)
	_, hasLID := meta["from_logical_id"]
	assert.False(t, hasLID)
}

func TestBuildMessageMetadata_AllFields(t *testing.T) {
	p := &WhatsAppParser{}
	m := waMessage{
		FromLogicalID: "logi-all",
		Errors: []waError{
			{Code: 99, Title: "T", Message: "M"},
		},
	}
	result := p.buildMessageMetadata(m, "FR")
	require.NotNil(t, result)
	meta := mustUnmarshalMap(t, result)
	assert.Equal(t, "logi-all", meta["from_logical_id"])
	assert.Equal(t, "FR", meta["country_code"])
	_, hasErrors := meta["errors"]
	assert.True(t, hasErrors)
}

// ---- Timestamp edge cases ----------------------------------------------------

func TestParseUnixString_ZeroFallsToNow(t *testing.T) {
	before := time.Now()
	ts := parseUnixString("0")
	after := time.Now()
	// When the unix value is zero, time.Now() is returned.
	// The returned timestamp must fall within the window [before, after].
	assert.True(t, !ts.Before(before.Add(-time.Second)),
		"timestamp must not be more than 1s before the test start")
	assert.True(t, !ts.After(after.Add(time.Second)),
		"timestamp must not be more than 1s after the test end")
}

func TestParseUnixString_ValidUnix(t *testing.T) {
	ts := parseUnixString("1700000000")
	assert.Equal(t, time.Unix(1700000000, 0), ts)
}

func TestParseUnixString_InvalidString_FallsToNow(t *testing.T) {
	before := time.Now()
	ts := parseUnixString("notanumber")
	// ts == 0 branch → time.Now() was called.  The returned time must not be
	// before the test started.
	assert.False(t, ts.Before(before.Add(-time.Second)))
}

// ---- Multiple messages in a single change ------------------------------------

func TestWhatsAppParser_Parse_MultipleMessages(t *testing.T) {
	value := waValue{
		Messages: []waMessage{
			{From: "1", ID: "m1", Timestamp: "1700000100", Type: "text", Text: &waText{Body: "First"}},
			{From: "2", ID: "m2", Timestamp: "1700000101", Type: "audio", Audio: &waMedia{ID: "a1", MimeType: "audio/ogg"}},
			{From: "3", ID: "m3", Timestamp: "1700000102", Type: "location", Location: &waLocation{Latitude: 1.0, Longitude: 2.0}},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 3)

	assert.Equal(t, "m1", events[0].Message.ExternalMessageID)
	assert.Equal(t, MediaTypeText, events[0].Message.MediaType)

	assert.Equal(t, "m2", events[1].Message.ExternalMessageID)
	assert.Equal(t, MediaTypeAudio, events[1].Message.MediaType)

	assert.Equal(t, "m3", events[2].Message.ExternalMessageID)
	assert.Equal(t, MediaTypeLocation, events[2].Message.MediaType)
}

// ---- Contact name lookup — sender not in contacts list -----------------------

func TestWhatsAppParser_Parse_SenderNotInContacts(t *testing.T) {
	// When the sender's WaID does not match any contact, ContactName is empty.
	value := waValue{
		Contacts: []waContact{
			{Profile: waProfile{Name: "Other Person"}, WaID: "99999"},
		},
		Messages: []waMessage{
			{
				From:      "00001",
				ID:        "nc-001",
				Timestamp: "1700000200",
				Type:      "text",
				Text:      &waText{Body: "Hi"},
			},
		},
	}
	raw := buildWAPayload("messages", value)

	p := &WhatsAppParser{}
	events, err := p.Parse(raw)
	require.NoError(t, err)
	require.Len(t, events, 1)

	msg := events[0].Message
	assert.Equal(t, "", msg.ContactName)
	assert.Equal(t, "00001", msg.ContactPhone)
}

// ---- captionOrPlaceholder unit test -----------------------------------------

func TestCaptionOrPlaceholder_NilMedia(t *testing.T) {
	result := captionOrPlaceholder(nil, "[Fallback]")
	assert.Equal(t, "[Fallback]", result)
}

func TestCaptionOrPlaceholder_EmptyCaption(t *testing.T) {
	result := captionOrPlaceholder(&waMedia{Caption: ""}, "[Fallback]")
	assert.Equal(t, "[Fallback]", result)
}

func TestCaptionOrPlaceholder_WithCaption(t *testing.T) {
	result := captionOrPlaceholder(&waMedia{Caption: "Nice pic"}, "[Fallback]")
	assert.Equal(t, "Nice pic", result)
}
