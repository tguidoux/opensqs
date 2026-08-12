package handlers

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/tguidoux/opensqs/pkgs/v1/id"
	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// AllAttributes is the special attribute name that requests all attributes
// in GetQueueAttributes and ReceiveMessage.
const AllAttributes = "All"

// computeMD5 returns the hex-encoded MD5 hash of the input string.
func computeMD5(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// computeMD5OfAttributes computes the MD5 hash of message attributes (user or system)
// using the AWS SQS algorithm: sorted attribute names, each encoded as
// name + dataType + (stringValue or binaryValue as base64).
// The attrFields function extracts the three fields from the concrete type,
// avoiding the need for duplicate function bodies.
func computeMD5OfAttributes[T any](attrs map[string]T, fields func(T) (dataType, stringValue string, binaryValue []byte)) string {
	if len(attrs) == 0 {
		return ""
	}
	keys := slices.Sorted(maps.Keys(attrs))

	h := md5.New()
	for _, name := range keys {
		dataType, strVal, binVal := fields(attrs[name])
		h.Write([]byte(name))
		h.Write([]byte(dataType))
		if strVal != "" {
			h.Write([]byte(strVal))
		}
		if len(binVal) > 0 {
			h.Write([]byte(base64.StdEncoding.EncodeToString(binVal)))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// computeMD5OfMessageAttributes computes the MD5 hash of user message attributes.
func computeMD5OfMessageAttributes(attrs map[string]types.MessageAttribute) string {
	return computeMD5OfAttributes(attrs, func(a types.MessageAttribute) (string, string, []byte) {
		return a.DataType, a.StringValue, a.BinaryValue
	})
}

// computeMD5OfMessageSystemAttributes computes the MD5 hash of message system attributes.
func computeMD5OfMessageSystemAttributes(attrs map[string]types.MessageSystemAttribute) string {
	return computeMD5OfAttributes(attrs, func(a types.MessageSystemAttribute) (string, string, []byte) {
		return a.DataType, a.StringValue, a.BinaryValue
	})
}

// generateMessageID generates a unique message ID.
func generateMessageID() string {
	return id.NewUUID()
}

// newRequestID generates a unique request ID.
func newRequestID() string {
	return id.NewUUID()
}

// isFifoQueueName returns true if the queue name ends with ".fifo".
func isFifoQueueName(name string) bool {
	return strings.HasSuffix(name, ".fifo")
}

// checkDuplicateBatchIDs validates that all entry IDs in a batch are distinct.
// Returns an error if a duplicate ID is found.
func checkDuplicateBatchIDs(entries []BatchEntry) error {
	seenIDs := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if _, exists := seenIDs[entry.ID]; exists {
			return queue.NewBatchEntryIdsNotDistinct(
				fmt.Sprintf("Batch entry ids must be distinct. Duplicate id: %s.", entry.ID),
			)
		}
		seenIDs[entry.ID] = struct{}{}
	}
	return nil
}

// queryableAttributeNames returns all attribute names that can be returned
// by GetQueueAttributes. This includes both settable attributes (like
// VisibilityTimeout) and computed metrics (like ApproximateNumberOfMessages).
func queryableAttributeNames() []string {
	return []string{
		types.AttributeApproximateNumberOfMessages,
		types.AttributeApproximateNumberOfMessagesNotVisible,
		types.AttributeApproximateNumberOfMessagesDelayed,
		types.AttributeDelaySeconds,
		types.AttributeMaximumMessageSize,
		types.AttributeMessageRetentionPeriod,
		types.AttributeQueueArn,
		types.AttributeReceiveMessageWaitTimeSeconds,
		types.AttributeVisibilityTimeout,
		types.AttributeFifoQueue,
		types.AttributeContentBasedDeduplication,
		types.AttributeSqsManagedSseEnabled,
	}
}
