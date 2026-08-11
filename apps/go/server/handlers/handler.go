package handlers

import (
	"context"
	"time"

	"github.com/tguidoux/opensqs/apps/go/server/metrics"
	"github.com/tguidoux/opensqs/pkgs/v1/logger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/dlq"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// ProtocolType identifies which wire protocol a request came from.
type ProtocolType int

const (
	// QueryProtocol is the AWS Query Protocol (form-urlencoded, XML responses).
	QueryProtocol ProtocolType = iota
	// JSONProtocol is the AWS JSON Protocol 1.0 (JSON responses).
	JSONProtocol
)

// String returns the string representation of the protocol for metrics labels.
func (p ProtocolType) String() string {
	switch p {
	case QueryProtocol:
		return "query"
	case JSONProtocol:
		return "json"
	default:
		return "unknown"
	}
}

// BatchEntry represents a single entry in a batch request (SendMessageBatch,
// DeleteMessageBatch, ChangeMessageVisibilityBatch).
type BatchEntry struct {
	ID                      string
	MessageBody             string
	DelaySeconds            int
	ReceiptHandle           string
	VisibilityTimeout       int
	MessageAttributes       map[string]types.MessageAttribute
	MessageDeduplicationID  string
	MessageGroupID          string
	MessageSystemAttributes map[string]types.MessageSystemAttribute
}

// BatchResult represents a successful result entry in a batch response.
type BatchResult struct {
	ID                           string
	MessageID                    string
	MD5OfBody                    string
	MD5OfMessageAttributes       string
	MD5OfMessageSystemAttributes string
	SequenceNumber               string
}

// BatchError represents a failed entry in a batch response.
type BatchError struct {
	ID          string
	Code        string
	Message     string
	SenderFault bool
}

// MoveTaskResult represents a single message move task result for responses.
type MoveTaskResult struct {
	TaskHandle                   string
	SourceArn                    string
	DestinationArn               string
	Status                       string
	MaxNumberOfMessagesPerSecond int
	MovedMessages                int
}

// Request is the unified interface that both protocol parsers satisfy.
// The dispatcher uses this to extract parameters regardless of protocol.
type Request interface {
	GetAction() string
	GetQueueURL() string
	GetQueueName() string
	GetMessageBody() string
	GetDelaySeconds() int
	GetVisibilityTimeout() int
	GetMaxNumberOfMessages() int
	GetWaitTimeSeconds() int
	GetReceiptHandle() string
	GetPrefix() string
	GetAttributeNames() []string
	// Phase 2: Message attributes
	GetMessageAttributes() map[string]types.MessageAttribute
	GetMessageAttributeNames() []string
	// Phase 2: Queue attributes (SetQueueAttributes)
	GetAttributes() map[string]string
	// Phase 2: Batch operations
	GetBatchEntries() []BatchEntry
	// Phase 2: Queue tagging
	GetTags() map[string]string
	GetTagKeys() []string
	// Phase 2I: FIFO queues
	GetMessageDeduplicationID() string
	GetMessageGroupID() string
	// Phase 2L: Message system attributes (AWSTraceHeader, etc.)
	GetMessageSystemAttributes() map[string]types.MessageSystemAttribute
	// Phase 4: Message move tasks
	GetSourceArn() string
	GetDestinationArn() string
	GetTaskHandle() string
	GetMaxNumberOfMessagesPerSecond() int
}

// Response is the result of handling an action.
// It contains the data to be marshalled by the protocol layer.
type Response struct {
	// Action that was handled
	Action string
	// QueueURL for CreateQueue/GetQueueUrl responses
	QueueURL string
	// QueueURLs for ListQueues response
	QueueURLs []string
	// Message for SendMessage response
	Message *types.Message
	// Messages for ReceiveMessage response
	Messages []*types.Message
	// Attributes for GetQueueAttributes response
	Attributes map[string]string
	// Tags for ListQueueTags response
	Tags map[string]string
	// BatchResults for batch operation success entries
	BatchResults []BatchResult
	// BatchErrors for batch operation failure entries
	BatchErrors []BatchError
	// RequestID for the response metadata
	RequestID string
	// Phase 4: Message move task result
	MoveTaskResult  *MoveTaskResult
	MoveTaskResults []*MoveTaskResult
}

// Handler is the central dispatcher for SQS actions.
type Handler struct {
	manager     *queue.QueueManager
	limits      *queue.Limits
	autoCreate  bool
	moveTaskMgr *dlq.MoveTaskManager
	metrics     *metrics.Collector
	log         logger.LoggerInterface
}

// NewHandler creates a new Handler with the given queue manager, limits, and auto-create setting.
// If metricsCollector is non-nil, API request metrics are recorded.
func NewHandler(manager *queue.QueueManager, limits *queue.Limits, autoCreate bool, metricsCollector *metrics.Collector, log logger.LoggerInterface) *Handler {
	lookupFn := func(arn string) (dlq.QueueRef, error) {
		return manager.LookupQueueByArn(arn)
	}
	listFn := func(prefix string) []dlq.QueueRef {
		queues := manager.ListQueues(prefix)
		refs := make([]dlq.QueueRef, len(queues))
		for i, q := range queues {
			refs[i] = q
		}
		return refs
	}
	return &Handler{
		manager:     manager,
		limits:      limits,
		autoCreate:  autoCreate,
		moveTaskMgr: dlq.NewMoveTaskManager(lookupFn, listFn),
		metrics:     metricsCollector,
		log:         log,
	}
}

// HandleRequest dispatches a request to the appropriate action handler.
func (h *Handler) HandleRequest(ctx context.Context, req Request, proto ProtocolType) (*Response, error) {
	action := req.GetAction()

	// Record metrics if collector is configured
	if h.metrics != nil {
		start := time.Now()
		defer func() {
			h.metrics.IncAPIRequest(action, proto.String())
			h.metrics.ObserveAPIRequestDuration(action, proto.String(), time.Since(start).Seconds())
		}()
	}

	switch action {
	case types.ActionCreateQueue:
		return h.handleCreateQueue(ctx, req)
	case types.ActionDeleteQueue:
		return h.handleDeleteQueue(ctx, req)
	case types.ActionGetQueueUrl:
		return h.handleGetQueueURL(ctx, req)
	case types.ActionListQueues:
		return h.handleListQueues(ctx, req)
	case types.ActionSendMessage:
		return h.handleSendMessage(ctx, req)
	case types.ActionReceiveMessage:
		return h.handleReceiveMessage(ctx, req)
	case types.ActionDeleteMessage:
		return h.handleDeleteMessage(ctx, req)
	case types.ActionChangeMessageVisibility:
		return h.handleChangeMessageVisibility(ctx, req)
	case types.ActionGetQueueAttributes:
		return h.handleGetQueueAttributes(ctx, req)
	case types.ActionSetQueueAttributes:
		return h.handleSetQueueAttributes(ctx, req)
	case types.ActionPurgeQueue:
		return h.handlePurgeQueue(ctx, req)
	// Phase 2: Batch operations
	case types.ActionSendMessageBatch:
		return h.handleSendMessageBatch(ctx, req)
	case types.ActionDeleteMessageBatch:
		return h.handleDeleteMessageBatch(ctx, req)
	case types.ActionChangeMessageVisibilityBatch:
		return h.handleChangeMessageVisibilityBatch(ctx, req)
	// Phase 2: Queue tagging
	case types.ActionTagQueue:
		return h.handleTagQueue(ctx, req)
	case types.ActionUntagQueue:
		return h.handleUntagQueue(ctx, req)
	case types.ActionListQueueTags:
		return h.handleListQueueTags(ctx, req)
	// Phase 2: Permission stubs
	case types.ActionAddPermission:
		return h.handleAddPermission(ctx, req)
	case types.ActionRemovePermission:
		return h.handleRemovePermission(ctx, req)
	// Phase 2J: Dead-letter queues
	case types.ActionListDeadLetterSourceQueues:
		return h.handleListDeadLetterSourceQueues(ctx, req)
	// Phase 4: Message move tasks
	case types.ActionStartMessageMoveTask:
		return h.handleStartMessageMoveTask(ctx, req)
	case types.ActionCancelMessageMoveTask:
		return h.handleCancelMessageMoveTask(ctx, req)
	case types.ActionListMessageMoveTasks:
		return h.handleListMessageMoveTasks(ctx, req)
	default:
		return nil, queue.NewInvalidAction("The action " + action + " is not valid for this endpoint.")
	}
}

// resolveQueue looks up a queue by URL, returning an error if not found.
// If autoCreate is enabled and the queue doesn't exist, it creates it with default attributes.
func (h *Handler) resolveQueue(queueURL string) (*queue.Queue, error) {
	if queueURL == "" {
		return nil, queue.NewMissingParameter("QueueUrl")
	}
	q, err := h.manager.LookupQueueByURL(queueURL)
	if err != nil {
		if h.autoCreate {
			name := queue.ExtractQueueNameFromURL(queueURL)
			if name != "" {
				attrs := queue.NewDefaultQueueAttributes()
				q, createErr := h.manager.CreateQueue(name, attrs)
				if createErr != nil {
					h.log.Errorf("failed to auto-create queue %q: %v", name, createErr)
				} else {
					return q, nil
				}
			}
		}
		return nil, err
	}
	return q, nil
}
