package handlers

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/dlq"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// handleCreateQueue handles the CreateQueue action.
func (h *Handler) handleCreateQueue(ctx context.Context, req Request) (*Response, error) {
	name := req.GetQueueName()
	if name == "" {
		return nil, queue.NewMissingParameter("QueueName")
	}

	if err := h.limits.VerifyQueueName(name); err != nil {
		return nil, err
	}

	// Build attributes from request if provided
	attrs := queue.NewDefaultQueueAttributes()
	for name, value := range req.GetAttributes() {
		if err := attrs.SetAttribute(name, value); err != nil {
			return nil, queue.NewInvalidAttributeName(err.Error())
		}
	}

	// Validate FIFO queue name suffix
	if attrs.FifoQueue && !isFifoQueueName(name) {
		return nil, queue.NewInvalidParameterValue(
			"The name of a FIFO queue can only include alphanumeric characters, hyphens, or underscores, must end with .fifo suffix and be at most 80 characters long.",
		)
	}

	q, err := h.manager.CreateQueue(name, attrs)
	if err != nil {
		return nil, err
	}

	return &Response{
		Action:    types.ActionCreateQueue,
		QueueURL:  q.URL(h.manager.NodeAddress(), h.manager.AccountID()),
		RequestID: newRequestID(),
	}, nil
}

// handleDeleteQueue handles the DeleteQueue action.
func (h *Handler) handleDeleteQueue(ctx context.Context, req Request) (*Response, error) {
	queueURL := req.GetQueueURL()
	if queueURL == "" {
		return nil, queue.NewMissingParameter("QueueUrl")
	}

	name := queue.ExtractQueueNameFromURL(queueURL)
	if err := h.manager.DeleteQueue(name); err != nil {
		return nil, err
	}

	return &Response{
		Action:    types.ActionDeleteQueue,
		RequestID: newRequestID(),
	}, nil
}

// handleGetQueueURL handles the GetQueueUrl action.
func (h *Handler) handleGetQueueURL(ctx context.Context, req Request) (*Response, error) {
	name := req.GetQueueName()
	if name == "" {
		return nil, queue.NewMissingParameter("QueueName")
	}

	if _, err := h.manager.LookupQueue(name); err != nil {
		return nil, err
	}

	return &Response{
		Action:    types.ActionGetQueueUrl,
		QueueURL:  h.manager.QueueURL(name),
		RequestID: newRequestID(),
	}, nil
}

// handleListQueues handles the ListQueues action.
func (h *Handler) handleListQueues(ctx context.Context, req Request) (*Response, error) {
	prefix := req.GetPrefix()
	urls := h.manager.ListQueueURLs(prefix)

	return &Response{
		Action:    types.ActionListQueues,
		QueueURLs: urls,
		RequestID: newRequestID(),
	}, nil
}

// handleSendMessage handles the SendMessage action.
func (h *Handler) handleSendMessage(ctx context.Context, req Request) (*Response, error) {
	q, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	body := req.GetMessageBody()
	if body == "" {
		return nil, queue.NewMissingParameter("MessageBody")
	}

	maxSize := q.Attributes().MaximumMessageSize
	if err := h.limits.VerifyMessageSize(body, maxSize); err != nil {
		return nil, err
	}

	delaySeconds := req.GetDelaySeconds()
	if err := h.limits.VerifyDelaySeconds(delaySeconds); err != nil {
		return nil, err
	}

	// Generate message ID and MD5
	msgID := generateMessageID()
	md5OfBody := computeMD5(body)

	// Extract message attributes from request
	msgAttrs := req.GetMessageAttributes()
	var md5OfMsgAttrs string
	if len(msgAttrs) > 0 {
		md5OfMsgAttrs = computeMD5OfMessageAttributes(msgAttrs)
	}

	// Extract message system attributes from request
	sysAttrs := req.GetMessageSystemAttributes()
	var md5OfSysAttrs string
	if len(sysAttrs) > 0 {
		md5OfSysAttrs = computeMD5OfMessageSystemAttributes(sysAttrs)
	}

	msg := &types.Message{
		MessageID:                    msgID,
		Body:                         body,
		MD5OfBody:                    md5OfBody,
		MD5OfMessageAttributes:       md5OfMsgAttrs,
		MessageAttributes:            msgAttrs,
		MD5OfMessageSystemAttributes: md5OfSysAttrs,
		SystemAttributes:             sysAttrs,
		SentTimestamp:                time.Now().UTC(),
	}

	// FIFO queue validation
	if q.Attributes().FifoQueue {
		groupID := req.GetMessageGroupID()
		if groupID == "" {
			return nil, queue.NewMissingParameter("MessageGroupId")
		}
		if err := h.limits.VerifyMessageGroupId(groupID); err != nil {
			return nil, err
		}
		msg.MessageGroupID = groupID

		dedupID := req.GetMessageDeduplicationID()
		if dedupID == "" && !q.Attributes().ContentBasedDeduplication {
			return nil, queue.NewInvalidParameterValue(
				"The queue should either have ContentBasedDeduplication enabled or MessageDeduplicationId provided explicitly",
			)
		}
		if dedupID != "" {
			if err := h.limits.VerifyDeduplicationId(dedupID); err != nil {
				return nil, err
			}
			msg.MessageDeduplicationID = dedupID
		}
	}

	if err := q.Store().SendMessage(ctx, msg, delaySeconds); err != nil {
		return nil, queue.NewInternalError(err.Error())
	}

	if h.metrics != nil {
		h.metrics.IncMessagesSent(q.Name())
	}

	return &Response{
		Action:    types.ActionSendMessage,
		Message:   msg,
		RequestID: newRequestID(),
	}, nil
}

