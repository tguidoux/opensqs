package queue

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/tguidoux/opensqs/pkgs/v1/queue/dlq"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// QueueAttributes holds all SQS queue attributes with defaults and validation.
// All field access should go through GetAttribute/SetAttribute which are
// protected by a mutex for concurrent safety.
type QueueAttributes struct {
	mu                            sync.RWMutex
	VisibilityTimeout             int    `yaml:"visibilityTimeout"`
	DelaySeconds                  int    `yaml:"delaySeconds"`
	MaximumMessageSize            int    `yaml:"maximumMessageSize"`
	MessageRetentionPeriod        int    `yaml:"messageRetentionPeriod"`
	ReceiveMessageWaitTimeSeconds int    `yaml:"receiveMessageWaitTimeSeconds"`
	QueueArn                      string `yaml:"queueArn"`
	Policy                        string `yaml:"policy"`
	RedrivePolicy                 string `yaml:"redrivePolicy"`
	FifoQueue                     bool   `yaml:"fifoQueue"`
	ContentBasedDeduplication     bool   `yaml:"contentBasedDeduplication"`
	KmsMasterKeyId                string `yaml:"kmsMasterKeyId"`
	KmsDataKeyReusePeriodSeconds  int    `yaml:"kmsDataKeyReusePeriodSeconds"`
	DeduplicationScope            string `yaml:"deduplicationScope"`
	FifoThroughputLimit           string `yaml:"fifoThroughputLimit"`
	SqsManagedSseEnabled          bool   `yaml:"sqsManagedSseEnabled"`
}

// NewDefaultQueueAttributes returns attributes initialized with SQS defaults.
func NewDefaultQueueAttributes() *QueueAttributes {
	return &QueueAttributes{
		VisibilityTimeout:             types.DefaultVisibilityTimeout,
		DelaySeconds:                  types.DefaultDelaySeconds,
		MaximumMessageSize:            types.DefaultMaximumMessageSize,
		MessageRetentionPeriod:        types.DefaultMessageRetentionPeriod,
		ReceiveMessageWaitTimeSeconds: types.DefaultReceiveMessageWaitTime,
		SqsManagedSseEnabled:          true,
	}
}

// GetAttribute returns the value of a named attribute as a string.
func (a *QueueAttributes) GetAttribute(name string) (string, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	switch name {
	case types.AttributeVisibilityTimeout:
		return strconv.Itoa(a.VisibilityTimeout), true
	case types.AttributeDelaySeconds:
		return strconv.Itoa(a.DelaySeconds), true
	case types.AttributeMaximumMessageSize:
		return strconv.Itoa(a.MaximumMessageSize), true
	case types.AttributeMessageRetentionPeriod:
		return strconv.Itoa(a.MessageRetentionPeriod), true
	case types.AttributeReceiveMessageWaitTimeSeconds:
		return strconv.Itoa(a.ReceiveMessageWaitTimeSeconds), true
	case types.AttributeQueueArn:
		return a.QueueArn, true
	case types.AttributePolicy:
		return a.Policy, true
	case types.AttributeRedrivePolicy:
		return a.RedrivePolicy, true
	case types.AttributeFifoQueue:
		return strconv.FormatBool(a.FifoQueue), true
	case types.AttributeContentBasedDeduplication:
		return strconv.FormatBool(a.ContentBasedDeduplication), true
	case types.AttributeKmsMasterKeyId:
		return a.KmsMasterKeyId, true
	case types.AttributeKmsDataKeyReusePeriodSeconds:
		return strconv.Itoa(a.KmsDataKeyReusePeriodSeconds), true
	case types.AttributeDeduplicationScope:
		return a.DeduplicationScope, true
	case types.AttributeFifoThroughputLimit:
		return a.FifoThroughputLimit, true
	case types.AttributeSqsManagedSseEnabled:
		return strconv.FormatBool(a.SqsManagedSseEnabled), true
	default:
		return "", false
	}
}

