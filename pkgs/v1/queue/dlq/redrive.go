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

	// First try parsing with MaxReceiveCount as a string (AWS format)
	var raw struct {
		DeadLetterTargetArn string `json:"deadLetterTargetArn"`
		MaxReceiveCount     string `json:"maxReceiveCount"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		// Try with MaxReceiveCount as a number
		var rawNum struct {
			DeadLetterTargetArn string `json:"deadLetterTargetArn"`
			MaxReceiveCount     int    `json:"maxReceiveCount"`
		}
		if err2 := json.Unmarshal([]byte(jsonStr), &rawNum); err2 != nil {
			return nil, fmt.Errorf("invalid redrive policy JSON: %w", err)
		}
		return &RedrivePolicy{
			DeadLetterTargetArn: rawNum.DeadLetterTargetArn,
			MaxReceiveCount:     rawNum.MaxReceiveCount,
		}, nil
	}

	count, err := strconv.Atoi(raw.MaxReceiveCount)
	if err != nil {
		// If string parse fails, the count might already be a number
		// Try unmarshalling as a number directly
		var rawNum struct {
			DeadLetterTargetArn string `json:"deadLetterTargetArn"`
			MaxReceiveCount     int    `json:"maxReceiveCount"`
		}
		if err2 := json.Unmarshal([]byte(jsonStr), &rawNum); err2 != nil {
			return nil, fmt.Errorf("invalid maxReceiveCount in redrive policy: %w", err)
		}
		return &RedrivePolicy{
			DeadLetterTargetArn: rawNum.DeadLetterTargetArn,
			MaxReceiveCount:     rawNum.MaxReceiveCount,
		}, nil
	}

	return &RedrivePolicy{
		DeadLetterTargetArn: raw.DeadLetterTargetArn,
		MaxReceiveCount:     count,
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
