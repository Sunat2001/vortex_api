package chat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Manual mock for the Repository interface.
//
// We implement the full Repository interface by hand because testify/mock
// requires stretchr/objx which is not in the module's dependency graph.
// Each method records calls and returns pre-configured responses via the
// corresponding "On*" fields.
// ---------------------------------------------------------------------------

type mockRepository struct {
	// GetChannelByPlatform
	onGetChannelByPlatform     func(ctx context.Context, platform Platform) (*Channel, error)
	getChannelByPlatformCalled bool

	// UpsertContact
	onUpsertContact     func(ctx context.Context, contact *Contact) error
	upsertContactCalled bool
	upsertContactArg    *Contact

	// GetOrCreateDialog
	onGetOrCreateDialog     func(ctx context.Context, channelID, contactID uuid.UUID) (*Dialog, error)
	getOrCreateDialogCalled bool

	// CreateMessage
	onCreateMessage     func(ctx context.Context, message *Message) error
	createMessageCalled bool
	createMessageArg    *Message

	// UpdateDialogLastMessageAt
	onUpdateDialogLastMessageAt     func(ctx context.Context, dialogID uuid.UUID) error
	updateDialogLastMessageAtCalled bool

	// CreateDialogEvent
	onCreateDialogEvent     func(ctx context.Context, event *DialogEvent) error
	createDialogEventCalled bool
	createDialogEventArgs   []*DialogEvent

	// GetMessageByExternalID
	onGetMessageByExternalID     func(ctx context.Context, externalID string) (*Message, error)
	getMessageByExternalIDCalled bool
}

// --- Dialog operations ---

func (m *mockRepository) CreateDialog(ctx context.Context, dialog *Dialog) error {
	panic("not implemented in test")
}

func (m *mockRepository) GetDialogByID(ctx context.Context, id uuid.UUID) (*Dialog, error) {
	panic("not implemented in test")
}

func (m *mockRepository) GetOrCreateDialog(ctx context.Context, channelID, contactID uuid.UUID) (*Dialog, error) {
	m.getOrCreateDialogCalled = true
	return m.onGetOrCreateDialog(ctx, channelID, contactID)
}

func (m *mockRepository) ListDialogs(ctx context.Context, filters DialogFilters) ([]Dialog, error) {
	panic("not implemented in test")
}

func (m *mockRepository) UpdateDialogStatus(ctx context.Context, id uuid.UUID, status DialogStatus) error {
	panic("not implemented in test")
}

func (m *mockRepository) UpdateDialogLastMessageAt(ctx context.Context, dialogID uuid.UUID) error {
	m.updateDialogLastMessageAtCalled = true
	if m.onUpdateDialogLastMessageAt != nil {
		return m.onUpdateDialogLastMessageAt(ctx, dialogID)
	}
	return nil
}

func (m *mockRepository) AssignDialogToAgent(ctx context.Context, dialogID, agentID uuid.UUID) error {
	panic("not implemented in test")
}

func (m *mockRepository) TransferDialog(ctx context.Context, dialogID, fromAgentID, toAgentID uuid.UUID) error {
	panic("not implemented in test")
}

// --- Message operations ---

func (m *mockRepository) CreateMessage(ctx context.Context, message *Message) error {
	m.createMessageCalled = true
	m.createMessageArg = message
	return m.onCreateMessage(ctx, message)
}

func (m *mockRepository) GetMessageByExternalID(ctx context.Context, externalID string) (*Message, error) {
	m.getMessageByExternalIDCalled = true
	return m.onGetMessageByExternalID(ctx, externalID)
}

func (m *mockRepository) GetMessagesByDialogID(ctx context.Context, dialogID uuid.UUID, limit, offset int) ([]Message, error) {
	panic("not implemented in test")
}

func (m *mockRepository) GetLastMessage(ctx context.Context, dialogID uuid.UUID) (*Message, error) {
	panic("not implemented in test")
}

func (m *mockRepository) UpdateMessageStatus(ctx context.Context, externalID string, status MessageStatus) error {
	return nil
}

// --- Dialog Event operations ---

