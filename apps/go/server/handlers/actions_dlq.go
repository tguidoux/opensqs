package handlers

import (
	"context"

	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/dlq"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

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
			h.log.Errorf("failed to parse redrive policy for queue %q: %v", q.Name(), err)
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
			TaskHandle: task.TaskHandle(),
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
			TaskHandle:                   t.TaskHandle(),
			SourceArn:                    t.SourceArn(),
			DestinationArn:               t.DestinationArn(),
			Status:                       string(t.Status()),
			MaxNumberOfMessagesPerSecond: t.MaxNumberOfMessagesPerSecond(),
			MovedMessages:                t.MovedMessages(),
		})
	}

	return &Response{
		Action:          types.ActionListMessageMoveTasks,
		MoveTaskResults: results,
		RequestID:       newRequestID(),
	}, nil
}
