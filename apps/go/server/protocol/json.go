package protocol

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// JSONRequest represents a parsed AWS JSON Protocol 1.0 request.
// The JSON Protocol uses the X-Amz-Target header to specify the action
// and a JSON body for parameters.
type JSONRequest struct {
	Action  string
	Body    map[string]any
	RawBody []byte
}

// ParseJSONRequest parses an AWS JSON Protocol 1.0 request.
// The action is extracted from the X-Amz-Target header (e.g., "AmazonSQS.CreateQueue").
func ParseJSONRequest(targetHeader string, body []byte) (*JSONRequest, error) {
	action := extractActionFromTarget(targetHeader)

	req := &JSONRequest{
		Action:  action,
		RawBody: body,
		Body:    make(map[string]any),
	}

	if len(body) > 0 {
		if err := json.Unmarshal(body, &req.Body); err != nil {
			return nil, fmt.Errorf("failed to parse JSON body: %w", err)
		}
	}

	return req, nil
}

// extractActionFromTarget extracts the action name from the X-Amz-Target header.
// The header format is "AmazonSQS.<Action>" (e.g., "AmazonSQS.CreateQueue").
func extractActionFromTarget(target string) string {
	if target == "" {
		return ""
	}
	// Split on the last dot
	idx := strings.LastIndex(target, ".")
	if idx == -1 {
		return target
	}
	return target[idx+1:]
}