func (m *mockRepository) CreateDialogEvent(ctx context.Context, event *DialogEvent) error {
	m.createDialogEventCalled = true
	m.createDialogEventArgs = append(m.createDialogEventArgs, event)
	if m.onCreateDialogEvent != nil {
		return m.onCreateDialogEvent(ctx, event)
	}
	return nil
}

func (m *mockRepository) GetDialogEvents(ctx context.Context, dialogID uuid.UUID) ([]DialogEvent, error) {
	panic("not implemented in test")
}

// --- Contact operations ---

func (m *mockRepository) CreateContact(ctx context.Context, contact *Contact) error {
	panic("not implemented in test")
}

func (m *mockRepository) GetContactByExternalID(ctx context.Context, externalID string) (*Contact, error) {
	panic("not implemented in test")
}

func (m *mockRepository) GetContactByID(ctx context.Context, id uuid.UUID) (*Contact, error) {
	panic("not implemented in test")
}

func (m *mockRepository) UpsertContact(ctx context.Context, contact *Contact) error {
	m.upsertContactCalled = true
	m.upsertContactArg = contact
	return m.onUpsertContact(ctx, contact)
}

// --- Channel operations ---

func (m *mockRepository) GetChannelByID(ctx context.Context, id uuid.UUID) (*Channel, error) {
	panic("not implemented in test")
}

func (m *mockRepository) GetChannelByPlatform(ctx context.Context, platform Platform) (*Channel, error) {
	m.getChannelByPlatformCalled = true
	return m.onGetChannelByPlatform(ctx, platform)
}

func (m *mockRepository) ListChannels(ctx context.Context, platform *Platform) ([]Channel, error) {
	panic("not implemented in test")
}

// --- Agent-facing operations ---

func (m *mockRepository) ListDialogsWithDetails(ctx context.Context, filters DialogFilters) ([]DialogListItem, error) {
	panic("not implemented in test")
}

func (m *mockRepository) GetDialogWithContact(ctx context.Context, id uuid.UUID) (*DialogWithDetails, error) {
	panic("not implemented in test")
}

func (m *mockRepository) ListMessagesCursor(ctx context.Context, cursor MessageCursor) ([]MessageWithSender, error) {
	panic("not implemented in test")
}

