// Package badger provides a BadgerDB-backed Store implementation for OpenSQS queues,
// using dgraph-io/badger/v4 with iterator-based scanning and lazy visibility timeout evaluation.
package badger

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// BadgerDB is a wrapper around badger.DB that provides shared access
// across multiple queue stores. Each queue uses a key prefix to isolate
// its data within the same BadgerDB instance.
type BadgerDB struct {
	db *badger.DB
}

// Open creates or opens a BadgerDB instance at the given directory path.
func Open(path string) (*BadgerDB, error) {
	opts := badger.DefaultOptions(path).
		WithLogger(nil). // Silence Badger's internal logging
		WithCompression(0)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB at %s: %w", path, err)
	}

	return &BadgerDB{db: db}, nil
}

// Close releases the underlying BadgerDB resources.
func (b *BadgerDB) Close() error {
	return b.db.Close()
}

// BadgerStore is a BadgerDB-backed implementation of the Store interface.
// It uses lazy visibility timeout evaluation (no goroutines/timers).
// Messages become visible when their visible_at timestamp is checked
// on the next ReceiveMessages call.
type BadgerStore struct {
	mu                sync.Mutex
	db                *badger.DB
	queueName         string
	keyPrefix         []byte
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

// storedMessage is the serialized form of a message in BadgerDB.
type storedMessage struct {
	ID              string                                  `json:"id"`
	Body            string                                  `json:"body"`
	MD5OfBody       string                                  `json:"md5_of_body"`
	MD5OfMsgAttrs   string                                  `json:"md5_of_msg_attrs"`
	MessageAttrs    map[string]types.MessageAttribute       `json:"message_attrs"`
	SystemAttrs     map[string]types.MessageSystemAttribute `json:"system_attrs"`
	SentTimestamp   int64                                   `json:"sent_ts"`
	VisibleAt       int64                                   `json:"visible_at"`
	ReceiptHandle   string                                  `json:"receipt_handle"`
	ReceiveCount    int                                     `json:"receive_count"`
	FirstReceivedAt int64                                   `json:"first_received_at"`
	SequenceNumber  string                                  `json:"sequence_number"`
	DedupID         string                                  `json:"dedup_id"`
	GroupID         string                                  `json:"group_id"`
}

// NewBadgerStore creates a new BadgerDB-backed message store.
// The db parameter should be an already-open *BadgerDB connection.
// Each queue uses a key prefix to isolate its data.
// The queue name is validated to prevent prefix collisions.
func NewBadgerStore(db *BadgerDB, queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) (*BadgerStore, error) {
	// Validate queue name to prevent key prefix collisions
	// (e.g., a queue named "q:foo" would collide with prefix "q:q:foo:")
	for _, c := range queueName {
		if c == ':' {
			return nil, fmt.Errorf("invalid queue name %q: contains ':'", queueName)
		}
	}

	s := &BadgerStore{
		db:                        db.db,
		queueName:                 queueName,
		keyPrefix:                 []byte("q:" + queueName + ":"),
		visibilityTimeout:         visibilityTimeout,
		serverSecret:              serverSecret,
		isFifo:                    cfg.IsFifo,
		contentBasedDeduplication: cfg.ContentBasedDeduplication,
		dedupCache:                make(map[string]time.Time),
		maxReceiveCount:           cfg.MaxReceiveCount,
		redriveFunc:               cfg.RedriveFunc,
	}

	return s, nil
}

// msgKey returns the BadgerDB key for a message ID.
func (s *BadgerStore) msgKey(messageID string) []byte {
	return append(s.keyPrefix, []byte(messageID)...)
}

// SendMessage adds a message to the queue with an optional delay.
func (s *BadgerStore) SendMessage(ctx context.Context, msg *types.Message, delaySeconds int) error {
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

	sm := storedMessage{
		ID:             msg.MessageID,
		Body:           msg.Body,
		MD5OfBody:      msg.MD5OfBody,
		MD5OfMsgAttrs:  msg.MD5OfMessageAttributes,
		MessageAttrs:   msg.MessageAttributes,
		SystemAttrs:    msg.SystemAttributes,
		SentTimestamp:  now.UnixMilli(),
		VisibleAt:      visibleAt.UnixMilli(),
		ReceiptHandle:  "",
		ReceiveCount:   0,
		SequenceNumber: msg.SequenceNumber,
		DedupID:        msg.MessageDeduplicationID,
		GroupID:        msg.MessageGroupID,
	}

	data, err := json.Marshal(sm)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(s.msgKey(msg.MessageID), data)
	})
	if err != nil {
		return fmt.Errorf("failed to store message: %w", err)
	}

	return nil
}