// handleReceiveMessage handles the ReceiveMessage action.
func (h *Handler) handleReceiveMessage(ctx context.Context, req Request) (*Response, error) {
	q, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	maxMessages := req.GetMaxNumberOfMessages()
	if err := h.limits.VerifyMaxNumberOfMessages(maxMessages); err != nil {
		return nil, err
	}

	visibilityTimeout := req.GetVisibilityTimeout()
	if visibilityTimeout < 0 {
		visibilityTimeout = q.Attributes().VisibilityTimeout
	}
	if err := h.limits.VerifyVisibilityTimeout(visibilityTimeout); err != nil {
		return nil, err
	}

	waitTime := req.GetWaitTimeSeconds()
	if waitTime < 0 {
		waitTime = q.Attributes().ReceiveMessageWaitTimeSeconds
	}
	if err := h.limits.VerifyReceiveMessageWaitTime(waitTime); err != nil {
		return nil, err
	}

	msgs, err := q.Store().ReceiveMessages(ctx, maxMessages, visibilityTimeout, waitTime)
	if err != nil {
		return nil, queue.NewInternalError(err.Error())
	}

	// Filter returned attributes based on requested AttributeNames and MessageAttributeNames
	// If "All" is requested (or the list is empty), return everything
	attrNames := req.GetAttributeNames()
	msgAttrNames := req.GetMessageAttributeNames()
	requestAllAttrs := len(attrNames) == 0
	requestAllMsgAttrs := len(msgAttrNames) == 0
	for _, m := range msgs {
		if !requestAllAttrs && !containsString(attrNames, "All") {
			filtered := make(map[string]string)
			for _, name := range attrNames {
				if v, ok := m.Attributes[name]; ok {
					filtered[name] = v
				}
			}
			m.Attributes = filtered
		}
		if !requestAllMsgAttrs && !containsString(msgAttrNames, "All") {
			filtered := make(map[string]types.MessageAttribute)
			for _, name := range msgAttrNames {
				if v, ok := m.MessageAttributes[name]; ok {
					filtered[name] = v
				}
			}
			m.MessageAttributes = filtered
		}
	}

	if h.metrics != nil && len(msgs) > 0 {
		h.metrics.IncMessagesReceived(q.Name(), len(msgs))
	}

	return &Response{
		Action:    types.ActionReceiveMessage,
		Messages:  msgs,
		RequestID: newRequestID(),
	}, nil
}

// handleDeleteMessage handles the DeleteMessage action.
func (h *Handler) handleDeleteMessage(ctx context.Context, req Request) (*Response, error) {
	q, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	receiptHandle := req.GetReceiptHandle()
	if receiptHandle == "" {
		return nil, queue.NewMissingParameter("ReceiptHandle")
	}

	if err := q.Store().DeleteMessage(ctx, receiptHandle); err != nil {
		return nil, err
	}

	if h.metrics != nil {
		h.metrics.IncMessagesDeleted(q.Name())
	}

	return &Response{
		Action:    types.ActionDeleteMessage,
		RequestID: newRequestID(),
	}, nil
}