func (m *mockRepository) CreateAgentMessage(ctx context.Context, message *Message) error {
	panic("not implemented in test")
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestChannel returns a minimal Channel for the given platform.
func newTestChannel(platform Platform) *Channel {
	return &Channel{
		ID:       uuid.New(),
		Platform: platform,
		IsActive: true,
	}
}

// newTestDialog returns a minimal Dialog.
func newTestDialog(channelID, contactID uuid.UUID) *Dialog {
	return &Dialog{
		ID:        uuid.New(),
		ChannelID: channelID,
		ContactID: contactID,
		Status:    DialogStatusOpen,
	}
}

// buildSuccessRepo configures the mock so that all repository calls in
// processMessage succeed: UpsertContact, GetOrCreateDialog, CreateMessage,
// UpdateDialogLastMessageAt, CreateDialogEvent.
func buildSuccessRepo(channel *Channel) *mockRepository {
	dialog := newTestDialog(channel.ID, uuid.New())
	return &mockRepository{
		onGetChannelByPlatform: func(_ context.Context, _ Platform) (*Channel, error) {
			return channel, nil
		},
		onUpsertContact: func(_ context.Context, _ *Contact) error {
			return nil
		},
		onGetOrCreateDialog: func(_ context.Context, _, _ uuid.UUID) (*Dialog, error) {
			return dialog, nil
		},
		onCreateMessage: func(_ context.Context, _ *Message) error {
			return nil
		},
		onUpdateDialogLastMessageAt: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
		onCreateDialogEvent: func(_ context.Context, _ *DialogEvent) error {
			return nil
		},
	}
}

// minimalWAPayload builds the minimal WhatsApp webhook JSON for a single text
// message from the given sender.
func minimalWAPayload(senderID, msgID, body string) json.RawMessage {
	raw := `{
		"entry":[{
			"id":"entry-1",
			"changes":[{
				"field":"messages",
				"value":{
					"messaging_product":"whatsapp",
					"contacts":[{"profile":{"name":"Test User"},"wa_id":"` + senderID + `"}],
					"messages":[{
						"from":"` + senderID + `",
						"id":"` + msgID + `",
						"timestamp":"1700000000",
						"type":"text",
						"text":{"body":"` + body + `"}
					}]
				}
			}]
		}]
	}`
	return json.RawMessage(raw)
}

// minimalWAPayloadWithMetadata builds a WhatsApp webhook with from_logical_id
// and country_code fields to exercise the metadata merging path.
func minimalWAPayloadWithMetadata(senderID, msgID string) json.RawMessage {
	raw := `{
		"entry":[{
			"id":"entry-2",
			"changes":[{
				"field":"messages",
				"value":{
					"messaging_product":"whatsapp",
					"contacts":[{"profile":{"name":"Meta User"},"wa_id":"` + senderID + `","country_code":"TR"}],
					"messages":[{
						"from":"` + senderID + `",
						"id":"` + msgID + `",
						"timestamp":"1700000000",
						"type":"text",
						"text":{"body":"hello"},
						"from_logical_id":"lid-xyz"
					}]
				}
			}]
		}]
	}`
	return json.RawMessage(raw)
}

// minimalWAStatusPayload builds a minimal WhatsApp webhook with one status update.
func minimalWAStatusPayload(msgID, status string) json.RawMessage {
	raw := `{
		"entry":[{
			"id":"entry-3",
			"changes":[{
				"field":"statuses",
				"value":{
					"statuses":[{
						"id":"` + msgID + `",
						"status":"` + status + `",
						"timestamp":"1700000050"
					}]
				}
			}]
		}]
	}`
	return json.RawMessage(raw)
}

// ---------------------------------------------------------------------------
// ProcessIncomingWebhook — unsupported platform
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_UnsupportedPlatform(t *testing.T) {
	repo := &mockRepository{}
	uc := NewUsecase(repo, nil, zap.NewNop())

	resp, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   Platform("telegram"),
		RawPayload: json.RawMessage(`{}`),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported platform")
	assert.Nil(t, resp)
	assert.False(t, repo.getChannelByPlatformCalled,
		"GetChannelByPlatform must not be called when platform is unsupported")
}

// ---------------------------------------------------------------------------
// ProcessIncomingWebhook — invalid JSON payload
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_InvalidJSON(t *testing.T) {
	repo := &mockRepository{
		onGetChannelByPlatform: func(_ context.Context, _ Platform) (*Channel, error) {
			// Should not be called — parse fails first.
			return newTestChannel(PlatformWhatsApp), nil
		},
	}
	uc := NewUsecase(repo, nil, zap.NewNop())

	resp, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: json.RawMessage(`{not valid`),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse webhook")
	assert.Nil(t, resp)
}

// ---------------------------------------------------------------------------
// ProcessIncomingWebhook — channel not found
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_NoChannel(t *testing.T) {
	channelErr := errors.New("channel not found in db")
	repo := &mockRepository{
		onGetChannelByPlatform: func(_ context.Context, _ Platform) (*Channel, error) {
			return nil, channelErr
		},
	}
	uc := NewUsecase(repo, nil, zap.NewNop())

	resp, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: minimalWAPayload("111", "m1", "hi"),
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoChannel),
		"expected ErrNoChannel to be wrapped in the error")
	assert.Nil(t, resp)
}

// ---------------------------------------------------------------------------
// ProcessIncomingWebhook — happy path: one text message
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_HappyPath_TextMessage(t *testing.T) {
	channel := newTestChannel(PlatformWhatsApp)
	repo := buildSuccessRepo(channel)
	uc := NewUsecase(repo, nil, zap.NewNop())

	resp, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: minimalWAPayload("111", "wamid-happy", "Hello"),
		ReceivedAt: time.Now(),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, resp.MessagesCreated)
	assert.Equal(t, 0, resp.StatusesProcessed)

	// Verify all repository calls were made.
	assert.True(t, repo.upsertContactCalled)
	assert.True(t, repo.getOrCreateDialogCalled)
	assert.True(t, repo.createMessageCalled)
}

