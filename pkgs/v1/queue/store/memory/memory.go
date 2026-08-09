package memory

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// MemoryStore is an in-memory implementation of the Store interface.
type MemoryStore struct {
	mu                        sync.Mutex
	messages                  []*memoryMessage
	queueName                 string
	visibilityTimeout         int
	notify                    chan struct{}
	serverSecret              []byte
	closed                    bool
	isFifo                    bool
	contentBasedDeduplication bool
	dedupCache                map[string]time.Time
	messageGroups             map[string][]*memoryMessage
	inFlightGroups            map[string]bool
	sequenceCounter           int64
	maxReceiveCount           int
	redriveFunc               store.RedriveFunc
}

type memoryMessage struct {
	msg             *types.Message
	visibleAt       time.Time
	receiptHandle   string
	receiveCount    int
	firstReceived   time.Time
	visibilityTimer *time.Timer
}

// NewMemoryStore creates a new in-memory message store.
func NewMemoryStore(queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) *MemoryStore {
	return &MemoryStore{
		messages:                  []*memoryMessage{},
		queueName:                 queueName,
		visibilityTimeout:         visibilityTimeout,
		notify:                    make(chan struct{}, 1),
		serverSecret:              serverSecret,
		isFifo:                    cfg.IsFifo,
		contentBasedDeduplication: cfg.ContentBasedDeduplication,
		dedupCache:                make(map[string]time.Time),
		messageGroups:             make(map[string][]*memoryMessage),
		inFlightGroups:            make(map[string]bool),
		maxReceiveCount:           cfg.MaxReceiveCount,
		redriveFunc:               cfg.RedriveFunc,
	}
}

// SendMessage adds a message to the queue with an optional delay.
func (s *MemoryStore) SendMessage(ctx context.Context, msg *types.Message, delaySeconds int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	// FIFO queue: deduplication and sequence number assignment
	if s.isFifo {
		// Clean expired dedup entries
		s.cleanExpiredDedupEntries()

		// Determine deduplication ID
		dedupID := msg.MessageDeduplicationID
		if dedupID == "" && s.contentBasedDeduplication {
			dedupID = computeContentBasedDedupID(msg.Body)
		}

		// Check dedup cache — if seen within 5 minutes, return the existing message ID
		if dedupID != "" {
			if _, exists := s.dedupCache[dedupID]; exists {
				// Duplicate message within dedup window — silently accept but don't enqueue
				// AWS returns the same MessageId and SequenceNumber
				return nil
			}
			// Add to dedup cache with 5-minute expiry
			s.dedupCache[dedupID] = store.Now().Add(5 * time.Minute)
		}

		// Assign sequence number
		s.sequenceCounter++
		msg.SequenceNumber = fmt.Sprintf("%d", s.sequenceCounter)
	}

	visibleAt := store.Now()
	if delaySeconds > 0 {
		visibleAt = visibleAt.Add(time.Duration(delaySeconds) * time.Second)
	}

	msg.SentTimestamp = store.Now()
	msg.IsVisible = delaySeconds == 0

	mm := &memoryMessage{
		msg:          msg,
		visibleAt:    visibleAt,
		receiveCount: 0,
	}

	s.messages = append(s.messages, mm)

	// Track message group for FIFO
	if s.isFifo && msg.MessageGroupID != "" {
		s.messageGroups[msg.MessageGroupID] = append(s.messageGroups[msg.MessageGroupID], mm)
	}

	if delaySeconds == 0 {
		s.notifyWaiters()
	}

	return nil
}