// handleChangeMessageVisibility handles the ChangeMessageVisibility action.
func (h *Handler) handleChangeMessageVisibility(ctx context.Context, req Request) (*Response, error) {
	q, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	receiptHandle := req.GetReceiptHandle()
	if receiptHandle == "" {
		return nil, queue.NewMissingParameter("ReceiptHandle")
	}

	visibilityTimeout := req.GetVisibilityTimeout()
	if err := h.limits.VerifyVisibilityTimeout(visibilityTimeout); err != nil {
		return nil, err
	}

	if err := q.Store().ChangeMessageVisibility(ctx, receiptHandle, visibilityTimeout); err != nil {
		return nil, err
	}

	return &Response{
		Action:    types.ActionChangeMessageVisibility,
		RequestID: newRequestID(),
	}, nil
}

// handleGetQueueAttributes handles the GetQueueAttributes action.
func (h *Handler) handleGetQueueAttributes(ctx context.Context, req Request) (*Response, error) {
	q, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	attrNames := req.GetAttributeNames()
	attrs := make(map[string]string)

	if len(attrNames) == 0 || containsString(attrNames, "All") || containsString(attrNames, "all") {
		// Return all attributes
		for _, name := range allAttributeNames() {
			if v, ok := q.GetAttribute(name); ok {
				attrs[name] = v
			}
		}
	} else {
		for _, name := range attrNames {
			if v, ok := q.GetAttribute(name); ok {
				attrs[name] = v
			}
		}
	}

	return &Response{
		Action:     types.ActionGetQueueAttributes,
		Attributes: attrs,
		RequestID:  newRequestID(),
	}, nil
}

// handleSetQueueAttributes handles the SetQueueAttributes action.
func (h *Handler) handleSetQueueAttributes(ctx context.Context, req Request) (*Response, error) {
	q, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	attrs := req.GetAttributes()
	// Immutable attributes that cannot be changed after queue creation
	immutableAttrs := []string{
		types.AttributeFifoQueue,
		types.AttributeContentBasedDeduplication,
		types.AttributeDeduplicationScope,
		types.AttributeFifoThroughputLimit,
	}
	for name, value := range attrs {
		// Check immutable attributes
		for _, imm := range immutableAttrs {
			if name == imm {
				current, _ := q.GetAttribute(imm)
				if current != value {
					return nil, queue.NewInvalidAttributeValue(
						fmt.Sprintf("The %s queue attribute cannot be changed after the queue has been created.", imm),
					)
				}
				continue
			}
		}
		if name == types.AttributeFifoQueue {
			continue // Already validated above
		}
		if err := q.Attributes().SetAttribute(name, value); err != nil {
			return nil, queue.NewInvalidAttributeName(err.Error())
		}
	}

	return &Response{
		Action:    types.ActionSetQueueAttributes,
		RequestID: newRequestID(),
	}, nil
}

// handlePurgeQueue handles the PurgeQueue action.
func (h *Handler) handlePurgeQueue(ctx context.Context, req Request) (*Response, error) {
	q, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	if err := h.manager.PurgeQueue(ctx, q.Name()); err != nil {
		return nil, queue.NewInternalError(err.Error())
	}

	return &Response{
		Action:    types.ActionPurgeQueue,
		RequestID: newRequestID(),
	}, nil
}

// ---------------------------------------------------------------------------
// Phase 2: Batch Operations
// ---------------------------------------------------------------------------