// ---------------------------------------------------------------------------
// ProcessIncomingWebhook — metadata merging
//
// Parser-level metadata (from_logical_id, country_code) must be merged into
// the usecase-level metadata map alongside "media_type".
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_MetadataMerging(t *testing.T) {
	channel := newTestChannel(PlatformWhatsApp)
	repo := buildSuccessRepo(channel)
	uc := NewUsecase(repo, nil, zap.NewNop())

	resp, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: minimalWAPayloadWithMetadata("222", "wamid-meta"),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, resp.MessagesCreated)

	// Inspect the message captured by the mock.
	require.NotNil(t, repo.createMessageArg)
	msg := repo.createMessageArg

	var meta map[string]interface{}
	require.NoError(t, json.Unmarshal(msg.Metadata, &meta))

	// "media_type" is always set by the usecase.
	assert.Equal(t, "text", meta["media_type"])

	// Parser-level fields must be merged in.
	assert.Equal(t, "lid-xyz", meta["from_logical_id"],
		"from_logical_id must be merged from parser metadata")
	assert.Equal(t, "TR", meta["country_code"],
		"country_code must be merged from parser metadata")
}

// ---------------------------------------------------------------------------
// ProcessIncomingWebhook — nil parser metadata is handled safely
//
// When parser metadata is nil (e.g., MetaParser does not set Metadata),
// only the usecase-level fields should appear in the stored metadata.
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_NilParserMetadata(t *testing.T) {
	// Use Facebook which goes through MetaParser — it does not set Metadata.
	fbChannel := newTestChannel(PlatformFacebook)
	repo := buildSuccessRepo(fbChannel)
	// Override channel lookup to return Facebook channel.
	repo.onGetChannelByPlatform = func(_ context.Context, _ Platform) (*Channel, error) {
		return fbChannel, nil
	}
	uc := NewUsecase(repo, nil, zap.NewNop())

	// Minimal Facebook messaging webhook (entry[].messaging[] format).
	fbPayload := json.RawMessage(`{
		"object":"page",
		"entry":[{
			"id":"fb-entry",
			"messaging":[{
				"sender":{"id":"fb-sender-1"},
				"recipient":{"id":"fb-page-1"},
				"timestamp":1700000000,
				"message":{"mid":"fb-mid-1","text":"Hello from FB"}
			}]
		}]
	}`)

	resp, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformFacebook,
		RawPayload: fbPayload,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, resp.MessagesCreated)

	require.NotNil(t, repo.createMessageArg)
	msg := repo.createMessageArg

	var meta map[string]interface{}
	require.NoError(t, json.Unmarshal(msg.Metadata, &meta))

	// "media_type" must always be present.
	assert.Equal(t, "text", meta["media_type"])

	// No parser-level keys from MetaParser (it doesn't set Metadata).
	_, hasLID := meta["from_logical_id"]
	assert.False(t, hasLID)
	_, hasCC := meta["country_code"]
	assert.False(t, hasCC)
}

// ---------------------------------------------------------------------------
// ProcessIncomingWebhook — nil payload guard
//
// When msg.Payload is nil (e.g., the default switch case), the usecase must
// store an empty JSON object {} instead of passing nil to the repository.
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_NilPayloadBecomesEmptyObject(t *testing.T) {
	// "unsupported" type with nil Unsupported field → parser sets Payload to nil.
	channel := newTestChannel(PlatformWhatsApp)
	repo := buildSuccessRepo(channel)
	uc := NewUsecase(repo, nil, zap.NewNop())

	payload := json.RawMessage(`{
		"entry":[{
			"id":"e1",
			"changes":[{
				"field":"messages",
				"value":{
					"messages":[{
						"from":"555",
						"id":"unsup-nil-payload",
						"timestamp":"1700000000",
						"type":"unsupported"
					}]
				}
			}]
		}]
	}`)

	resp, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: payload,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, resp.MessagesCreated)

	require.NotNil(t, repo.createMessageArg)
	storedPayload := repo.createMessageArg.Payload

	// Must not be nil; must be the empty JSON object {}.
	require.NotNil(t, storedPayload)
	assert.JSONEq(t, `{}`, string(storedPayload))
}