// ReceiveMessages retrieves up to maxMessages visible messages.
// Long polling is implemented via a polling loop with short sleeps.
func (s *BadgerStore) ReceiveMessages(ctx context.Context, maxMessages int, visibilityTimeout int, waitTimeSeconds int) ([]*types.Message, error) {
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
		nowMilli := now.UnixMilli()

		// Collect all visible messages
		type candidate struct {
			key           []byte
			msg           *types.Message
			sm            storedMessage
			receiveCount  int
			firstReceived time.Time
		}
		var candidates []candidate
		inFlightGroups := make(map[string]bool)

		err := s.db.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.Prefix = s.keyPrefix
			it := txn.NewIterator(opts)
			defer it.Close()

			for it.Seek(s.keyPrefix); it.ValidForPrefix(s.keyPrefix); it.Next() {
				item := it.Item()
				var sm storedMessage
				err := item.Value(func(val []byte) error {
					return json.Unmarshal(val, &sm)
				})
				if err != nil {
					continue
				}

				// Skip messages not yet visible
				if sm.VisibleAt > nowMilli {
					// FIFO: if this message is in-flight (has receipt handle and visible in future),
					// mark its group as in-flight so we skip other messages in the same group
					if s.isFifo && sm.GroupID != "" && sm.ReceiptHandle != "" {
						inFlightGroups[sm.GroupID] = true
					}
					continue
				}

				// FIFO: skip if this group already has an in-flight message
				if s.isFifo && sm.GroupID != "" {
					if inFlightGroups[sm.GroupID] {
						continue
					}
					// Check if this message is already in-flight (shouldn't be visible, but check anyway)
					if sm.ReceiptHandle != "" {
						inFlightGroups[sm.GroupID] = true
						continue
					}
					inFlightGroups[sm.GroupID] = true
				}

				msg := s.storedToMessage(sm)
				candidates = append(candidates, candidate{
					key:           append([]byte{}, item.Key()...),
					msg:           msg,
					sm:            sm,
					receiveCount:  sm.ReceiveCount,
					firstReceived: time.UnixMilli(sm.FirstReceivedAt),
				})

				if len(candidates) >= maxMessages {
					break
				}
			}
			return nil
		})

		if err != nil {
			s.mu.Unlock()
			return nil, fmt.Errorf("failed to query messages: %w", err)
		}

		// Sort candidates by sent timestamp for FIFO ordering
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].sm.SentTimestamp < candidates[j].sm.SentTimestamp
		})

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

		err = s.db.Update(func(txn *badger.Txn) error {
			for _, c := range candidates {
				newReceiveCount := c.receiveCount + 1
				firstReceived := c.firstReceived
				if firstReceived.IsZero() {
					firstReceived = now
				}

				receiptHandle := s.generateReceiptHandle(c.msg.MessageID, now)

				c.sm.ReceiptHandle = receiptHandle
				c.sm.VisibleAt = newVisibleAt.UnixMilli()
				c.sm.ReceiveCount = newReceiveCount
				c.sm.FirstReceivedAt = firstReceived.UnixMilli()

				data, err := json.Marshal(c.sm)
				if err != nil {
					continue
				}

				err = txn.Set(c.key, data)
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
			return nil
		})

		s.mu.Unlock()

		if err != nil {
			return nil, fmt.Errorf("failed to update messages: %w", err)
		}

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
// Uses a single Update transaction to avoid TOCTOU race between find and delete.
func (s *BadgerStore) DeleteMessage(ctx context.Context, receiptHandle string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	found := false

	err := s.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = s.keyPrefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(s.keyPrefix); it.ValidForPrefix(s.keyPrefix); it.Next() {
			item := it.Item()
			var sm storedMessage
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &sm)
			})
			if err != nil {
				continue
			}

			if sm.ReceiptHandle == receiptHandle {
				found = true
				return txn.Delete(item.KeyCopy(nil))
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	if !found {
		return types.NewReceiptHandleIsInvalid(fmt.Sprintf("Receipt handle %s is invalid", receiptHandle))
	}

	return nil
}

// ChangeMessageVisibility updates the visibility timeout of a message.
// Uses a single Update transaction to avoid TOCTOU race between find and update.
func (s *BadgerStore) ChangeMessageVisibility(ctx context.Context, receiptHandle string, visibilityTimeout int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	now := store.Now()
	found := false

	err := s.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = s.keyPrefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(s.keyPrefix); it.ValidForPrefix(s.keyPrefix); it.Next() {
			item := it.Item()
			var sm storedMessage
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &sm)
			})
			if err != nil {
				continue
			}

			if sm.ReceiptHandle == receiptHandle {
				found = true
				if visibilityTimeout == 0 {
					// Immediately make visible
					sm.VisibleAt = now.UnixMilli()
					sm.ReceiptHandle = ""
				} else {
					newVisibleAt := now.Add(time.Duration(visibilityTimeout) * time.Second)
					sm.VisibleAt = newVisibleAt.UnixMilli()
				}

				data, err := json.Marshal(sm)
				if err != nil {
					return fmt.Errorf("failed to marshal message: %w", err)
				}
				return txn.Set(item.KeyCopy(nil), data)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to change message visibility: %w", err)
	}

	if !found {
		return types.NewReceiptHandleIsInvalid(fmt.Sprintf("Receipt handle %s is invalid", receiptHandle))
	}

	return nil
}