// handleSendMessageBatch handles the SendMessageBatch action.
func (h *Handler) handleSendMessageBatch(ctx context.Context, req Request) (*Response, error) {
	q, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	entries := req.GetBatchEntries()
	if len(entries) == 0 {
		return nil, queue.NewMissingParameter("Entries")
	}

	if err := h.limits.VerifyBatchSize(len(entries)); err != nil {
		return nil, err
	}

	// Check for duplicate IDs
	seenIDs := make(map[string]bool)
	for _, entry := range entries {
		if seenIDs[entry.ID] {
			return nil, queue.NewBatchEntryIdsNotDistinct(
				fmt.Sprintf("Batch entry ids must be distinct. Duplicate id: %s", entry.ID),
			)
		}
		seenIDs[entry.ID] = true
	}

	maxSize := q.Attributes().MaximumMessageSize
	isFifo := q.Attributes().FifoQueue

	var results []BatchResult
	var batchErrors []BatchError

	for _, entry := range entries {
		if entry.MessageBody == "" {
			batchErrors = append(batchErrors, BatchError{
				ID:          entry.ID,
				Code:        "MissingParameter",
				Message:     "MessageBody is required.",
				SenderFault: true,
			})
			continue
		}

		if err := h.limits.VerifyMessageSize(entry.MessageBody, maxSize); err != nil {
			batchErrors = append(batchErrors, BatchError{
				ID:          entry.ID,
				Code:        "InvalidParameterValue",
				Message:     err.Error(),
				SenderFault: true,
			})
			continue
		}

		if err := h.limits.VerifyDelaySeconds(entry.DelaySeconds); err != nil {
			batchErrors = append(batchErrors, BatchError{
				ID:          entry.ID,
				Code:        "InvalidParameterValue",
				Message:     err.Error(),
				SenderFault: true,
			})
			continue
		}

		// FIFO validation for batch entries
		if isFifo {
			if entry.MessageGroupID == "" {
				batchErrors = append(batchErrors, BatchError{
					ID:          entry.ID,
					Code:        "MissingParameter",
					Message:     "MessageGroupId is required for FIFO queues.",
					SenderFault: true,
				})
				continue
			}
			if err := h.limits.VerifyMessageGroupId(entry.MessageGroupID); err != nil {
				batchErrors = append(batchErrors, BatchError{
					ID:          entry.ID,
					Code:        "InvalidParameterValue",
					Message:     err.Error(),
					SenderFault: true,
				})
				continue
			}
			if entry.MessageDeduplicationID == "" && !q.Attributes().ContentBasedDeduplication {
				batchErrors = append(batchErrors, BatchError{
					ID:          entry.ID,
					Code:        "InvalidParameterValue",
					Message:     "The queue should either have ContentBasedDeduplication enabled or MessageDeduplicationId provided explicitly",
					SenderFault: true,
				})
				continue
			}
			if entry.MessageDeduplicationID != "" {
				if err := h.limits.VerifyDeduplicationId(entry.MessageDeduplicationID); err != nil {
					batchErrors = append(batchErrors, BatchError{
						ID:          entry.ID,
						Code:        "InvalidParameterValue",
						Message:     err.Error(),
						SenderFault: true,
					})
					continue
				}
			}
		}

		msgID := generateMessageID()
		md5OfBody := computeMD5(entry.MessageBody)

		var md5OfMsgAttrs string
		if len(entry.MessageAttributes) > 0 {
			md5OfMsgAttrs = computeMD5OfMessageAttributes(entry.MessageAttributes)
		}

		// Extract message system attributes from batch entry
		sysAttrs := entry.MessageSystemAttributes
		var md5OfSysAttrs string
		if len(sysAttrs) > 0 {
			md5OfSysAttrs = computeMD5OfMessageSystemAttributes(sysAttrs)
		}

		msg := &types.Message{
			MessageID:                    msgID,
			Body:                         entry.MessageBody,
			MD5OfBody:                    md5OfBody,
			MD5OfMessageAttributes:       md5OfMsgAttrs,
			MessageAttributes:            entry.MessageAttributes,
			MD5OfMessageSystemAttributes: md5OfSysAttrs,
			SystemAttributes:             sysAttrs,
			SentTimestamp:                time.Now().UTC(),
		}

		if isFifo {
			msg.MessageGroupID = entry.MessageGroupID
			msg.MessageDeduplicationID = entry.MessageDeduplicationID
		}

		if err := q.Store().SendMessage(ctx, msg, entry.DelaySeconds); err != nil {
			batchErrors = append(batchErrors, BatchError{
				ID:          entry.ID,
				Code:        "InternalError",
				Message:     err.Error(),
				SenderFault: false,
			})
			continue
		}

		results = append(results, BatchResult{
			ID:                           entry.ID,
			MessageID:                    msgID,
			MD5OfBody:                    md5OfBody,
			MD5OfMessageAttributes:       md5OfMsgAttrs,
			MD5OfMessageSystemAttributes: md5OfSysAttrs,
			SequenceNumber:               msg.SequenceNumber,
		})
	}

	return &Response{
		Action:       types.ActionSendMessageBatch,
		BatchResults: results,
		BatchErrors:  batchErrors,
		RequestID:    newRequestID(),
	}, nil
}