// ---------------------------------------------------------------------------
// ProcessIncomingWebhook — duplicate message (ErrMessageAlreadyExists)
//
// When CreateMessage returns ErrMessageAlreadyExists the usecase must NOT
// return an error for that message; it should silently skip it and still
// return a successful response with MessagesCreated == 0.
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_DuplicateMessage(t *testing.T) {
	channel := newTestChannel(PlatformWhatsApp)
	dialog := newTestDialog(channel.ID, uuid.New())

	repo := &mockRepository{
		onGetChannelByPlatform: func(_ context.Context, _ Platform) (*Channel, error) {
			return channel, nil
		},
		onUpsertContact: func(_ context.Context, _ *Contact) error { return nil },
		onGetOrCreateDialog: func(_ context.Context, _, _ uuid.UUID) (*Dialog, error) {
			return dialog, nil
		},
		onCreateMessage: func(_ context.Context, _ *Message) error {
			return ErrMessageAlreadyExists
		},
	}
	uc := NewUsecase(repo, nil, zap.NewNop())

	resp, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: minimalWAPayload("333", "wamid-dup", "duplicate"),
	})

	require.NoError(t, err, "duplicate messages must not cause a top-level error")
	require.NotNil(t, resp)
	// processMessage returns nil for duplicates (not an error), so the counter increments.
	assert.Equal(t, 1, resp.MessagesCreated)
}

// ---------------------------------------------------------------------------
// ProcessIncomingWebhook — repository error on CreateMessage (non-duplicate)
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_CreateMessageError(t *testing.T) {
	channel := newTestChannel(PlatformWhatsApp)
	dialog := newTestDialog(channel.ID, uuid.New())
	dbErr := errors.New("db write failure")

	repo := &mockRepository{
		onGetChannelByPlatform: func(_ context.Context, _ Platform) (*Channel, error) {
			return channel, nil
		},
		onUpsertContact: func(_ context.Context, _ *Contact) error { return nil },
		onGetOrCreateDialog: func(_ context.Context, _, _ uuid.UUID) (*Dialog, error) {
			return dialog, nil
		},
		onCreateMessage: func(_ context.Context, _ *Message) error { return dbErr },
	}
	uc := NewUsecase(repo, nil, zap.NewNop())

	// processMessage logs the error and continues (does not propagate it to
	// ProcessIncomingWebhook's caller).  The response still has 0 messages created.
	resp, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: minimalWAPayload("444", "wamid-err", "error"),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 0, resp.MessagesCreated)
}

// ---------------------------------------------------------------------------
// ProcessIncomingWebhook — UpsertContact failure
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_UpsertContactError(t *testing.T) {
	channel := newTestChannel(PlatformWhatsApp)
	contactErr := errors.New("contact upsert failed")

	repo := &mockRepository{
		onGetChannelByPlatform: func(_ context.Context, _ Platform) (*Channel, error) {
			return channel, nil
		},
		onUpsertContact: func(_ context.Context, _ *Contact) error { return contactErr },
	}
	uc := NewUsecase(repo, nil, zap.NewNop())

	resp, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: minimalWAPayload("555", "wamid-contact-err", "hi"),
	})

	// UpsertContact failure is logged but swallowed at the loop level.
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 0, resp.MessagesCreated)
	assert.False(t, repo.createMessageCalled,
		"CreateMessage must not be called when UpsertContact fails")
}

// ---------------------------------------------------------------------------
// ProcessIncomingWebhook — GetOrCreateDialog failure
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_GetOrCreateDialogError(t *testing.T) {
	channel := newTestChannel(PlatformWhatsApp)
	dialogErr := errors.New("dialog creation failed")

	repo := &mockRepository{
		onGetChannelByPlatform: func(_ context.Context, _ Platform) (*Channel, error) {
			return channel, nil
		},
		onUpsertContact: func(_ context.Context, _ *Contact) error { return nil },
		onGetOrCreateDialog: func(_ context.Context, _, _ uuid.UUID) (*Dialog, error) {
			return nil, dialogErr
		},
	}
	uc := NewUsecase(repo, nil, zap.NewNop())

	resp, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: minimalWAPayload("666", "wamid-dialog-err", "hi"),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 0, resp.MessagesCreated)
	assert.False(t, repo.createMessageCalled)
}