// ReceiveMessages retrieves up to maxMessages visible messages.
func (s *MemoryStore) ReceiveMessages(ctx context.Context, maxMessages int, visibilityTimeout int, waitTimeSeconds int) ([]*types.Message, error) {
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

		now := store.Now()
		var result []*types.Message

		for _, mm := range s.messages {
			if mm.visibleAt.After(now) {
				continue
			}

			// FIFO: only one message per message group can be in-flight at a time
			if s.isFifo && mm.msg.MessageGroupID != "" {
				if s.inFlightGroups[mm.msg.MessageGroupID] {
					continue
				}
			}

			result = append(result, s.receiveMessage(mm, visibilityTimeout, now))
			if len(result) >= maxMessages {
				break
			}
		}

		if len(result) > 0 {
			s.mu.Unlock()
			return result, nil
		}

		// No messages available
		if waitTimeSeconds <= 0 {
			s.mu.Unlock()
			return []*types.Message{}, nil
		}

		// Wait for new messages or timeout
		s.mu.Unlock()

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return []*types.Message{}, nil
		}

		select {
		case <-s.notify:
			continue
		case <-time.After(remaining):
			return []*types.Message{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (s *MemoryStore) receiveMessage(mm *memoryMessage, visibilityTimeout int, now time.Time) *types.Message {
	mm.receiveCount++
	if mm.firstReceived.IsZero() {
		mm.firstReceived = now
	}

	mm.visibleAt = now.Add(time.Duration(visibilityTimeout) * time.Second)
	mm.receiptHandle = s.generateReceiptHandle(mm.msg.MessageID, now)

	// Mark message group as in-flight for FIFO queues
	if s.isFifo && mm.msg.MessageGroupID != "" {
		s.inFlightGroups[mm.msg.MessageGroupID] = true
	}

	// Cancel any existing visibility timer
	if mm.visibilityTimer != nil {
		mm.visibilityTimer.Stop()
	}

	// Set up a timer to make the message visible again (or redrive if DLQ threshold reached)
	mm.visibilityTimer = time.AfterFunc(time.Duration(visibilityTimeout)*time.Second, func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		// Check if message should be redrived to a dead-letter queue
		if s.maxReceiveCount > 0 && mm.receiveCount >= s.maxReceiveCount && s.redriveFunc != nil {
			// Redrive the message — remove from store and send to DLQ
			s.removeMessage(mm)
			s.redriveFunc(mm.msg)
			s.notifyWaiters()
			return
		}

		// Normal visibility timeout expiry — make message visible again
		mm.msg.IsVisible = true
		mm.receiptHandle = ""
		mm.visibilityTimer = nil
		// Clear in-flight group for FIFO queues
		if s.isFifo && mm.msg.MessageGroupID != "" {
			delete(s.inFlightGroups, mm.msg.MessageGroupID)
		}
		s.notifyWaiters()
	})

	mm.msg.IsVisible = false
	mm.msg.ReceiptHandle = mm.receiptHandle
	mm.msg.ApproximateReceiveCount = mm.receiveCount
	mm.msg.ReceivedTimestamp = now
	mm.msg.ApproximateFirstReceiveTimestamp = mm.firstReceived

	return mm.msg
}

// DeleteMessage removes a message by receipt handle.
func (s *MemoryStore) DeleteMessage(ctx context.Context, receiptHandle string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, mm := range s.messages {
		if mm.receiptHandle == receiptHandle && mm.receiptHandle != "" {
			s.removeMessage(mm)
			s.notifyWaiters()
			return nil
		}
	}

	return types.NewReceiptHandleIsInvalid(fmt.Sprintf("Receipt handle %s is invalid", receiptHandle))
}

// ChangeMessageVisibility updates the visibility timeout of a message.
func (s *MemoryStore) ChangeMessageVisibility(ctx context.Context, receiptHandle string, visibilityTimeout int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, mm := range s.messages {
		if mm.receiptHandle == receiptHandle && mm.receiptHandle != "" {
			if mm.visibilityTimer != nil {
				mm.visibilityTimer.Stop()
			}

			if visibilityTimeout == 0 {
				// Immediately make visible
				mm.visibleAt = store.Now()
				mm.receiptHandle = ""
				mm.msg.IsVisible = true
				mm.visibilityTimer = nil
				// Clear in-flight group for FIFO queues
				if s.isFifo && mm.msg.MessageGroupID != "" {
					delete(s.inFlightGroups, mm.msg.MessageGroupID)
				}
				s.notifyWaiters()
			} else {
				mm.visibleAt = store.Now().Add(time.Duration(visibilityTimeout) * time.Second)
				mm.visibilityTimer = time.AfterFunc(time.Duration(visibilityTimeout)*time.Second, func() {
					s.mu.Lock()
					defer s.mu.Unlock()

					// Check if message should be redrived to a dead-letter queue
					if s.maxReceiveCount > 0 && mm.receiveCount >= s.maxReceiveCount && s.redriveFunc != nil {
						s.removeMessage(mm)
						s.redriveFunc(mm.msg)
						s.notifyWaiters()
						return
					}

					mm.msg.IsVisible = true
					mm.receiptHandle = ""
					mm.visibilityTimer = nil
					// Clear in-flight group for FIFO queues
					if s.isFifo && mm.msg.MessageGroupID != "" {
						delete(s.inFlightGroups, mm.msg.MessageGroupID)
					}
					s.notifyWaiters()
				})
			}
			return nil
		}
	}

	return types.NewReceiptHandleIsInvalid(fmt.Sprintf("Receipt handle %s is invalid", receiptHandle))
}

// ApproximateNumberOfMessages returns the count of visible messages.
func (s *MemoryStore) ApproximateNumberOfMessages() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := store.Now()
	count := 0
	for _, mm := range s.messages {
		if !mm.visibleAt.After(now) {
			count++
		}
	}
	return count
}

