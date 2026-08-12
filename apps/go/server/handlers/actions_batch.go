package handlers

import (
	"context"
	"time"

	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// Batch error codes used in batch responses.
const (
	batchErrCodeMissingParameter      = "MissingParameter"
	batchErrCodeInvalidParameterValue = "InvalidParameterValue"
	batchErrCodeInternalError         = "InternalError"
	batchErrCodeReceiptHandleInvalid  = "ReceiptHandleIsInvalid"
)

// fifoDedupRequiredMsg is the shared error message for FIFO queues requiring
// either ContentBasedDeduplication or an explicit MessageDeduplicationId.
const fifoDedupRequiredMsg = "The queue should either have ContentBasedDeduplication enabled or MessageDeduplicationId provided explicitly"

// newBatchError is a convenience helper for constructing BatchError values,
// reducing repetition across batch handlers.
func newBatchError(id, code, msg string, senderFault bool) BatchError {
	return BatchError{
		ID:          id,
		Code:        code,
		Message:     msg,
		SenderFault: senderFault,
	}
}

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

	if err := checkDuplicateBatchIDs(entries); err != nil {
		return nil, err
	}

	maxSize := q.Attributes().MaximumMessageSize
	isFifo := q.Attributes().FifoQueue

	var results []BatchResult
	var batchErrors []BatchError

	for _, entry := range entries {
		if entry.MessageBody == "" {
			batchErrors = append(batchErrors, newBatchError(
				entry.ID, batchErrCodeMissingParameter, "MessageBody is required.", true,
			))
			continue
		}

		if err := h.limits.VerifyMessageSize(entry.MessageBody, maxSize); err != nil {
			batchErrors = append(batchErrors, newBatchError(
				entry.ID, batchErrCodeInvalidParameterValue, err.Error(), true,
			))
			continue
		}

		if err := h.limits.VerifyDelaySeconds(entry.DelaySeconds); err != nil {
			batchErrors = append(batchErrors, newBatchError(
				entry.ID, batchErrCodeInvalidParameterValue, err.Error(), true,
			))
			continue
		}

		// FIFO validation for batch entries
		if isFifo {
			if entry.MessageGroupID == "" {
				batchErrors = append(batchErrors, newBatchError(
					entry.ID, batchErrCodeMissingParameter, "MessageGroupId is required for FIFO queues.", true,
				))
				continue
			}
			if err := h.limits.VerifyMessageGroupId(entry.MessageGroupID); err != nil {
				batchErrors = append(batchErrors, newBatchError(
					entry.ID, batchErrCodeInvalidParameterValue, err.Error(), true,
				))
				continue
			}
			if entry.MessageDeduplicationID == "" && !q.Attributes().ContentBasedDeduplication {
				batchErrors = append(batchErrors, newBatchError(
					entry.ID, batchErrCodeInvalidParameterValue, fifoDedupRequiredMsg, true,
				))
				continue
			}
			if entry.MessageDeduplicationID != "" {
				if err := h.limits.VerifyDeduplicationId(entry.MessageDeduplicationID); err != nil {
					batchErrors = append(batchErrors, newBatchError(
						entry.ID, batchErrCodeInvalidParameterValue, err.Error(), true,
					))
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
			batchErrors = append(batchErrors, newBatchError(
				entry.ID, batchErrCodeInternalError, err.Error(), false,
			))
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

	if err := checkDuplicateBatchIDs(entries); err != nil {
		return nil, err
	}

	var results []BatchResult
	var batchErrors []BatchError

	for _, entry := range entries {
		if entry.ReceiptHandle == "" {
			batchErrors = append(batchErrors, newBatchError(
				entry.ID, batchErrCodeMissingParameter, "ReceiptHandle is required.", true,
			))
			continue
		}

		if err := q.Store().DeleteMessage(ctx, entry.ReceiptHandle); err != nil {
			batchErrors = append(batchErrors, newBatchError(
				entry.ID, batchErrCodeReceiptHandleInvalid, err.Error(), true,
			))
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

	if err := checkDuplicateBatchIDs(entries); err != nil {
		return nil, err
	}

	var results []BatchResult
	var batchErrors []BatchError

	for _, entry := range entries {
		if entry.ReceiptHandle == "" {
			batchErrors = append(batchErrors, newBatchError(
				entry.ID, batchErrCodeMissingParameter, "ReceiptHandle is required.", true,
			))
			continue
		}

		visibilityTimeout := entry.VisibilityTimeout
		if visibilityTimeout < 0 {
			visibilityTimeout = q.Attributes().VisibilityTimeout
		}
		if err := h.limits.VerifyVisibilityTimeout(visibilityTimeout); err != nil {
			batchErrors = append(batchErrors, newBatchError(
				entry.ID, batchErrCodeInvalidParameterValue, err.Error(), true,
			))
			continue
		}

		if err := q.Store().ChangeMessageVisibility(ctx, entry.ReceiptHandle, visibilityTimeout); err != nil {
			batchErrors = append(batchErrors, newBatchError(
				entry.ID, batchErrCodeReceiptHandleInvalid, err.Error(), true,
			))
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