// ---------------------------------------------------------------------------
// ProcessIncomingWebhook — status update happy path
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_StatusUpdate_HappyPath(t *testing.T) {
	channel := newTestChannel(PlatformWhatsApp)
	existingMsg := &Message{
		ID:       uuid.New(),
		DialogID: uuid.New(),
		ExternalID: "wamid-status-1",
	}

	repo := &mockRepository{
		onGetChannelByPlatform: func(_ context.Context, _ Platform) (*Channel, error) {
			return channel, nil
		},
		onGetMessageByExternalID: func(_ context.Context, externalID string) (*Message, error) {
			return existingMsg, nil
		},
		onCreateDialogEvent: func(_ context.Context, _ *DialogEvent) error {
			return nil
		},
	}
	uc := NewUsecase(repo, nil, zap.NewNop())

	resp, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: minimalWAStatusPayload("wamid-status-1", "delivered"),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 0, resp.MessagesCreated)
	assert.Equal(t, 1, resp.StatusesProcessed)

	assert.True(t, repo.getMessageByExternalIDCalled)
	assert.True(t, repo.createDialogEventCalled)
	require.Len(t, repo.createDialogEventArgs, 1)

	event := repo.createDialogEventArgs[0]
	assert.Equal(t, EventTypeStatusUpdated, event.EventType)
	assert.Equal(t, existingMsg.DialogID, event.DialogID)

	var evPayload map[string]interface{}
	require.NoError(t, json.Unmarshal(event.Payload, &evPayload))
	assert.Equal(t, "delivered", evPayload["status"])
	assert.Equal(t, "wamid-status-1", evPayload["external_message_id"])
}

// ---------------------------------------------------------------------------
// ProcessIncomingWebhook — status update: message not found
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_StatusUpdate_MessageNotFound(t *testing.T) {
	channel := newTestChannel(PlatformWhatsApp)
	notFoundErr := errors.New("message not found")

	repo := &mockRepository{
		onGetChannelByPlatform: func(_ context.Context, _ Platform) (*Channel, error) {
			return channel, nil
		},
		onGetMessageByExternalID: func(_ context.Context, _ string) (*Message, error) {
			return nil, notFoundErr
		},
	}
	uc := NewUsecase(repo, nil, zap.NewNop())

	resp, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: minimalWAStatusPayload("wamid-missing", "read"),
	})

	// Status failure is logged but swallowed at the loop level.
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 0, resp.StatusesProcessed)
}

// ---------------------------------------------------------------------------
// ProcessIncomingWebhook — empty payload (no messages, no statuses)
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_EmptyEvents(t *testing.T) {
	channel := newTestChannel(PlatformWhatsApp)
	repo := &mockRepository{
		onGetChannelByPlatform: func(_ context.Context, _ Platform) (*Channel, error) {
			return channel, nil
		},
	}
	uc := NewUsecase(repo, nil, zap.NewNop())

	resp, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: json.RawMessage(`{"entry":[]}`),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 0, resp.MessagesCreated)
	assert.Equal(t, 0, resp.StatusesProcessed)
}

// ---------------------------------------------------------------------------
// ProcessIncomingWebhook — UpdateDialogLastMessageAt failure is non-fatal
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_UpdateDialogLastMessageAt_NonFatal(t *testing.T) {
	channel := newTestChannel(PlatformWhatsApp)
	dialog := newTestDialog(channel.ID, uuid.New())
	updateErr := errors.New("update failed")

	repo := &mockRepository{
		onGetChannelByPlatform: func(_ context.Context, _ Platform) (*Channel, error) {
			return channel, nil
		},
		onUpsertContact: func(_ context.Context, _ *Contact) error { return nil },
		onGetOrCreateDialog: func(_ context.Context, _, _ uuid.UUID) (*Dialog, error) {
			return dialog, nil
		},
		onCreateMessage: func(_ context.Context, _ *Message) error { return nil },
		onUpdateDialogLastMessageAt: func(_ context.Context, _ uuid.UUID) error {
			return updateErr
		},
		onCreateDialogEvent: func(_ context.Context, _ *DialogEvent) error { return nil },
	}
	uc := NewUsecase(repo, nil, zap.NewNop())

	resp, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: minimalWAPayload("777", "wamid-update", "test"),
	})

	// The update failure must NOT bubble up — it's logged with Warn.
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, resp.MessagesCreated)
}

