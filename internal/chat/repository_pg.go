package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgRepository implements Repository interface using pgx/v5
type PgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL repository
func NewRepository(pool *pgxpool.Pool) Repository {
	return &PgRepository{
		pool: pool,
	}
}

// Contact operations

func (r *PgRepository) CreateContact(ctx context.Context, contact *Contact) error {
	query := `
		INSERT INTO contacts (id, external_id, name, phone, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.pool.Exec(ctx, query, contact.ID, contact.ExternalID, contact.Name, contact.Phone, contact.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create contact: %w", err)
	}
	return nil
}

func (r *PgRepository) GetContactByExternalID(ctx context.Context, externalID string) (*Contact, error) {
	query := `SELECT id, external_id, name, phone, created_at FROM contacts WHERE external_id = $1`

	var contact Contact
	err := r.pool.QueryRow(ctx, query, externalID).Scan(
		&contact.ID,
		&contact.ExternalID,
		&contact.Name,
		&contact.Phone,
		&contact.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get contact by external_id: %w", err)
	}

	return &contact, nil
}

func (r *PgRepository) GetContactByID(ctx context.Context, id uuid.UUID) (*Contact, error) {
	query := `SELECT id, external_id, name, phone, created_at FROM contacts WHERE id = $1`

	var contact Contact
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&contact.ID,
		&contact.ExternalID,
		&contact.Name,
		&contact.Phone,
		&contact.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get contact by id: %w", err)
	}

	return &contact, nil
}

// UpsertContact creates or updates a contact (used by webhook worker)
func (r *PgRepository) UpsertContact(ctx context.Context, contact *Contact) error {
	query := `
		INSERT INTO contacts (id, external_id, name, phone, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (external_id) DO UPDATE
		SET name = EXCLUDED.name,
		    phone = EXCLUDED.phone
		RETURNING id
	`
	err := r.pool.QueryRow(ctx, query,
		contact.ID,
		contact.ExternalID,
		contact.Name,
		contact.Phone,
		contact.CreatedAt,
	).Scan(&contact.ID)

	if err != nil {
		return fmt.Errorf("failed to upsert contact: %w", err)
	}

	return nil
}

// Channel operations

func (r *PgRepository) GetChannelByID(ctx context.Context, id uuid.UUID) (*Channel, error) {
	query := `SELECT id, platform, credentials, is_active, created_at FROM channels WHERE id = $1`

	var channel Channel
	var credentialsJSON []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&channel.ID,
		&channel.Platform,
		&credentialsJSON,
		&channel.IsActive,
		&channel.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get channel by id: %w", err)
	}

	channel.Credentials = json.RawMessage(credentialsJSON)
	return &channel, nil
}

func (r *PgRepository) GetChannelByPlatform(ctx context.Context, platform Platform) (*Channel, error) {
	query := `SELECT id, platform, credentials, is_active, created_at FROM channels WHERE platform = $1 AND is_active = true LIMIT 1`

	var channel Channel
	var credentialsJSON []byte

	err := r.pool.QueryRow(ctx, query, platform).Scan(
		&channel.ID,
		&channel.Platform,
		&credentialsJSON,
		&channel.IsActive,
		&channel.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get channel by platform: %w", err)
	}

	channel.Credentials = json.RawMessage(credentialsJSON)
	return &channel, nil
}

func (r *PgRepository) ListChannels(ctx context.Context, platform *Platform) ([]Channel, error) {
	query := `SELECT id, platform, credentials, is_active, created_at FROM channels WHERE 1=1`
	args := []interface{}{}

	if platform != nil {
		query += ` AND platform = $1`
		args = append(args, *platform)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list channels: %w", err)
	}
	defer rows.Close()

	channels := []Channel{}
	for rows.Next() {
		var channel Channel
		var credentialsJSON []byte

		if err := rows.Scan(
			&channel.ID,
			&channel.Platform,
			&credentialsJSON,
			&channel.IsActive,
			&channel.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan channel: %w", err)
		}

		channel.Credentials = json.RawMessage(credentialsJSON)
		channels = append(channels, channel)
	}

	return channels, nil
}

// Dialog operations

func (r *PgRepository) CreateDialog(ctx context.Context, dialog *Dialog) error {
	query := `
		INSERT INTO dialogs (id, channel_id, contact_id, current_agent_id, source_ad_id, status, tags, last_message_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.pool.Exec(ctx, query,
		dialog.ID,
		dialog.ChannelID,
		dialog.ContactID,
		dialog.CurrentAgentID,
		dialog.SourceAdID,
		dialog.Status,
		dialog.Tags,
		dialog.LastMessageAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create dialog: %w", err)
	}

	return nil
}

func (r *PgRepository) GetDialogByID(ctx context.Context, id uuid.UUID) (*Dialog, error) {
	query := `
		SELECT id, channel_id, contact_id, current_agent_id, source_ad_id, status, tags, last_message_at
		FROM dialogs WHERE id = $1
	`

	var dialog Dialog
	var tagsJSON []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&dialog.ID,
		&dialog.ChannelID,
		&dialog.ContactID,
		&dialog.CurrentAgentID,
		&dialog.SourceAdID,
		&dialog.Status,
		&tagsJSON,
		&dialog.LastMessageAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get dialog by id: %w", err)
	}

	dialog.Tags = json.RawMessage(tagsJSON)
	return &dialog, nil
}

func (r *PgRepository) GetOrCreateDialog(ctx context.Context, channelID, contactID uuid.UUID) (*Dialog, error) {
	// Atomic upsert using CTE to avoid race condition when concurrent workers
	// process messages from the same contact on the same channel simultaneously.
	//
	// The INSERT uses ON CONFLICT with the partial unique index
	// idx_dialogs_channel_contact_active (channel_id, contact_id) WHERE status IN ('open','pending').
	// If an active dialog already exists, DO NOTHING; the subsequent SELECT picks it up.
	query := `
		WITH new_dialog AS (
			INSERT INTO dialogs (id, channel_id, contact_id, status, tags, last_message_at)
			VALUES ($1, $2, $3, 'open', '[]', NOW())
			ON CONFLICT (channel_id, contact_id) WHERE status IN ('open', 'pending')
			DO NOTHING
			RETURNING id, channel_id, contact_id, current_agent_id, source_ad_id, status, tags, last_message_at
		)
		SELECT id, channel_id, contact_id, current_agent_id, source_ad_id, status, tags, last_message_at
		FROM new_dialog
		UNION ALL
		SELECT id, channel_id, contact_id, current_agent_id, source_ad_id, status, tags, last_message_at
		FROM dialogs
		WHERE channel_id = $2 AND contact_id = $3 AND status IN ('open', 'pending')
		  AND NOT EXISTS (SELECT 1 FROM new_dialog)
		ORDER BY last_message_at DESC
		LIMIT 1
	`

	newID := uuid.New()

	var dialog Dialog
	var tagsJSON []byte

	err := r.pool.QueryRow(ctx, query, newID, channelID, contactID).Scan(
		&dialog.ID,
		&dialog.ChannelID,
		&dialog.ContactID,
		&dialog.CurrentAgentID,
		&dialog.SourceAdID,
		&dialog.Status,
		&tagsJSON,
		&dialog.LastMessageAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create dialog: %w", err)
	}

	dialog.Tags = json.RawMessage(tagsJSON)
	return &dialog, nil
}

func (r *PgRepository) ListDialogs(ctx context.Context, filters DialogFilters) ([]Dialog, error) {
	query := `
		SELECT id, channel_id, contact_id, current_agent_id, source_ad_id, status, tags, last_message_at
		FROM dialogs WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if filters.AgentID != nil {
		query += fmt.Sprintf(` AND current_agent_id = $%d`, argIdx)
		args = append(args, *filters.AgentID)
		argIdx++
	}

	if filters.Status != nil {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, *filters.Status)
		argIdx++
	}

	query += ` ORDER BY last_message_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list dialogs: %w", err)
	}
	defer rows.Close()

	dialogs := []Dialog{}
	for rows.Next() {
		var dialog Dialog
		var tagsJSON []byte

		if err := rows.Scan(
			&dialog.ID,
			&dialog.ChannelID,
			&dialog.ContactID,
			&dialog.CurrentAgentID,
			&dialog.SourceAdID,
			&dialog.Status,
			&tagsJSON,
			&dialog.LastMessageAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan dialog: %w", err)
		}

		dialog.Tags = json.RawMessage(tagsJSON)
		dialogs = append(dialogs, dialog)
	}

	return dialogs, nil
}