// ApproximateNumberOfMessages returns the count of visible messages.
func (s *BadgerStore) ApproximateNumberOfMessages() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	nowMilli := store.Now().UnixMilli()
	count := 0

	if err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = s.keyPrefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(s.keyPrefix); it.ValidForPrefix(s.keyPrefix); it.Next() {
			item := it.Item()
			var sm storedMessage
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &sm)
			})
			if err != nil {
				continue
			}
			if sm.VisibleAt <= nowMilli {
				count++
			}
		}
		return nil
	}); err != nil {
		return 0
	}

	return count
}

// ApproximateNumberOfMessagesNotVisible returns the count of in-flight messages.
func (s *BadgerStore) ApproximateNumberOfMessagesNotVisible() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	nowMilli := store.Now().UnixMilli()
	count := 0

	if err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = s.keyPrefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(s.keyPrefix); it.ValidForPrefix(s.keyPrefix); it.Next() {
			item := it.Item()
			var sm storedMessage
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &sm)
			})
			if err != nil {
				continue
			}
			if sm.VisibleAt > nowMilli && sm.ReceiptHandle != "" {
				count++
			}
		}
		return nil
	}); err != nil {
		return 0
	}

	return count
}

// ApproximateNumberOfMessagesDelayed returns the count of delayed messages.
func (s *BadgerStore) ApproximateNumberOfMessagesDelayed() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	nowMilli := store.Now().UnixMilli()
	count := 0

	if err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = s.keyPrefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(s.keyPrefix); it.ValidForPrefix(s.keyPrefix); it.Next() {
			item := it.Item()
			var sm storedMessage
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &sm)
			})
			if err != nil {
				continue
			}
			if sm.VisibleAt > nowMilli && sm.ReceiptHandle == "" {
				count++
			}
		}
		return nil
	}); err != nil {
		return 0
	}

	return count
}

// Purge removes all messages from the store.
// Uses a single Update transaction to avoid non-atomic read-then-delete.
func (s *BadgerStore) Purge(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	err := s.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = s.keyPrefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(s.keyPrefix); it.ValidForPrefix(s.keyPrefix); it.Next() {
			if err := txn.Delete(it.Item().KeyCopy(nil)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to purge queue: %w", err)
	}

	s.dedupCache = make(map[string]time.Time)
	return nil
}

// Close releases resources.
func (s *BadgerStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	// Note: the *badger.DB is shared across queues, so we don't close it here.
	// The caller (factory) is responsible for closing the DB connection.
	return nil
}

// storedToMessage converts a storedMessage to a types.Message.
func (s *BadgerStore) storedToMessage(sm storedMessage) *types.Message {
	return &types.Message{
		MessageID:              sm.ID,
		Body:                   sm.Body,
		MD5OfBody:              sm.MD5OfBody,
		MD5OfMessageAttributes: sm.MD5OfMsgAttrs,
		MessageAttributes:      sm.MessageAttrs,
		SystemAttributes:       sm.SystemAttrs,
		SentTimestamp:          time.UnixMilli(sm.SentTimestamp),
		SequenceNumber:         sm.SequenceNumber,
		MessageDeduplicationID: sm.DedupID,
		MessageGroupID:         sm.GroupID,
	}
}

// generateReceiptHandle creates a signed, base64-encoded receipt handle.
func (s *BadgerStore) generateReceiptHandle(messageID string, now time.Time) string {
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
func (s *BadgerStore) cleanExpiredDedupEntries() {
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
// exceeds maxReceiveCount.
// The caller must hold the mutex.
func (s *BadgerStore) redriveIfNeededLocked(ctx context.Context) {
	if s.maxReceiveCount <= 0 || s.redriveFunc == nil {
		return
	}

	nowMilli := store.Now().UnixMilli()

	type redriveCandidate struct {
		key []byte
		msg *types.Message
	}

	var toRedrive []redriveCandidate

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = s.keyPrefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(s.keyPrefix); it.ValidForPrefix(s.keyPrefix); it.Next() {
			item := it.Item()
			var sm storedMessage
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &sm)
			})
			if err != nil {
				continue
			}

			// Messages that have become visible again (visibility expired) and
			// whose receive count exceeds maxReceiveCount
			if sm.VisibleAt <= nowMilli && sm.ReceiptHandle != "" && sm.ReceiveCount >= s.maxReceiveCount {
				msg := s.storedToMessage(sm)
				toRedrive = append(toRedrive, redriveCandidate{
					key: append([]byte{}, item.Key()...),
					msg: msg,
				})
			}
		}
		return nil
	})
	if err != nil {
		return
	}

	if len(toRedrive) == 0 {
		return
	}

	// Redrive messages and delete from this queue in a single transaction
	err = s.db.Update(func(txn *badger.Txn) error {
		for _, rc := range toRedrive {
			rc.msg.ReceiptHandle = ""
			rc.msg.IsVisible = true
			rc.msg.ApproximateReceiveCount = 0
			s.redriveFunc(rc.msg)
			txn.Delete(rc.key)
		}
		return nil
	})
	_ = err // redrive errors are non-fatal; messages will be retried on next ReceiveMessages
}