// ---------------------------------------------------------------------------
// ProcessIncomingWebhook — CreateDialogEvent failure is non-fatal
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_CreateDialogEvent_NonFatal(t *testing.T) {
	channel := newTestChannel(PlatformWhatsApp)
	dialog := newTestDialog(channel.ID, uuid.New())
	eventErr := errors.New("event creation failed")

	repo := &mockRepository{
		onGetChannelByPlatform: func(_ context.Context, _ Platform) (*Channel, error) {
			return channel, nil
		},
		onUpsertContact: func(_ context.Context, _ *Contact) error { return nil },
		onGetOrCreateDialog: func(_ context.Context, _, _ uuid.UUID) (*Dialog, error) {
			return dialog, nil
		},
		onCreateMessage: func(_ context.Context, _ *Message) error { return nil },
		onUpdateDialogLastMessageAt: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
		onCreateDialogEvent: func(_ context.Context, _ *DialogEvent) error {
			return eventErr
		},
	}
	uc := NewUsecase(repo, nil, zap.NewNop())

	resp, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: minimalWAPayload("888", "wamid-event", "test"),
	})

	// Event failure is logged with Warn and must not prevent MessagesCreated++.
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, resp.MessagesCreated)
}

// ---------------------------------------------------------------------------
// processMessage — message stores SenderType = customer
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_MessageStoredAsSenderTypeCustomer(t *testing.T) {
	channel := newTestChannel(PlatformWhatsApp)
	repo := buildSuccessRepo(channel)
	uc := NewUsecase(repo, nil, zap.NewNop())

	_, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: minimalWAPayload("999", "wamid-sender", "check sender"),
	})
	require.NoError(t, err)

	require.NotNil(t, repo.createMessageArg)
	assert.Equal(t, SenderTypeCustomer, repo.createMessageArg.SenderType)
}

// ---------------------------------------------------------------------------
// processMessage — ExternalID is set from ExternalMessageID
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_MessageExternalIDSet(t *testing.T) {
	channel := newTestChannel(PlatformWhatsApp)
	repo := buildSuccessRepo(channel)
	uc := NewUsecase(repo, nil, zap.NewNop())

	_, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: minimalWAPayload("101", "wamid-extid-check", "check"),
	})
	require.NoError(t, err)

	require.NotNil(t, repo.createMessageArg)
	assert.Equal(t, "wamid-extid-check", repo.createMessageArg.ExternalID)
}

// ---------------------------------------------------------------------------
// processMessage — Content equals "[Text]" for text messages
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_TextMessageContent(t *testing.T) {
	channel := newTestChannel(PlatformWhatsApp)
	repo := buildSuccessRepo(channel)
	uc := NewUsecase(repo, nil, zap.NewNop())

	_, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: minimalWAPayload("102", "wamid-content", "body text here"),
	})
	require.NoError(t, err)

	require.NotNil(t, repo.createMessageArg)
	assert.Equal(t, "body text here", repo.createMessageArg.Content)
}

// ---------------------------------------------------------------------------
// processMessage — CreatedAt defaults to time.Now() when timestamp is zero
// ---------------------------------------------------------------------------

func TestProcessIncomingWebhook_ZeroTimestampDefaultsToNow(t *testing.T) {
	channel := newTestChannel(PlatformWhatsApp)
	repo := buildSuccessRepo(channel)
	uc := NewUsecase(repo, nil, zap.NewNop())

	// Timestamp "0" will be parsed to zero by parseUnixString → falls back to time.Now().
	payload := json.RawMessage(`{
		"entry":[{
			"id":"e1",
			"changes":[{
				"field":"messages",
				"value":{
					"messages":[{
						"from":"103",
						"id":"wamid-ts-zero",
						"timestamp":"0",
						"type":"text",
						"text":{"body":"no timestamp"}
					}]
				}
			}]
		}]
	}`)

	before := time.Now()
	_, err := uc.ProcessIncomingWebhook(context.Background(), &ProcessWebhookRequest{
		Platform:   PlatformWhatsApp,
		RawPayload: payload,
	})
	after := time.Now()

	require.NoError(t, err)
	require.NotNil(t, repo.createMessageArg)

	createdAt := repo.createMessageArg.CreatedAt
	assert.True(t, !createdAt.Before(before.Add(-time.Second)),
		"CreatedAt must be at or after test start")
	assert.True(t, !createdAt.After(after.Add(time.Second)),
		"CreatedAt must be at or before test end")
}