func (r *PgRepository) UpdateDialogStatus(ctx context.Context, id uuid.UUID, status DialogStatus) error {
	query := `UPDATE dialogs SET status = $1 WHERE id = $2`
	result, err := r.pool.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update dialog status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("dialog not found")
	}

	return nil
}

func (r *PgRepository) UpdateDialogLastMessageAt(ctx context.Context, dialogID uuid.UUID) error {
	query := `UPDATE dialogs SET last_message_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, dialogID)
	if err != nil {
		return fmt.Errorf("failed to update dialog last_message_at: %w", err)
	}
	return nil
}

func (r *PgRepository) AssignDialogToAgent(ctx context.Context, dialogID, agentID uuid.UUID) error {
	query := `UPDATE dialogs SET current_agent_id = $1 WHERE id = $2`
	result, err := r.pool.Exec(ctx, query, agentID, dialogID)
	if err != nil {
		return fmt.Errorf("failed to assign dialog to agent: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("dialog not found")
	}

	return nil
}

func (r *PgRepository) TransferDialog(ctx context.Context, dialogID, fromAgentID, toAgentID uuid.UUID) error {
	query := `
		UPDATE dialogs
		SET current_agent_id = $1
		WHERE id = $2 AND current_agent_id = $3
	`
	result, err := r.pool.Exec(ctx, query, toAgentID, dialogID, fromAgentID)
	if err != nil {
		return fmt.Errorf("failed to transfer dialog: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("dialog not found or not assigned to source agent")
	}

	return nil
}

// Message operations

func (r *PgRepository) CreateMessage(ctx context.Context, message *Message) error {
	query := `
		INSERT INTO messages (id, dialog_id, sender_type, external_id, content, payload, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.pool.Exec(ctx, query,
		message.ID,
		message.DialogID,
		message.SenderType,
		nilIfEmpty(message.ExternalID),
		message.Content,
		message.Payload,
		message.Metadata,
		message.CreatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrMessageAlreadyExists
		}
		return fmt.Errorf("failed to create message: %w", err)
	}

	return nil
}

