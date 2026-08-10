package handlers

import (
	"context"
	"fmt"

	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// handleTagQueue handles the TagQueue action.
func (h *Handler) handleTagQueue(ctx context.Context, req Request) (*Response, error) {
	q, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	tags := req.GetTags()

	// AWS limit: maximum MaxQueueTags tags per queue
	currentTags := q.Tags()
	if len(currentTags)+len(tags) > types.MaxQueueTags {
		return nil, queue.NewInvalidParameterValue(fmt.Sprintf("Maximum number of tags per queue is %d.", types.MaxQueueTags))
	}

	for key, value := range tags {
		// AWS limits: tag key max MaxTagKeyLength chars, value max MaxTagValueLength chars
		if len(key) > types.MaxTagKeyLength {
			return nil, queue.NewInvalidParameterValue(fmt.Sprintf("Tag key too long (max %d): %s.", types.MaxTagKeyLength, key))
		}
		if len(value) > types.MaxTagValueLength {
			return nil, queue.NewInvalidParameterValue(fmt.Sprintf("Tag value too long (max %d): %s.", types.MaxTagValueLength, value))
		}
	}

	// Atomically merge new tags into existing tags to avoid get-modify-set race.
	q.UpdateTags(func(existing map[string]string) map[string]string {
		for key, value := range tags {
			existing[key] = value
		}
		return existing
	})

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

	// Atomically remove tags to avoid get-modify-set race.
	q.UpdateTags(func(existing map[string]string) map[string]string {
		for _, key := range tagKeys {
			delete(existing, key)
		}
		return existing
	})

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
