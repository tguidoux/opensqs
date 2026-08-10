package handlers

import (
	"context"
	"slices"
	"time"

	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

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

	// FIFO queue validation — validate before constructing the message
	// to avoid wasted work if validation fails.
	var groupID, dedupID string
	if q.Attributes().FifoQueue {
		groupID = req.GetMessageGroupID()
		if groupID == "" {
			return nil, queue.NewMissingParameter("MessageGroupId")
		}
		if err := h.limits.VerifyMessageGroupId(groupID); err != nil {
			return nil, err
		}

		dedupID = req.GetMessageDeduplicationID()
		if dedupID == "" && !q.Attributes().ContentBasedDeduplication {
			return nil, queue.NewInvalidParameterValue(fifoDedupRequiredMsg)
		}
		if dedupID != "" {
			if err := h.limits.VerifyDeduplicationId(dedupID); err != nil {
				return nil, err
			}
		}
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
		MessageGroupID:               groupID,
		MessageDeduplicationID:       dedupID,
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
		if !requestAllAttrs && !slices.Contains(attrNames, AllAttributes) {
			filtered := make(map[string]string)
			for _, name := range attrNames {
				if v, ok := m.Attributes[name]; ok {
					filtered[name] = v
				}
			}
			m.Attributes = filtered
		}
		if !requestAllMsgAttrs && !slices.Contains(msgAttrNames, AllAttributes) {
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
		return nil, queue.NewInternalError(err.Error())
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
		return nil, queue.NewInternalError(err.Error())
	}

	return &Response{
		Action:    types.ActionChangeMessageVisibility,
		RequestID: newRequestID(),
	}, nil
}
