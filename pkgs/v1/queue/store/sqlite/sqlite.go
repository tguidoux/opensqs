package sqlite

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
	_ "modernc.org/sqlite"
)

// SQLiteStore is a SQLite-backed implementation of the Store interface.
// It uses lazy visibility timeout evaluation (no goroutines/timers).
// Messages become visible when their visible_at timestamp is checked
// on the next ReceiveMessages call.
type SQLiteStore struct {
	mu                sync.Mutex
	db                *sql.DB
	queueName         string
	visibilityTimeout int
	serverSecret      []byte
	closed            bool

	// FIFO support
	isFifo                    bool
	contentBasedDeduplication bool
	dedupCache                map[string]time.Time
	sequenceCounter           int64

	// DLQ support
	maxReceiveCount int
	redriveFunc     store.RedriveFunc
}

// NewSQLiteStore creates a new SQLite-backed message store.
// The db parameter should be an already-open *sql.DB connection.
// Each queue uses its own table namespace based on the queue name.
func NewSQLiteStore(db *sql.DB, queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) (*SQLiteStore, error) {
	s := &SQLiteStore{
		db:                        db,
		queueName:                 queueName,
		visibilityTimeout:         visibilityTimeout,
		serverSecret:              serverSecret,
		isFifo:                    cfg.IsFifo,
		contentBasedDeduplication: cfg.ContentBasedDeduplication,
		dedupCache:                make(map[string]time.Time),
		maxReceiveCount:           cfg.MaxReceiveCount,
		redriveFunc:               cfg.RedriveFunc,
	}

	if err := s.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize SQLite schema: %w", err)
	}

	return s, nil
}

