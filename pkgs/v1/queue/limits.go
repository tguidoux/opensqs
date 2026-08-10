package queue

import (
	"fmt"

	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// LimitsMode controls whether limits are enforced strictly or relaxed.
type LimitsMode int

const (
	// StrictMode enforces all SQS limits exactly as AWS does.
	StrictMode LimitsMode = iota
	// RelaxedMode allows exceeding some limits (useful for local development).
	RelaxedMode
)

// Limits enforces SQS message and queue limits.
type Limits struct {
	mode LimitsMode
}

// NewLimits creates a new Limits instance with the given mode.
func NewLimits(mode LimitsMode) *Limits {
	return &Limits{mode: mode}
}

// VerifyMessageSize checks if a message body is within the allowed size.
// In RelaxedMode, the size limit is doubled for local development.
func (l *Limits) VerifyMessageSize(body string, maxMessageSize int) error {
	effectiveMax := maxMessageSize
	if l.mode == RelaxedMode {
		effectiveMax = effectiveMax * 2
		// Guard against integer overflow
		if effectiveMax < maxMessageSize {
			effectiveMax = int(^uint(0) >> 1) // max int
		}
	}
	if len(body) > effectiveMax {
		return NewInvalidParameterValue(
			fmt.Sprintf("Message too long: %d bytes exceeds maximum %d bytes", len(body), effectiveMax),
		)
	}
	return nil
}

// VerifyBatchSize checks if a batch has the allowed number of entries.
func (l *Limits) VerifyBatchSize(entryCount int) error {
	if entryCount > types.MaxBatchEntries {
		return NewTooManyEntriesInBatchRequest(
			fmt.Sprintf("Maximum number of entries per request are %d. You have sent %d", types.MaxBatchEntries, entryCount),
		)
	}
	return nil
}

// VerifyVisibilityTimeout checks if a visibility timeout is within allowed range.
func (l *Limits) VerifyVisibilityTimeout(timeout int) error {
	if timeout < 0 || timeout > types.MaxVisibilityTimeout {
		return NewInvalidParameterValue(
			fmt.Sprintf("VisibilityTimeout must be between 0 and %d, got %d", types.MaxVisibilityTimeout, timeout),
		)
	}
	return nil
}

// VerifyDelaySeconds checks if a delay is within allowed range.
func (l *Limits) VerifyDelaySeconds(delay int) error {
	if delay < 0 || delay > types.MaxDelaySeconds {
		return NewInvalidParameterValue(
			fmt.Sprintf("DelaySeconds must be between 0 and %d, got %d", types.MaxDelaySeconds, delay),
		)
	}
	return nil
}

// VerifyReceiveMessageWaitTime checks if wait time is within allowed range.
func (l *Limits) VerifyReceiveMessageWaitTime(waitTime int) error {
	if waitTime < 0 || waitTime > types.MaxReceiveMessageWaitTime {
		return NewInvalidParameterValue(
			fmt.Sprintf("WaitTimeSeconds must be between 0 and %d, got %d", types.MaxReceiveMessageWaitTime, waitTime),
		)
	}
	return nil
}

// VerifyMaxNumberOfMessages checks if the requested number of messages is valid.
func (l *Limits) VerifyMaxNumberOfMessages(maxMessages int) error {
	if maxMessages < 1 || maxMessages > types.MaxNumberOfMessages {
		return NewInvalidParameterValue(
			fmt.Sprintf("MaxNumberOfMessages must be between 1 and %d, got %d", types.MaxNumberOfMessages, maxMessages),
		)
	}
	return nil
}

// VerifyMessageRetentionPeriod checks if retention period is within allowed range.
func (l *Limits) VerifyMessageRetentionPeriod(retention int) error {
	if retention < types.MinMessageRetentionPeriod || retention > types.MaxMessageRetentionPeriod {
		return NewInvalidParameterValue(
			fmt.Sprintf("MessageRetentionPeriod must be between %d and %d, got %d", types.MinMessageRetentionPeriod, types.MaxMessageRetentionPeriod, retention),
		)
	}
	return nil
}

// VerifyMaximumMessageSize checks if max message size is within allowed range.
func (l *Limits) VerifyMaximumMessageSize(size int) error {
	if size < types.MinMaximumMessageSize || size > types.MaxMaximumMessageSize {
		return NewInvalidParameterValue(
			fmt.Sprintf("MaximumMessageSize must be between %d and %d, got %d", types.MinMaximumMessageSize, types.MaxMaximumMessageSize, size),
		)
	}
	return nil
}

// VerifyQueueName checks if a queue name is valid.
func (l *Limits) VerifyQueueName(name string) error {
	if len(name) == 0 {
		return NewInvalidParameterValue("QueueName must not be empty")
	}
	if len(name) > types.MaxQueueNameLength {
		return NewInvalidParameterValue(fmt.Sprintf("QueueName too long: %d characters, maximum is %d", len(name), types.MaxQueueNameLength))
	}
	// FIFO queues must end with .fifo
	if len(name) >= 5 && name[len(name)-5:] == ".fifo" {
		// Validate the base name (before .fifo) uses valid characters
		base := name[:len(name)-5]
		for _, c := range base {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
				return NewInvalidParameterValue(fmt.Sprintf("Invalid character in QueueName: %c", c))
			}
		}
		return nil
	}
	// Check for valid characters: alphanumeric, hyphens, underscores
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return NewInvalidParameterValue(fmt.Sprintf("Invalid character in QueueName: %c", c))
		}
	}
	return nil
}

// VerifyDeduplicationId checks if a deduplication ID is within the allowed size.
func (l *Limits) VerifyDeduplicationId(id string) error {
	if len(id) > types.MaxDeduplicationIdLength {
		return NewInvalidParameterValue(
			fmt.Sprintf("MessageDeduplicationId too long: %d characters, maximum is %d", len(id), types.MaxDeduplicationIdLength),
		)
	}
	return nil
}

// VerifyMessageGroupId checks if a message group ID is within the allowed size.
func (l *Limits) VerifyMessageGroupId(id string) error {
	if len(id) > types.MaxMessageGroupIdLength {
		return NewInvalidParameterValue(
			fmt.Sprintf("MessageGroupId too long: %d characters, maximum is %d", len(id), types.MaxMessageGroupIdLength),
		)
	}
	return nil
}
