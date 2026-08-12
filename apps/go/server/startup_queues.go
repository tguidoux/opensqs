package main

import (
	"github.com/tguidoux/opensqs/pkgs/v1/queue"
)

// startupAttrsToQueueAttrs converts optional startup config attributes into
// QueueAttributes. Unspecified fields fall back to SQS defaults.
func startupAttrsToQueueAttrs(sa *StartupQueueAttributes) *queue.QueueAttributes {
	attrs := queue.NewDefaultQueueAttributes()
	if sa == nil {
		return attrs
	}
	if sa.VisibilityTimeout != nil {
		attrs.VisibilityTimeout = *sa.VisibilityTimeout
	}
	if sa.DelaySeconds != nil {
		attrs.DelaySeconds = *sa.DelaySeconds
	}
	if sa.MaximumMessageSize != nil {
		attrs.MaximumMessageSize = *sa.MaximumMessageSize
	}
	if sa.MessageRetentionPeriod != nil {
		attrs.MessageRetentionPeriod = *sa.MessageRetentionPeriod
	}
	if sa.ReceiveMessageWaitTimeSeconds != nil {
		attrs.ReceiveMessageWaitTimeSeconds = *sa.ReceiveMessageWaitTimeSeconds
	}
	if sa.FifoQueue != nil {
		attrs.FifoQueue = *sa.FifoQueue
	}
	if sa.ContentBasedDeduplication != nil {
		attrs.ContentBasedDeduplication = *sa.ContentBasedDeduplication
	}
	return attrs
}
