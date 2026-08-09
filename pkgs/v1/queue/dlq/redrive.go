package dlq

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// RedrivePolicy represents the SQS RedrivePolicy queue attribute.
// It specifies the dead-letter queue (DLQ) ARN and the maximum number
// of receive attempts before a message is redrived.
type RedrivePolicy struct {
	DeadLetterTargetArn string `json:"deadLetterTargetArn"`
	MaxReceiveCount     int    `json:"maxReceiveCount"`
}

// ParseRedrivePolicy parses a RedrivePolicy JSON string.
// AWS encodes MaxReceiveCount as a string-encoded integer in the JSON.
// Example: {"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:my-dlq","maxReceiveCount":"5"}
func ParseRedrivePolicy(jsonStr string) (*RedrivePolicy, error) {
	if jsonStr == "" {
		return nil, fmt.Errorf("empty redrive policy")
	}

	// Use json.Number to handle both string and number formats for maxReceiveCount
	var raw struct {
		DeadLetterTargetArn string      `json:"deadLetterTargetArn"`
		MaxReceiveCount     json.Number `json:"maxReceiveCount"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("invalid redrive policy JSON: %w", err)
	}

	if raw.DeadLetterTargetArn == "" {
		return nil, fmt.Errorf("redrive policy missing deadLetterTargetArn")
	}

	count, err := raw.MaxReceiveCount.Int64()
	if err != nil {
		return nil, fmt.Errorf("invalid maxReceiveCount in redrive policy: %w", err)
	}

	return &RedrivePolicy{
		DeadLetterTargetArn: raw.DeadLetterTargetArn,
		MaxReceiveCount:     int(count),
	}, nil
}

// String returns the JSON string representation of the RedrivePolicy.
// MaxReceiveCount is encoded as a string to match AWS format.
func (rp *RedrivePolicy) String() string {
	data, _ := json.Marshal(map[string]string{
		"deadLetterTargetArn": rp.DeadLetterTargetArn,
		"maxReceiveCount":     strconv.Itoa(rp.MaxReceiveCount),
	})
	return string(data)
}
