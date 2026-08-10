// Package types defines SQS message types, attribute structures, and API constants
// used throughout the OpenSQS queue engine.
package types

// SQS API version — all responses include this in the XML/JSON envelope.
const SQSVersion = "2012-11-05"

// EmptyRequestID is used when no request ID is available.
const EmptyRequestID = "00000000-0000-0000-0000-000000000000"

// Content types for the two SQS wire protocols.
const (
	// QueryProtocolContentType is the content type for AWS Query Protocol (form-urlencoded).
	QueryProtocolContentType = "application/x-www-form-urlencoded"
	// JSONProtocolContentType is the content type for AWS JSON Protocol 1.0.
	JSONProtocolContentType = "application/x-amz-json-1.0"
	// XMLContentType is the content type for XML responses (Query Protocol).
	XMLContentType = "text/xml"
	// JSONContentType is the content type for JSON responses (JSON Protocol).
	JSONContentType = "application/x-amz-json-1.0"
)

// Action names for all SQS API operations.
const (
	ActionCreateQueue             = "CreateQueue"
	ActionDeleteQueue             = "DeleteQueue"
	ActionGetQueueUrl             = "GetQueueUrl"
	ActionListQueues              = "ListQueues"
	ActionSendMessage             = "SendMessage"
	ActionReceiveMessage          = "ReceiveMessage"
	ActionDeleteMessage           = "DeleteMessage"
	ActionChangeMessageVisibility = "ChangeMessageVisibility"
	ActionGetQueueAttributes      = "GetQueueAttributes"
	ActionSetQueueAttributes      = "SetQueueAttributes"
	ActionPurgeQueue              = "PurgeQueue"
	// Phase 2 actions (not implemented in Phase 1 but defined for completeness).
	ActionSendMessageBatch             = "SendMessageBatch"
	ActionDeleteMessageBatch           = "DeleteMessageBatch"
	ActionChangeMessageVisibilityBatch = "ChangeMessageVisibilityBatch"
	ActionListQueueTags                = "ListQueueTags"
	ActionTagQueue                     = "TagQueue"
	ActionUntagQueue                   = "UntagQueue"
	ActionAddPermission                = "AddPermission"
	ActionRemovePermission             = "RemovePermission"
	ActionListDeadLetterSourceQueues   = "ListDeadLetterSourceQueues"
	ActionGetDeadLetterSourceQueues    = "GetDeadLetterSourceQueues"
	ActionStartMessageMoveTask         = "StartMessageMoveTask"
	ActionCancelMessageMoveTask        = "CancelMessageMoveTask"
	ActionListMessageMoveTasks         = "ListMessageMoveTasks"
)

// AttributeName constants for SQS queue attributes.
const (
	AttributeApproximateNumberOfMessages           = "ApproximateNumberOfMessages"
	AttributeApproximateNumberOfMessagesNotVisible = "ApproximateNumberOfMessagesNotVisible"
	AttributeApproximateNumberOfMessagesDelayed    = "ApproximateNumberOfMessagesDelayed"
	AttributeDelaySeconds                          = "DelaySeconds"
	AttributeMaximumMessageSize                    = "MaximumMessageSize"
	AttributeMessageRetentionPeriod                = "MessageRetentionPeriod"
	AttributePolicy                                = "Policy"
	AttributeQueueArn                              = "QueueArn"
	AttributeReceiveMessageWaitTimeSeconds         = "ReceiveMessageWaitTimeSeconds"
	AttributeRedrivePolicy                         = "RedrivePolicy"
	AttributeVisibilityTimeout                     = "VisibilityTimeout"
	AttributeKmsMasterKeyId                        = "KmsMasterKeyId"
	AttributeKmsDataKeyReusePeriodSeconds          = "KmsDataKeyReusePeriodSeconds"
	AttributeFifoQueue                             = "FifoQueue"
	AttributeContentBasedDeduplication             = "ContentBasedDeduplication"
	AttributeDeduplicationScope                    = "DeduplicationScope"
	AttributeFifoThroughputLimit                   = "FifoThroughputLimit"
	AttributeSqsManagedSseEnabled                  = "SqsManagedSseEnabled"
)

// MessageAttribute data types.
const (
	MessageAttributeTypeString = "String"
	MessageAttributeTypeNumber = "Number"
	MessageAttributeTypeBinary = "Binary"
)

// Default SQS limits.
const (
	DefaultVisibilityTimeout      = 30
	DefaultMessageRetentionPeriod = 345600 // 4 days
	DefaultMaximumMessageSize     = 262144 // 256 KiB
	DefaultDelaySeconds           = 0
	DefaultReceiveMessageWaitTime = 0
)

// Max SQS limits.
const (
	MaxVisibilityTimeout            = 43200   // 12 hours
	MaxMessageRetentionPeriod       = 1209600 // 14 days
	MaxMaximumMessageSize           = 262144  // 256 KiB
	MaxDelaySeconds                 = 900     // 15 minutes
	MaxReceiveMessageWaitTime       = 20
	MaxNumberOfMessages             = 10
	MaxBatchEntries                 = 10
	MaxQueueNameLength              = 80
	MaxDeduplicationIdLength        = 128
	MaxMessageGroupIdLength         = 128
	MaxKmsDataKeyReusePeriodSeconds = 86400 // 24 hours
)

// Min SQS limits.
const (
	MinMessageRetentionPeriod       = 60   // 1 minute
	MinMaximumMessageSize           = 1024 // 1 KiB
	MinVisibilityTimeout            = 0
	MinDelaySeconds                 = 0
	MinReceiveMessageWaitTime       = 0
	MinKmsDataKeyReusePeriodSeconds = 60 // 1 minute
)