// initSchema creates the messages table if it doesn't exist.
func (s *SQLiteStore) initSchema() error {
	tableName := s.tableName()
	_, err := s.db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			queue_name TEXT NOT NULL,
			body TEXT NOT NULL,
			md5_of_body TEXT NOT NULL,
			md5_of_message_attributes TEXT NOT NULL DEFAULT '',
			message_attributes TEXT NOT NULL DEFAULT '{}',
			system_attributes TEXT NOT NULL DEFAULT '{}',
			sent_timestamp INTEGER NOT NULL,
			visible_at INTEGER NOT NULL,
			receipt_handle TEXT NOT NULL DEFAULT '',
			receive_count INTEGER NOT NULL DEFAULT 0,
			first_received_at INTEGER NOT NULL DEFAULT 0,
			sequence_number TEXT NOT NULL DEFAULT '',
			message_dedup_id TEXT NOT NULL DEFAULT '',
			message_group_id TEXT NOT NULL DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS idx_%s_visible_at ON %s (queue_name, visible_at);
		CREATE INDEX IF NOT EXISTS idx_%s_receipt_handle ON %s (receipt_handle);
	`, tableName, tableName, tableName, tableName, tableName))
	return err
}

// tableName returns the sanitized table name for this queue.
func (s *SQLiteStore) tableName() string {
	// Sanitize queue name for use as a table name
	name := s.queueName
	for i := 0; i < len(name); i++ {
		c := name[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			name = name[:i] + "_" + name[i+1:]
		}
	}
	return "queue_" + name
}

// SendMessage adds a message to the queue with an optional delay.
func (s *SQLiteStore) SendMessage(ctx context.Context, msg *types.Message, delaySeconds int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	// FIFO queue: deduplication and sequence number assignment
	if s.isFifo {
		s.cleanExpiredDedupEntries()

		dedupID := msg.MessageDeduplicationID
		if dedupID == "" && s.contentBasedDeduplication {
			dedupID = computeContentBasedDedupID(msg.Body)
		}

		if dedupID != "" {
			if _, exists := s.dedupCache[dedupID]; exists {
				// Duplicate within dedup window — silently accept
				return nil
			}
			s.dedupCache[dedupID] = store.Now().Add(5 * time.Minute)
		}

		s.sequenceCounter++
		msg.SequenceNumber = fmt.Sprintf("%d", s.sequenceCounter)
	}

	now := store.Now()
	visibleAt := now
	if delaySeconds > 0 {
		visibleAt = now.Add(time.Duration(delaySeconds) * time.Second)
	}

	msg.SentTimestamp = now
	msg.IsVisible = delaySeconds == 0

	// Serialize message attributes to JSON
	attrsJSON, _ := json.Marshal(msg.MessageAttributes)
	sysAttrsJSON, _ := json.Marshal(msg.SystemAttributes)

	tableName := s.tableName()
	_, err := s.db.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (id, queue_name, body, md5_of_body, md5_of_message_attributes, message_attributes, system_attributes, sent_timestamp, visible_at, receipt_handle, receive_count, first_received_at, sequence_number, message_dedup_id, message_group_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, tableName),
		msg.MessageID, s.queueName, msg.Body, msg.MD5OfBody, msg.MD5OfMessageAttributes,
		string(attrsJSON), string(sysAttrsJSON),
		now.UnixMilli(), visibleAt.UnixMilli(),
		"", 0, 0,
		msg.SequenceNumber, msg.MessageDeduplicationID, msg.MessageGroupID,
	)

	return err
}

// ReceiveMessages retrieves up to maxMessages visible messages.
// Long polling is implemented via a polling loop with short sleeps.
func (s *SQLiteStore) ReceiveMessages(ctx context.Context, maxMessages int, visibilityTimeout int, waitTimeSeconds int) ([]*types.Message, error) {
	if visibilityTimeout <= 0 {
		visibilityTimeout = s.visibilityTimeout
	}

	deadline := time.Time{}
	if waitTimeSeconds > 0 {
		deadline = store.Now().Add(time.Duration(waitTimeSeconds) * time.Second)
	}

	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return nil, fmt.Errorf("store is closed")
		}

		// Check for messages that need to be redrived to a DLQ
		s.redriveIfNeededLocked(ctx)

		now := store.Now()
		tableName := s.tableName()

		// Select visible messages (visible_at <= now), ordered by sent_timestamp
		// For FIFO: need to handle one in-flight per message group
		// Note: messages whose visibility timeout expired still have an old receipt_handle,
		// so we only check visible_at, not receipt_handle.
		query := fmt.Sprintf(`
			SELECT id, body, md5_of_body, md5_of_message_attributes, message_attributes, system_attributes,
			       sent_timestamp, receive_count, first_received_at, sequence_number, message_dedup_id, message_group_id
			FROM %s
			WHERE visible_at <= ?
			ORDER BY sent_timestamp ASC
			LIMIT ?
		`, tableName)

		rows, err := s.db.QueryContext(ctx, query, now.UnixMilli(), maxMessages)
		if err != nil {
			s.mu.Unlock()
			return nil, fmt.Errorf("failed to query messages: %w", err)
		}
		defer rows.Close()

		type candidate struct {
			msg           *types.Message
			receiveCount  int
			firstReceived time.Time
		}
		var candidates []candidate
		// Track which groups are already in-flight (for FIFO)
		inFlightGroups := make(map[string]bool)

		for rows.Next() {
			var (
				id, body, md5OfBody, md5OfMsgAttrs, msgAttrsJSON, sysAttrsJSON string
				sentTs, receiveCount, firstReceived                            int64
				sequenceNumber, dedupID, groupID                               string
			)

			if err := rows.Scan(&id, &body, &md5OfBody, &md5OfMsgAttrs, &msgAttrsJSON, &sysAttrsJSON,
				&sentTs, &receiveCount, &firstReceived, &sequenceNumber, &dedupID, &groupID); err != nil {
				rows.Close()
				s.mu.Unlock()
				return nil, fmt.Errorf("failed to scan message: %w", err)
			}

			// FIFO: skip if this group already has an in-flight message
			if s.isFifo && groupID != "" {
				if inFlightGroups[groupID] {
					continue
				}
				// Check if any message in this group is already in-flight (visible_at > now)
				var inFlight int
				s.db.QueryRowContext(ctx, fmt.Sprintf(
					`SELECT COUNT(*) FROM %s WHERE message_group_id = ? AND receipt_handle != '' AND visible_at > ?`, tableName,
				), groupID, now.UnixMilli()).Scan(&inFlight)
				if inFlight > 0 {
					inFlightGroups[groupID] = true
					continue
				}
				inFlightGroups[groupID] = true
			}

			msg := &types.Message{
				MessageID:              id,
				Body:                   body,
				MD5OfBody:              md5OfBody,
				MD5OfMessageAttributes: md5OfMsgAttrs,
				SentTimestamp:          time.UnixMilli(sentTs),
				SequenceNumber:         sequenceNumber,
				MessageDeduplicationID: dedupID,
				MessageGroupID:         groupID,
			}

			// Deserialize message attributes
			if msgAttrsJSON != "" && msgAttrsJSON != "{}" {
				var attrs map[string]types.MessageAttribute
				if json.Unmarshal([]byte(msgAttrsJSON), &attrs) == nil {
					msg.MessageAttributes = attrs
				}
			}

			// Deserialize system attributes
			if sysAttrsJSON != "" && sysAttrsJSON != "{}" {
				var sysAttrs map[string]types.MessageSystemAttribute
				if json.Unmarshal([]byte(sysAttrsJSON), &sysAttrs) == nil {
					msg.SystemAttributes = sysAttrs
				}
			}

			candidates = append(candidates, candidate{
				msg:           msg,
				receiveCount:  int(receiveCount),
				firstReceived: time.UnixMilli(firstReceived),
			})
		}

		if len(candidates) == 0 {
			// No messages available
			if waitTimeSeconds <= 0 {
				s.mu.Unlock()
				return []*types.Message{}, nil
			}

			s.mu.Unlock()
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return []*types.Message{}, nil
			}

			// Poll every 200ms
			sleepDur := 200 * time.Millisecond
			if remaining < sleepDur {
				sleepDur = remaining
			}
			select {
			case <-time.After(sleepDur):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Process candidates: update visibility, assign receipt handles
		var result []*types.Message
		newVisibleAt := now.Add(time.Duration(visibilityTimeout) * time.Second)

		for _, c := range candidates {
			newReceiveCount := c.receiveCount + 1
			firstReceived := c.firstReceived
			if firstReceived.IsZero() {
				firstReceived = now
			}

			receiptHandle := s.generateReceiptHandle(c.msg.MessageID, now)

			// Update the row with new receipt handle and visibility
			_, err := s.db.ExecContext(ctx, fmt.Sprintf(`
				UPDATE %s SET receipt_handle = ?, visible_at = ?, receive_count = ?, first_received_at = ?
				WHERE id = ?
			`, tableName), receiptHandle, newVisibleAt.UnixMilli(), newReceiveCount, firstReceived.UnixMilli(), c.msg.MessageID)
			if err != nil {
				continue
			}

			c.msg.ReceiptHandle = receiptHandle
			c.msg.IsVisible = false
			c.msg.ApproximateReceiveCount = newReceiveCount
			c.msg.ReceivedTimestamp = now
			c.msg.ApproximateFirstReceiveTimestamp = firstReceived

			result = append(result, c.msg)

			if len(result) >= maxMessages {
				break
			}
		}

		s.mu.Unlock()

		if len(result) > 0 {
			return result, nil
		}

		// All candidates were claimed by another receiver — retry
		if waitTimeSeconds <= 0 {
			return []*types.Message{}, nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return []*types.Message{}, nil
		}
		select {
		case <-time.After(100 * time.Millisecond):
			continue
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// DeleteMessage removes a message by receipt handle.
func (s *SQLiteStore) DeleteMessage(ctx context.Context, receiptHandle string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	tableName := s.tableName()
	result, err := s.db.ExecContext(ctx, fmt.Sprintf(
		`DELETE FROM %s WHERE receipt_handle = ?`, tableName,
	), receiptHandle)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return types.NewReceiptHandleIsInvalid(fmt.Sprintf("Receipt handle %s is invalid", receiptHandle))
	}

	return nil
}

// ChangeMessageVisibility updates the visibility timeout of a message.
func (s *SQLiteStore) ChangeMessageVisibility(ctx context.Context, receiptHandle string, visibilityTimeout int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	now := store.Now()
	tableName := s.tableName()

	if visibilityTimeout == 0 {
		// Immediately make visible
		result, err := s.db.ExecContext(ctx, fmt.Sprintf(`
			UPDATE %s SET visible_at = ?, receipt_handle = ''
			WHERE receipt_handle = ?
		`, tableName), now.UnixMilli(), receiptHandle)
		if err != nil {
			return fmt.Errorf("failed to change message visibility: %w", err)
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			return types.NewReceiptHandleIsInvalid(fmt.Sprintf("Receipt handle %s is invalid", receiptHandle))
		}
		return nil
	}

	newVisibleAt := now.Add(time.Duration(visibilityTimeout) * time.Second)
	result, err := s.db.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s SET visible_at = ?
		WHERE receipt_handle = ?
	`, tableName), newVisibleAt.UnixMilli(), receiptHandle)
	if err != nil {
		return fmt.Errorf("failed to change message visibility: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return types.NewReceiptHandleIsInvalid(fmt.Sprintf("Receipt handle %s is invalid", receiptHandle))
	}

	return nil
}