// SetAttribute sets the value of a named attribute from a string.
// Validates numeric ranges per SQS limits.
func (a *QueueAttributes) SetAttribute(name, value string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch name {
	case types.AttributeVisibilityTimeout:
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid VisibilityTimeout: %s", value)
		}
		if v < types.MinVisibilityTimeout || v > types.MaxVisibilityTimeout {
			return fmt.Errorf("VisibilityTimeout must be between %d and %d", types.MinVisibilityTimeout, types.MaxVisibilityTimeout)
		}
		a.VisibilityTimeout = v
	case types.AttributeDelaySeconds:
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid DelaySeconds: %s", value)
		}
		if v < types.MinDelaySeconds || v > types.MaxDelaySeconds {
			return fmt.Errorf("DelaySeconds must be between %d and %d", types.MinDelaySeconds, types.MaxDelaySeconds)
		}
		a.DelaySeconds = v
	case types.AttributeMaximumMessageSize:
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid MaximumMessageSize: %s", value)
		}
		if v < types.MinMaximumMessageSize || v > types.MaxMaximumMessageSize {
			return fmt.Errorf("MaximumMessageSize must be between %d and %d", types.MinMaximumMessageSize, types.MaxMaximumMessageSize)
		}
		a.MaximumMessageSize = v
	case types.AttributeMessageRetentionPeriod:
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid MessageRetentionPeriod: %s", value)
		}
		if v < types.MinMessageRetentionPeriod || v > types.MaxMessageRetentionPeriod {
			return fmt.Errorf("MessageRetentionPeriod must be between %d and %d", types.MinMessageRetentionPeriod, types.MaxMessageRetentionPeriod)
		}
		a.MessageRetentionPeriod = v
	case types.AttributeReceiveMessageWaitTimeSeconds:
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid ReceiveMessageWaitTimeSeconds: %s", value)
		}
		if v < types.MinReceiveMessageWaitTime || v > types.MaxReceiveMessageWaitTime {
			return fmt.Errorf("ReceiveMessageWaitTimeSeconds must be between %d and %d", types.MinReceiveMessageWaitTime, types.MaxReceiveMessageWaitTime)
		}
		a.ReceiveMessageWaitTimeSeconds = v
	case types.AttributeQueueArn:
		a.QueueArn = value
	case types.AttributePolicy:
		a.Policy = value
	case types.AttributeRedrivePolicy:
		// Validate that the RedrivePolicy is valid JSON with required fields
		rp, err := dlq.ParseRedrivePolicy(value)
		if err != nil {
			return fmt.Errorf("invalid RedrivePolicy: %w", err)
		}
		if rp.DeadLetterTargetArn == "" {
			return fmt.Errorf("invalid RedrivePolicy: deadLetterTargetArn is required")
		}
		if rp.MaxReceiveCount < 1 || rp.MaxReceiveCount > 1000 {
			return fmt.Errorf("invalid RedrivePolicy: maxReceiveCount must be between 1 and 1000")
		}
		a.RedrivePolicy = value
	case types.AttributeFifoQueue:
		v, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid FifoQueue: %s", value)
		}
		a.FifoQueue = v
	case types.AttributeContentBasedDeduplication:
		v, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid ContentBasedDeduplication: %s", value)
		}
		a.ContentBasedDeduplication = v
	case types.AttributeKmsMasterKeyId:
		a.KmsMasterKeyId = value
	case types.AttributeKmsDataKeyReusePeriodSeconds:
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid KmsDataKeyReusePeriodSeconds: %s", value)
		}
		if v < types.MinKmsDataKeyReusePeriodSeconds || v > types.MaxKmsDataKeyReusePeriodSeconds {
			return fmt.Errorf("KmsDataKeyReusePeriodSeconds must be between %d and %d", types.MinKmsDataKeyReusePeriodSeconds, types.MaxKmsDataKeyReusePeriodSeconds)
		}
		a.KmsDataKeyReusePeriodSeconds = v
	case types.AttributeDeduplicationScope:
		a.DeduplicationScope = value
	case types.AttributeFifoThroughputLimit:
		a.FifoThroughputLimit = value
	case types.AttributeSqsManagedSseEnabled:
		v, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid SqsManagedSseEnabled: %s", value)
		}
		a.SqsManagedSseEnabled = v
	default:
		return fmt.Errorf("unknown attribute: %s", name)
	}
	return nil
}

// AllAttributes returns all attributes as a map of name to string value.
// Acquires the read lock once for efficiency.
func (a *QueueAttributes) AllAttributes() map[string]string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]string)
	result[types.AttributeVisibilityTimeout] = strconv.Itoa(a.VisibilityTimeout)
	result[types.AttributeDelaySeconds] = strconv.Itoa(a.DelaySeconds)
	result[types.AttributeMaximumMessageSize] = strconv.Itoa(a.MaximumMessageSize)
	result[types.AttributeMessageRetentionPeriod] = strconv.Itoa(a.MessageRetentionPeriod)
	result[types.AttributeReceiveMessageWaitTimeSeconds] = strconv.Itoa(a.ReceiveMessageWaitTimeSeconds)
	result[types.AttributeQueueArn] = a.QueueArn
	result[types.AttributePolicy] = a.Policy
	result[types.AttributeRedrivePolicy] = a.RedrivePolicy
	result[types.AttributeFifoQueue] = strconv.FormatBool(a.FifoQueue)
	result[types.AttributeContentBasedDeduplication] = strconv.FormatBool(a.ContentBasedDeduplication)
	result[types.AttributeKmsMasterKeyId] = a.KmsMasterKeyId
	result[types.AttributeKmsDataKeyReusePeriodSeconds] = strconv.Itoa(a.KmsDataKeyReusePeriodSeconds)
	result[types.AttributeDeduplicationScope] = a.DeduplicationScope
	result[types.AttributeFifoThroughputLimit] = a.FifoThroughputLimit
	result[types.AttributeSqsManagedSseEnabled] = strconv.FormatBool(a.SqsManagedSseEnabled)
	return result
}

// AllAttributeNames returns the list of all settable attribute names.
func AllAttributeNames() []string {
	return []string{
		types.AttributeVisibilityTimeout,
		types.AttributeDelaySeconds,
		types.AttributeMaximumMessageSize,
		types.AttributeMessageRetentionPeriod,
		types.AttributeReceiveMessageWaitTimeSeconds,
		types.AttributeQueueArn,
		types.AttributePolicy,
		types.AttributeRedrivePolicy,
		types.AttributeFifoQueue,
		types.AttributeContentBasedDeduplication,
		types.AttributeKmsMasterKeyId,
		types.AttributeKmsDataKeyReusePeriodSeconds,
		types.AttributeDeduplicationScope,
		types.AttributeFifoThroughputLimit,
		types.AttributeSqsManagedSseEnabled,
	}
}

// GetQueueArn returns the queue ARN.
func (a *QueueAttributes) GetQueueArn() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.QueueArn
}

// GetRedrivePolicy returns the redrive policy JSON string.
func (a *QueueAttributes) GetRedrivePolicy() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.RedrivePolicy
}