// ApproximateNumberOfMessagesNotVisible returns the count of in-flight messages.
func (s *MemoryStore) ApproximateNumberOfMessagesNotVisible() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := store.Now()
	count := 0
	for _, mm := range s.messages {
		if mm.visibleAt.After(now) && mm.receiptHandle != "" {
			count++
		}
	}
	return count
}

// ApproximateNumberOfMessagesDelayed returns the count of delayed messages.
func (s *MemoryStore) ApproximateNumberOfMessagesDelayed() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := store.Now()
	count := 0
	for _, mm := range s.messages {
		if mm.visibleAt.After(now) && mm.receiptHandle == "" {
			count++
		}
	}
	return count
}

// Purge removes all messages from the store.
func (s *MemoryStore) Purge(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, mm := range s.messages {
		if mm.visibilityTimer != nil {
			mm.visibilityTimer.Stop()
		}
	}

	s.messages = []*memoryMessage{}
	s.dedupCache = make(map[string]time.Time)
	s.messageGroups = make(map[string][]*memoryMessage)
	s.inFlightGroups = make(map[string]bool)
	s.notifyWaiters()
	return nil
}

// Close releases resources.
func (s *MemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	for _, mm := range s.messages {
		if mm.visibilityTimer != nil {
			mm.visibilityTimer.Stop()
		}
	}

	return nil
}

func (s *MemoryStore) notifyWaiters() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// generateReceiptHandle creates a signed, base64-encoded receipt handle.
func (s *MemoryStore) generateReceiptHandle(messageID string, now time.Time) string {
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
	rand.Read(b)
	return hex.EncodeToString(b)
}

// cleanExpiredDedupEntries removes dedup cache entries that have exceeded the 5-minute window.
func (s *MemoryStore) cleanExpiredDedupEntries() {
	now := store.Now()
	for id, expiry := range s.dedupCache {
		if now.After(expiry) {
			delete(s.dedupCache, id)
		}
	}
}

// computeContentBasedDedupID generates a deduplication ID from the message body using MD5.
func computeContentBasedDedupID(body string) string {
	h := sha256.Sum256([]byte(body))
	return hex.EncodeToString(h[:])
}

// removeFromMessageGroup removes a message from its group tracking slice.
func (s *MemoryStore) removeFromMessageGroup(mm *memoryMessage) {
	groupID := mm.msg.MessageGroupID
	group := s.messageGroups[groupID]
	for i, gmm := range group {
		if gmm == mm {
			s.messageGroups[groupID] = append(group[:i], group[i+1:]...)
			break
		}
	}
	if len(s.messageGroups[groupID]) == 0 {
		delete(s.messageGroups, groupID)
	}
}

// removeMessage removes a message from the store's internal slice and group tracking.
// The caller must hold the mutex.
func (s *MemoryStore) removeMessage(mm *memoryMessage) {
	if mm.visibilityTimer != nil {
		mm.visibilityTimer.Stop()
	}
	// Clear in-flight group for FIFO queues
	if s.isFifo && mm.msg.MessageGroupID != "" {
		delete(s.inFlightGroups, mm.msg.MessageGroupID)
		s.removeFromMessageGroup(mm)
	}
	for i, m := range s.messages {
		if m == mm {
			s.messages = append(s.messages[:i], s.messages[i+1:]...)
			break
		}
	}
}
