package handlers

import (
	"context"
	"fmt"
	"slices"

	"github.com/tguidoux/opensqs/pkgs/v1/queue"
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

// handleGetQueueAttributes handles the GetQueueAttributes action.
func (h *Handler) handleGetQueueAttributes(ctx context.Context, req Request) (*Response, error) {
	q, err := h.resolveQueue(req.GetQueueURL())
	if err != nil {
		return nil, err
	}

	attrNames := req.GetAttributeNames()
	attrs := make(map[string]string)

	if len(attrNames) == 0 || slices.Contains(attrNames, AllAttributes) {
		// Return all attributes
		for _, name := range queryableAttributeNames() {
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
	// Filter out immutable-but-unchanged attributes before batch set
	settable := make(map[string]string, len(attrs))
	for name, value := range attrs {
		// Reject attempts to change immutable attributes to a different value
		if slices.Contains(immutableAttrs, name) {
			current, _ := q.GetAttribute(name)
			if current != value {
				return nil, queue.NewInvalidAttributeValue(
					fmt.Sprintf("The %s queue attribute cannot be changed after the queue has been created.", name),
				)
			}
			continue // Attribute is immutable but unchanged — skip
		}
		settable[name] = value
	}

	// Set all attributes atomically under a single lock to prevent
	// other goroutines from seeing a partially-updated state.
	if err := q.Attributes().SetAttributes(settable); err != nil {
		return nil, queue.NewInvalidAttributeName(err.Error())
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
