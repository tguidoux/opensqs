package handlers

import (
	"context"

	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

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