// handleDeleteMessageBatch handles the DeleteMessageBatch action.
func (h *Handler) handleDeleteMessageBatch(ctx context.Context, req Request) (*Response, error) {
	q, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	entries := req.GetBatchEntries()
	if len(entries) == 0 {
		return nil, queue.NewMissingParameter("Entries")
	}

	if err := h.limits.VerifyBatchSize(len(entries)); err != nil {
		return nil, err
	}

	// Check for duplicate IDs
	seenIDs := make(map[string]bool)
	for _, entry := range entries {
		if seenIDs[entry.ID] {
			return nil, queue.NewBatchEntryIdsNotDistinct(
				fmt.Sprintf("Batch entry ids must be distinct. Duplicate id: %s", entry.ID),
			)
		}
		seenIDs[entry.ID] = true
	}

	var results []BatchResult
	var batchErrors []BatchError

	for _, entry := range entries {
		if entry.ReceiptHandle == "" {
			batchErrors = append(batchErrors, BatchError{
				ID:          entry.ID,
				Code:        "MissingParameter",
				Message:     "ReceiptHandle is required.",
				SenderFault: true,
			})
			continue
		}

		if err := q.Store().DeleteMessage(ctx, entry.ReceiptHandle); err != nil {
			batchErrors = append(batchErrors, BatchError{
				ID:          entry.ID,
				Code:        "ReceiptHandleIsInvalid",
				Message:     err.Error(),
				SenderFault: true,
			})
			continue
		}

		results = append(results, BatchResult{ID: entry.ID})
	}

	return &Response{
		Action:       types.ActionDeleteMessageBatch,
		BatchResults: results,
		BatchErrors:  batchErrors,
		RequestID:    newRequestID(),
	}, nil
}

// handleChangeMessageVisibilityBatch handles the ChangeMessageVisibilityBatch action.
func (h *Handler) handleChangeMessageVisibilityBatch(ctx context.Context, req Request) (*Response, error) {
	q, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	entries := req.GetBatchEntries()
	if len(entries) == 0 {
		return nil, queue.NewMissingParameter("Entries")
	}

	if err := h.limits.VerifyBatchSize(len(entries)); err != nil {
		return nil, err
	}

	// Check for duplicate IDs
	seenIDs := make(map[string]bool)
	for _, entry := range entries {
		if seenIDs[entry.ID] {
			return nil, queue.NewBatchEntryIdsNotDistinct(
				fmt.Sprintf("Batch entry ids must be distinct. Duplicate id: %s", entry.ID),
			)
		}
		seenIDs[entry.ID] = true
	}

	var results []BatchResult
	var batchErrors []BatchError

	for _, entry := range entries {
		if entry.ReceiptHandle == "" {
			batchErrors = append(batchErrors, BatchError{
				ID:          entry.ID,
				Code:        "MissingParameter",
				Message:     "ReceiptHandle is required.",
				SenderFault: true,
			})
			continue
		}

		visibilityTimeout := entry.VisibilityTimeout
		if visibilityTimeout < 0 {
			visibilityTimeout = q.Attributes().VisibilityTimeout
		}
		if err := h.limits.VerifyVisibilityTimeout(visibilityTimeout); err != nil {
			batchErrors = append(batchErrors, BatchError{
				ID:          entry.ID,
				Code:        "InvalidParameterValue",
				Message:     err.Error(),
				SenderFault: true,
			})
			continue
		}

		if err := q.Store().ChangeMessageVisibility(ctx, entry.ReceiptHandle, visibilityTimeout); err != nil {
			batchErrors = append(batchErrors, BatchError{
				ID:          entry.ID,
				Code:        "ReceiptHandleIsInvalid",
				Message:     err.Error(),
				SenderFault: true,
			})
			continue
		}

		results = append(results, BatchResult{ID: entry.ID})
	}

	return &Response{
		Action:       types.ActionChangeMessageVisibilityBatch,
		BatchResults: results,
		BatchErrors:  batchErrors,
		RequestID:    newRequestID(),
	}, nil
}

// ---------------------------------------------------------------------------
// Phase 2: Queue Tagging
// ---------------------------------------------------------------------------