// ApproximateNumberOfMessages returns the count of visible messages.
func (s *SQLiteStore) ApproximateNumberOfMessages() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := store.Now().UnixMilli()
	tableName := s.tableName()
	var count int
	if err := s.db.QueryRow(fmt.Sprintf(
		`SELECT COUNT(*) FROM %s WHERE visible_at <= ?`, tableName,
	), now).Scan(&count); err != nil {
		return 0
	}
	return count
}

// ApproximateNumberOfMessagesNotVisible returns the count of in-flight messages.
func (s *SQLiteStore) ApproximateNumberOfMessagesNotVisible() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := store.Now().UnixMilli()
	tableName := s.tableName()
	var count int
	if err := s.db.QueryRow(fmt.Sprintf(
		`SELECT COUNT(*) FROM %s WHERE visible_at > ? AND receipt_handle != ''`, tableName,
	), now).Scan(&count); err != nil {
		return 0
	}
	return count
}

// ApproximateNumberOfMessagesDelayed returns the count of delayed messages.
func (s *SQLiteStore) ApproximateNumberOfMessagesDelayed() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := store.Now().UnixMilli()
	tableName := s.tableName()
	var count int
	if err := s.db.QueryRow(fmt.Sprintf(
		`SELECT COUNT(*) FROM %s WHERE visible_at > ? AND receipt_handle = ''`, tableName,
	), now).Scan(&count); err != nil {
		return 0
	}
	return count
}