// getString extracts a string field from the JSON body.
func (r *JSONRequest) getString(key string) string {
	v, ok := r.Body[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// getInt extracts an integer field from the JSON body.
func (r *JSONRequest) getInt(key string) int {
	v, ok := r.Body[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}

// GetQueueURL extracts the QueueUrl parameter.
func (r *JSONRequest) GetQueueURL() string {
	return r.getString("QueueUrl")
}

// GetQueueName extracts the QueueName parameter.
func (r *JSONRequest) GetQueueName() string {
	return r.getString("QueueName")
}

// GetMessageBody extracts the MessageBody parameter.
func (r *JSONRequest) GetMessageBody() string {
	return r.getString("MessageBody")
}

// GetDelaySeconds extracts the DelaySeconds parameter, defaulting to 0.
func (r *JSONRequest) GetDelaySeconds() int {
	return r.getInt("DelaySeconds")
}

// GetVisibilityTimeout extracts the VisibilityTimeout parameter, defaulting to -1.
func (r *JSONRequest) GetVisibilityTimeout() int {
	if _, ok := r.Body["VisibilityTimeout"]; !ok {
		return -1
	}
	return r.getInt("VisibilityTimeout")
}

// GetMaxNumberOfMessages extracts MaxNumberOfMessages, defaulting to 1.
func (r *JSONRequest) GetMaxNumberOfMessages() int {
	if _, ok := r.Body["MaxNumberOfMessages"]; !ok {
		return 1
	}
	return r.getInt("MaxNumberOfMessages")
}

// GetWaitTimeSeconds extracts WaitTimeSeconds, defaulting to -1.
func (r *JSONRequest) GetWaitTimeSeconds() int {
	if _, ok := r.Body["WaitTimeSeconds"]; !ok {
		return -1
	}
	return r.getInt("WaitTimeSeconds")
}

// GetReceiptHandle extracts the ReceiptHandle parameter.
func (r *JSONRequest) GetReceiptHandle() string {
	return r.getString("ReceiptHandle")
}

// GetPrefix extracts the Prefix parameter for ListQueues.
func (r *JSONRequest) GetPrefix() string {
	return r.getString("Prefix")
}

// GetAction returns the action name.
func (r *JSONRequest) GetAction() string {
	return r.Action
}

// GetAttributes extracts queue attributes from the JSON body.
// In JSON Protocol, attributes are a map: {"Attributes": {"VisibilityTimeout": "30", ...}}
func (r *JSONRequest) GetAttributes() map[string]string {
	attrs := make(map[string]string)

	attributesRaw, ok := r.Body["Attributes"]
	if !ok {
		return attrs
	}

	attributesMap, ok := attributesRaw.(map[string]any)
	if !ok {
		return attrs
	}

	for k, v := range attributesMap {
		if s, ok := v.(string); ok {
			attrs[k] = s
		}
	}

	return attrs
}

// GetAttributeNames extracts the AttributeNames parameter for GetQueueAttributes.
// In JSON Protocol: {"AttributeNames": ["VisibilityTimeout", "QueueArn"]}
func (r *JSONRequest) GetAttributeNames() []string {
	var names []string

	raw, ok := r.Body["AttributeNames"]
	if !ok {
		return names
	}

	arr, ok := raw.([]any)
	if !ok {
		return names
	}

	for _, v := range arr {
		if s, ok := v.(string); ok {
			names = append(names, s)
		}
	}

	return names
}

// GetBatchEntries extracts batch entries from the JSON body.
// In JSON Protocol, entries are in an array: {"Entries": [{"Id": "...", "MessageBody": "..."}, ...]}
func (r *JSONRequest) GetBatchEntries() []JSONBatchEntry {
	var entries []JSONBatchEntry

	raw, ok := r.Body["Entries"]
	if !ok {
		return entries
	}

	arr, ok := raw.([]any)
	if !ok {
		return entries
	}

	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		entry := JSONBatchEntry{}
		if v, ok := m["Id"].(string); ok {
			entry.ID = v
		}
		if v, ok := m["MessageBody"].(string); ok {
			entry.MessageBody = v
		}
		if v, ok := m["DelaySeconds"].(float64); ok {
			entry.DelaySeconds = int(v)
		}
		if v, ok := m["VisibilityTimeout"].(float64); ok {
			entry.VisibilityTimeout = int(v)
		}
		if v, ok := m["ReceiptHandle"].(string); ok {
			entry.ReceiptHandle = v
		}
		if v, ok := m["MessageDeduplicationId"].(string); ok {
			entry.MessageDeduplicationID = v
		}
		if v, ok := m["MessageGroupId"].(string); ok {
			entry.MessageGroupID = v
		}
		// Parse message system attributes from batch entry
		if sysAttrsRaw, ok := m["MessageSystemAttributes"]; ok {
			if sysAttrsMap, ok := sysAttrsRaw.(map[string]any); ok {
				entry.MessageSystemAttributes = r.parseSystemAttrsFromMap(sysAttrsMap)
			}
		}
		entries = append(entries, entry)
	}

	return entries
}

// GetMessageAttributes extracts message attributes from the JSON body.
// In JSON Protocol: {"MessageAttributes": {"Priority": {"DataType": "Number", "StringValue": "1"}, ...}}
func (r *JSONRequest) GetMessageAttributes() map[string]types.MessageAttribute {
	attrs := make(map[string]types.MessageAttribute)

	raw, ok := r.Body["MessageAttributes"]
	if !ok {
		return attrs
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return attrs
	}

	for name, val := range m {
		attrMap, ok := val.(map[string]any)
		if !ok {
			continue
		}

		attr := types.MessageAttribute{}
		if v, ok := attrMap["DataType"].(string); ok {
			attr.DataType = v
		}
		if v, ok := attrMap["StringValue"].(string); ok {
			attr.StringValue = v
		}
		if v, ok := attrMap["BinaryValue"].(string); ok {
			if decoded, err := base64.StdEncoding.DecodeString(v); err == nil {
				attr.BinaryValue = decoded
			}
		}
		attrs[name] = attr
	}

	return attrs
}

// GetMessageAttributeNames extracts MessageAttributeNames parameter for ReceiveMessage.
// In JSON Protocol: {"MessageAttributeNames": ["Priority", "Timestamp"]}
func (r *JSONRequest) GetMessageAttributeNames() []string {
	var names []string

	raw, ok := r.Body["MessageAttributeNames"]
	if !ok {
		return names
	}

	arr, ok := raw.([]any)
	if !ok {
		return names
	}

	for _, v := range arr {
		if s, ok := v.(string); ok {
			names = append(names, s)
		}
	}

	return names
}

// GetMessageSystemAttributes extracts message system attributes from the JSON body.
// In JSON Protocol: {"MessageSystemAttributes": {"AWSTraceHeader": {"DataType": "String", "StringValue": "trace-id"}, ...}}
func (r *JSONRequest) GetMessageSystemAttributes() map[string]types.MessageSystemAttribute {
	attrs := make(map[string]types.MessageSystemAttribute)

	raw, ok := r.Body["MessageSystemAttributes"]
	if !ok {
		return attrs
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return attrs
	}

	return r.parseSystemAttrsFromMap(m)
}

// parseSystemAttrsFromMap parses a map of system attribute name → value into typed attributes.
func (r *JSONRequest) parseSystemAttrsFromMap(m map[string]any) map[string]types.MessageSystemAttribute {
	attrs := make(map[string]types.MessageSystemAttribute)

	for name, val := range m {
		attrMap, ok := val.(map[string]any)
		if !ok {
			continue
		}

		attr := types.MessageSystemAttribute{}
		if v, ok := attrMap["DataType"].(string); ok {
			attr.DataType = v
		}
		if v, ok := attrMap["StringValue"].(string); ok {
			attr.StringValue = v
		}
		if v, ok := attrMap["BinaryValue"].(string); ok {
			if decoded, err := base64.StdEncoding.DecodeString(v); err == nil {
				attr.BinaryValue = decoded
			}
		}
		attrs[name] = attr
	}

	return attrs
}

// GetTags extracts tags from the JSON body.
// In JSON Protocol: {"Tags": {"Environment": "dev", "Team": "backend"}}
func (r *JSONRequest) GetTags() map[string]string {
	tags := make(map[string]string)

	raw, ok := r.Body["Tags"]
	if !ok {
		return tags
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return tags
	}

	for k, v := range m {
		if s, ok := v.(string); ok {
			tags[k] = s
		}
	}

	return tags
}

// GetTagKeys extracts TagKeys parameter for UntagQueue.
// In JSON Protocol: {"TagKeys": ["Environment", "Team"]}
func (r *JSONRequest) GetTagKeys() []string {
	var keys []string

	raw, ok := r.Body["TagKeys"]
	if !ok {
		return keys
	}

	arr, ok := raw.([]any)
	if !ok {
		return keys
	}

	for _, v := range arr {
		if s, ok := v.(string); ok {
			keys = append(keys, s)
		}
	}

	return keys
}

// GetMessageDeduplicationId extracts the MessageDeduplicationId parameter.
func (r *JSONRequest) GetMessageDeduplicationId() string {
	return r.getString("MessageDeduplicationId")
}

// GetMessageGroupId extracts the MessageGroupId parameter.
func (r *JSONRequest) GetMessageGroupId() string {
	return r.getString("MessageGroupId")
}

// GetSourceArn extracts the SourceArn parameter for message move tasks.
func (r *JSONRequest) GetSourceArn() string {
	return r.getString("SourceArn")
}

// GetDestinationArn extracts the DestinationArn parameter for message move tasks.
func (r *JSONRequest) GetDestinationArn() string {
	return r.getString("DestinationArn")
}

// GetTaskHandle extracts the TaskHandle parameter for cancel message move task.
func (r *JSONRequest) GetTaskHandle() string {
	return r.getString("TaskHandle")
}

// GetMaxNumberOfMessagesPerSecond extracts the rate limit for message move tasks.
func (r *JSONRequest) GetMaxNumberOfMessagesPerSecond() int {
	return r.getInt("MaxNumberOfMessagesPerSecond")
}

// JSONBatchEntry represents a single entry in a JSON Protocol batch request.
type JSONBatchEntry struct {
	ID                      string
	MessageBody             string
	DelaySeconds            int
	VisibilityTimeout       int
	ReceiptHandle           string
	MessageDeduplicationID  string
	MessageGroupID          string
	MessageSystemAttributes map[string]types.MessageSystemAttribute
}