func (r *PgRepository) GetMessageByExternalID(ctx context.Context, externalID string) (*Message, error) {
	query := `
		SELECT id, dialog_id, sender_type, external_id, content, payload, metadata, created_at
		FROM messages
		WHERE external_id = $1
		LIMIT 1
	`

	var message Message
	var extID *string
	var payloadJSON, metadataJSON []byte

	err := r.pool.QueryRow(ctx, query, externalID).Scan(
		&message.ID,
		&message.DialogID,
		&message.SenderType,
		&extID,
		&message.Content,
		&payloadJSON,
		&metadataJSON,
		&message.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get message by external_id: %w", err)
	}

	if extID != nil {
		message.ExternalID = *extID
	}
	message.Payload = json.RawMessage(payloadJSON)
	message.Metadata = json.RawMessage(metadataJSON)
	return &message, nil
}

func (r *PgRepository) GetMessagesByDialogID(ctx context.Context, dialogID uuid.UUID, limit, offset int) ([]Message, error) {
	query := `
		SELECT id, dialog_id, sender_type, external_id, content, payload, metadata, created_at
		FROM messages
		WHERE dialog_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, dialogID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages by dialog_id: %w", err)
	}
	defer rows.Close()

	messages := []Message{}
	for rows.Next() {
		var message Message
		var externalID *string
		var payloadJSON, metadataJSON []byte

		if err := rows.Scan(
			&message.ID,
			&message.DialogID,
			&message.SenderType,
			&externalID,
			&message.Content,
			&payloadJSON,
			&metadataJSON,
			&message.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		if externalID != nil {
			message.ExternalID = *externalID
		}
		message.Payload = json.RawMessage(payloadJSON)
		message.Metadata = json.RawMessage(metadataJSON)
		messages = append(messages, message)
	}

	return messages, nil
}

func (r *PgRepository) GetLastMessage(ctx context.Context, dialogID uuid.UUID) (*Message, error) {
	query := `
		SELECT id, dialog_id, sender_type, external_id, content, payload, metadata, created_at
		FROM messages
		WHERE dialog_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var message Message
	var externalID *string
	var payloadJSON, metadataJSON []byte

	err := r.pool.QueryRow(ctx, query, dialogID).Scan(
		&message.ID,
		&message.DialogID,
		&message.SenderType,
		&externalID,
		&message.Content,
		&payloadJSON,
		&metadataJSON,
		&message.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get last message: %w", err)
	}

	if externalID != nil {
		message.ExternalID = *externalID
	}
	message.Payload = json.RawMessage(payloadJSON)
	message.Metadata = json.RawMessage(metadataJSON)
	return &message, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Dialog Event operations

func (r *PgRepository) CreateDialogEvent(ctx context.Context, event *DialogEvent) error {
	query := `
		INSERT INTO dialog_events (id, dialog_id, event_type, actor_id, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.pool.Exec(ctx, query,
		event.ID,
		event.DialogID,
		event.EventType,
		event.ActorID,
		event.Payload,
		event.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create dialog event: %w", err)
	}

	return nil
}

func (r *PgRepository) GetDialogEvents(ctx context.Context, dialogID uuid.UUID) ([]DialogEvent, error) {
	query := `
		SELECT id, dialog_id, event_type, actor_id, payload, created_at
		FROM dialog_events
		WHERE dialog_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.pool.Query(ctx, query, dialogID)
	if err != nil {
		return nil, fmt.Errorf("failed to get dialog events: %w", err)
	}
	defer rows.Close()

	events := []DialogEvent{}
	for rows.Next() {
		var event DialogEvent
		var payloadJSON []byte

		if err := rows.Scan(
			&event.ID,
			&event.DialogID,
			&event.EventType,
			&event.ActorID,
			&payloadJSON,
			&event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan dialog event: %w", err)
		}

		event.Payload = json.RawMessage(payloadJSON)
		events = append(events, event)
	}

	return events, nil
}