// handleTagQueue handles the TagQueue action.
func (h *Handler) handleTagQueue(ctx context.Context, req Request) (*Response, error) {
	q, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	tags := req.GetTags()

	// AWS limit: maximum 50 tags per queue
	currentTags := q.Tags()
	if len(currentTags)+len(tags) > 50 {
		return nil, queue.NewInvalidParameterValue("Maximum number of tags per queue is 50")
	}

	for key, value := range tags {
		// AWS limits: tag key max 128 chars, value max 256 chars
		if len(key) > 128 {
			return nil, queue.NewInvalidParameterValue(fmt.Sprintf("Tag key too long (max 128): %s", key))
		}
		if len(value) > 256 {
			return nil, queue.NewInvalidParameterValue(fmt.Sprintf("Tag value too long (max 256): %s", key))
		}
		currentTags[key] = value
	}
	q.SetTags(currentTags)

	return &Response{
		Action:    types.ActionTagQueue,
		RequestID: newRequestID(),
	}, nil
}

// handleUntagQueue handles the UntagQueue action.
func (h *Handler) handleUntagQueue(ctx context.Context, req Request) (*Response, error) {
	q, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	tagKeys := req.GetTagKeys()
	currentTags := q.Tags()
	for _, key := range tagKeys {
		delete(currentTags, key)
	}
	q.SetTags(currentTags)

	return &Response{
		Action:    types.ActionUntagQueue,
		RequestID: newRequestID(),
	}, nil
}

// handleListQueueTags handles the ListQueueTags action.
func (h *Handler) handleListQueueTags(ctx context.Context, req Request) (*Response, error) {
	q, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	return &Response{
		Action:    types.ActionListQueueTags,
		Tags:      q.Tags(),
		RequestID: newRequestID(),
	}, nil
}

// ---------------------------------------------------------------------------
// Phase 2: Permission Stubs
// ---------------------------------------------------------------------------

// handleAddPermission handles the AddPermission action.
// This is a stub — accepts the request and returns success without enforcing.
func (h *Handler) handleAddPermission(ctx context.Context, req Request) (*Response, error) {
	_, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	return &Response{
		Action:    types.ActionAddPermission,
		RequestID: newRequestID(),
	}, nil
}