// Purge removes all messages from the store.
func (s *SQLiteStore) Purge(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	tableName := s.tableName()
	_, err := s.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s`, tableName))
	if err != nil {
		return fmt.Errorf("failed to purge queue: %w", err)
	}

	s.dedupCache = make(map[string]time.Time)
	return nil
}

// Close releases resources.
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	// Note: the *sql.DB is shared across queues, so we don't close it here.
	// The caller (factory) is responsible for closing the DB connection.
	return nil
}

// generateReceiptHandle creates a signed, base64-encoded receipt handle.
func (s *SQLiteStore) generateReceiptHandle(messageID string, now time.Time) string {
	info := types.ReceiptHandleInfo{
		QueueName:        s.queueName,
		MessageID:        messageID,
		ReceiveTimestamp: now,
		RandomNonce:      generateNonce(),
	}

	data, _ := json.Marshal(info)
	mac := hmac.New(sha256.New, s.serverSecret)
	mac.Write(data)
	signature := mac.Sum(nil)

	handle := map[string]string{
		"data":      base64.StdEncoding.EncodeToString(data),
		"signature": hex.EncodeToString(signature),
	}

	encoded, _ := json.Marshal(handle)
	return base64.StdEncoding.EncodeToString(encoded)
}

// generateNonce creates a random hex string.
func generateNonce() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to time-based nonce if crypto/rand fails
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// cleanExpiredDedupEntries removes dedup cache entries that have exceeded the 5-minute window.
func (s *SQLiteStore) cleanExpiredDedupEntries() {
	now := store.Now()
	for id, expiry := range s.dedupCache {
		if now.After(expiry) {
			delete(s.dedupCache, id)
		}
	}
}

// computeContentBasedDedupID generates a deduplication ID from the message body using SHA-256.
func computeContentBasedDedupID(body string) string {
	h := sha256.Sum256([]byte(body))
	return hex.EncodeToString(h[:])
}

// redriveIfNeededLocked checks if any messages should be redrived to a DLQ.
// This is called lazily during ReceiveMessages when a message's receive count
// exceeds maxReceiveCount. In the SQLite store, redrive happens when a message
// that has exceeded maxReceiveCount becomes visible again (visibility expired).
// The caller must hold the mutex.
func (s *SQLiteStore) redriveIfNeededLocked(ctx context.Context) {
	if s.maxReceiveCount <= 0 || s.redriveFunc == nil {
		return
	}

	now := store.Now()
	tableName := s.tableName()

	// Find messages that have become visible again (visibility expired) and
	// whose receive count exceeds maxReceiveCount
	query := fmt.Sprintf(`
		SELECT id, body, md5_of_body, md5_of_message_attributes, message_attributes, system_attributes,
		       sent_timestamp, receive_count, first_received_at, sequence_number, message_dedup_id, message_group_id
		FROM %s
		WHERE visible_at <= ? AND receipt_handle != '' AND receive_count >= ?
	`, tableName)

	rows, err := s.db.QueryContext(ctx, query, now.UnixMilli(), s.maxReceiveCount)
	if err != nil {
		return
	}
	defer rows.Close()

	var toRedrive []*types.Message
	var idsToDelete []string

	for rows.Next() {
		var (
			id, body, md5OfBody, md5OfMsgAttrs, msgAttrsJSON, sysAttrsJSON string
			sentTs, receiveCount, firstReceived                            int64
			sequenceNumber, dedupID, groupID                               string
		)

		if err := rows.Scan(&id, &body, &md5OfBody, &md5OfMsgAttrs, &msgAttrsJSON, &sysAttrsJSON,
			&sentTs, &receiveCount, &firstReceived, &sequenceNumber, &dedupID, &groupID); err != nil {
			continue
		}

		msg := &types.Message{
			MessageID:              id,
			Body:                   body,
			MD5OfBody:              md5OfBody,
			MD5OfMessageAttributes: md5OfMsgAttrs,
			SentTimestamp:          time.UnixMilli(sentTs),
			SequenceNumber:         sequenceNumber,
			MessageDeduplicationID: dedupID,
			MessageGroupID:         groupID,
		}

		if msgAttrsJSON != "" && msgAttrsJSON != "{}" {
			var attrs map[string]types.MessageAttribute
			if json.Unmarshal([]byte(msgAttrsJSON), &attrs) == nil {
				msg.MessageAttributes = attrs
			}
		}

		if sysAttrsJSON != "" && sysAttrsJSON != "{}" {
			var sysAttrs map[string]types.MessageSystemAttribute
			if json.Unmarshal([]byte(sysAttrsJSON), &sysAttrs) == nil {
				msg.SystemAttributes = sysAttrs
			}
		}

		toRedrive = append(toRedrive, msg)
		idsToDelete = append(idsToDelete, id)
	}

	// Redrive messages and delete from this queue
	for i, msg := range toRedrive {
		msg.ReceiptHandle = ""
		msg.IsVisible = true
		msg.ApproximateReceiveCount = 0
		s.redriveFunc(msg)

		// Delete the redrived message
		s.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, tableName), idsToDelete[i])
	}
}