// handleRemovePermission handles the RemovePermission action.
// This is a stub — accepts the request and returns success without enforcing.
func (h *Handler) handleRemovePermission(ctx context.Context, req Request) (*Response, error) {
	_, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	return &Response{
		Action:    types.ActionRemovePermission,
		RequestID: newRequestID(),
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// computeMD5 returns the hex-encoded MD5 hash of the input string.
func computeMD5(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// computeMD5OfMessageAttributes computes the MD5 hash of message attributes
// using the AWS SQS algorithm: sorted attribute names, each encoded as
// name + dataType + (stringValue or binaryValue as base64).
func computeMD5OfMessageAttributes(attrs map[string]types.MessageAttribute) string {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := md5.New()
	for _, name := range keys {
		attr := attrs[name]
		h.Write([]byte(name))
		h.Write([]byte(attr.DataType))
		if attr.StringValue != "" {
			h.Write([]byte(attr.StringValue))
		}
		if len(attr.BinaryValue) > 0 {
			h.Write([]byte(base64.StdEncoding.EncodeToString(attr.BinaryValue)))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// computeMD5OfMessageSystemAttributes computes the MD5 hash of message system attributes.
// Uses the same algorithm as computeMD5OfMessageAttributes.
func computeMD5OfMessageSystemAttributes(attrs map[string]types.MessageSystemAttribute) string {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := md5.New()
	for _, name := range keys {
		attr := attrs[name]
		h.Write([]byte(name))
		h.Write([]byte(attr.DataType))
		if attr.StringValue != "" {
			h.Write([]byte(attr.StringValue))
		}
		if len(attr.BinaryValue) > 0 {
			h.Write([]byte(base64.StdEncoding.EncodeToString(attr.BinaryValue)))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// generateMessageID generates a unique message ID using crypto/rand.
func generateMessageID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp if crypto/rand fails
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	// Format as UUID v4 style
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// newRequestID generates a unique request ID.
func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return types.EmptyRequestID
	}
	// Format as UUID v4
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// containsString checks if a slice contains a string.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// allAttributeNames returns all SQS attribute names.
func allAttributeNames() []string {
	return []string{
		types.AttributeApproximateNumberOfMessages,
		types.AttributeApproximateNumberOfMessagesNotVisible,
		types.AttributeApproximateNumberOfMessagesDelayed,
		types.AttributeDelaySeconds,
		types.AttributeMaximumMessageSize,
		types.AttributeMessageRetentionPeriod,
		types.AttributeQueueArn,
		types.AttributeReceiveMessageWaitTimeSeconds,
		types.AttributeVisibilityTimeout,
		types.AttributeFifoQueue,
		types.AttributeContentBasedDeduplication,
		types.AttributeSqsManagedSseEnabled,
	}
}

// isFifoQueueName returns true if the queue name ends with ".fifo".
func isFifoQueueName(name string) bool {
	return len(name) >= 5 && name[len(name)-5:] == ".fifo"
}

// handleListDeadLetterSourceQueues handles the ListDeadLetterSourceQueues action.
// It returns the URLs of all queues that have a RedrivePolicy pointing to the specified DLQ.
func (h *Handler) handleListDeadLetterSourceQueues(ctx context.Context, req Request) (*Response, error) {
	dlqQueue, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	dlqArn := dlqQueue.Attributes().QueueArn

	// Iterate all queues and find those whose RedrivePolicy targets this DLQ
	allQueues := h.manager.ListQueues("")
	var sourceURLs []string
	for _, q := range allQueues {
		rpStr := q.Attributes().RedrivePolicy
		if rpStr == "" {
			continue
		}
		rp, err := dlq.ParseRedrivePolicy(rpStr)
		if err != nil {
			continue
		}
		if rp.DeadLetterTargetArn == dlqArn {
			sourceURLs = append(sourceURLs, q.URL(h.manager.NodeAddress(), h.manager.AccountID()))
		}
	}

	return &Response{
		Action:    types.ActionListDeadLetterSourceQueues,
		QueueURLs: sourceURLs,
		RequestID: newRequestID(),
	}, nil
}

// handleStartMessageMoveTask handles the StartMessageMoveTask action.
// It starts a background task that moves messages from a source queue (typically a DLQ)
// to a destination queue. If DestinationArn is empty, messages are moved back to the
// queue that originally redrived to the DLQ.
func (h *Handler) handleStartMessageMoveTask(ctx context.Context, req Request) (*Response, error) {
	sourceArn := req.GetSourceArn()
	if sourceArn == "" {
		return nil, queue.NewMissingParameter("SourceArn")
	}

	task, err := h.moveTaskMgr.StartTask(sourceArn, req.GetDestinationArn(), req.GetMaxNumberOfMessagesPerSecond())
	if err != nil {
		return nil, queue.NewInvalidParameterValue(err.Error())
	}

	return &Response{
		Action: types.ActionStartMessageMoveTask,
		MoveTaskResult: &MoveTaskResult{
			TaskHandle: task.TaskHandle,
		},
		RequestID: newRequestID(),
	}, nil
}

// handleCancelMessageMoveTask handles the CancelMessageMoveTask action.
// It cancels a running message move task and returns the approximate number
// of messages moved so far.
func (h *Handler) handleCancelMessageMoveTask(ctx context.Context, req Request) (*Response, error) {
	taskHandle := req.GetTaskHandle()
	if taskHandle == "" {
		return nil, queue.NewMissingParameter("TaskHandle")
	}

	moved, err := h.moveTaskMgr.CancelTask(taskHandle)
	if err != nil {
		return nil, queue.NewInvalidParameterValue(err.Error())
	}

	return &Response{
		Action: types.ActionCancelMessageMoveTask,
		MoveTaskResult: &MoveTaskResult{
			MovedMessages: moved,
		},
		RequestID: newRequestID(),
	}, nil
}

// handleListMessageMoveTasks handles the ListMessageMoveTasks action.
// It returns all message move tasks for a given source queue ARN.
func (h *Handler) handleListMessageMoveTasks(ctx context.Context, req Request) (*Response, error) {
	sourceArn := req.GetSourceArn()
	if sourceArn == "" {
		return nil, queue.NewMissingParameter("SourceArn")
	}

	tasks := h.moveTaskMgr.ListTasks(sourceArn)
	results := make([]*MoveTaskResult, 0, len(tasks))
	for _, t := range tasks {
		results = append(results, &MoveTaskResult{
			TaskHandle:                   t.TaskHandle,
			SourceArn:                    t.SourceArn,
			DestinationArn:               t.DestinationArn,
			Status:                       string(t.Status),
			MaxNumberOfMessagesPerSecond: t.MaxNumberOfMessagesPerSecond,
			MovedMessages:                t.MovedMessages(),
		})
	}

	return &Response{
		Action:          types.ActionListMessageMoveTasks,
		MoveTaskResults: results,
		RequestID:       newRequestID(),
	}, nil
}
