package protocol

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// QueryRequest represents a parsed AWS Query Protocol request.
// The Query Protocol uses form-urlencoded bodies with flat parameter names.
// Batch entries use indexed naming like "SendMessageBatchRequestEntry.1.Id".
type QueryRequest struct {
	Action       string
	Params       url.Values
	Attributes   map[string]string
	BatchEntries []QueryBatchEntry
}

// QueryBatchEntry represents a single entry in a batch request.
type QueryBatchEntry struct {
	ID                      string
	MessageBody             string
	DelaySeconds            int
	ReceiptHandle           string
	VisibilityTimeout       int
	MessageAttributes       map[string]MessageAttributeInput
	MessageDeduplicationID  string
	MessageGroupID          string
	MessageSystemAttributes map[string]types.MessageSystemAttribute
}

// MessageAttributeInput is the protocol-level representation of a message attribute.
type MessageAttributeInput struct {
	DataType    string
	StringValue string
	BinaryValue string // base64-encoded
}

// maxAttributeParams is the maximum number of Attribute.N.Name/Value pairs to parse.
const maxAttributeParams = 20

// ParseQueryRequest parses an AWS Query Protocol form-urlencoded body.
// The body is expected to be in application/x-www-form-urlencoded format.
func ParseQueryRequest(body string) (*QueryRequest, error) {
	values, err := url.ParseQuery(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query body: %w", err)
	}

	action := values.Get("Action")
	if action == "" {
		action = values.Get("action")
	}

	req := &QueryRequest{
		Action:     action,
		Params:     values,
		Attributes: parseQueryAttributes(values),
	}

	req.BatchEntries = parseBatchEntries(values)

	return req, nil
}

// parseQueryAttributes extracts queue attributes from form parameters.
// Query Protocol uses "Attribute.N.Name" and "Attribute.N.Value" pairs.
func parseQueryAttributes(values url.Values) map[string]string {
	attrs := make(map[string]string)

	// Handle Attribute.N.Name / Attribute.N.Value pattern
	for i := 1; i <= maxAttributeParams; i++ {
		nameKey := fmt.Sprintf("Attribute.%d.Name", i)
		valueKey := fmt.Sprintf("Attribute.%d.Value", i)

		name := values.Get(nameKey)
		value := values.Get(valueKey)

		if name != "" {
			attrs[name] = value
		}
	}

	// Also handle flat key=value attributes (used by some clients)
	for key, vals := range values {
		if len(vals) == 0 {
			continue
		}
		// Skip non-attribute keys
		if isReservedQueryParam(key) {
			continue
		}
		// If it looks like an attribute name (not a batch or indexed param)
		if !strings.Contains(key, ".") && !isKnownParam(key) {
			attrs[key] = vals[0]
		}
	}

	return attrs
}

// parseBatchEntries extracts batch entries from form parameters.
// Batch entries use the pattern: <Prefix>.N.<Field>
// e.g., SendMessageBatchRequestEntry.1.Id, SendMessageBatchRequestEntry.1.MessageBody
func parseBatchEntries(values url.Values) []QueryBatchEntry {
	entries := make(map[int]*QueryBatchEntry)
	// Track message attribute sub-fields per batch entry index
	type msgAttrParts struct {
		name        string
		dataType    string
		stringValue string
		binaryValue string
	}
	msgAttrData := make(map[int]map[int]*msgAttrParts)

	for key, vals := range values {
		if len(vals) == 0 {
			continue
		}
		value := vals[0]

		// Try to parse batch entry keys: <Prefix>.<Index>.<Field>
		parts := strings.Split(key, ".")
		if len(parts) < 3 {
			continue
		}

		// Check if the second part is a number (the index)
		idx, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}

		// Only process known batch entry prefixes
		if !isBatchEntryPrefix(parts[0]) {
			continue
		}

		entry, ok := entries[idx]
		if !ok {
			entry = &QueryBatchEntry{
				MessageAttributes: make(map[string]MessageAttributeInput),
			}
			entries[idx] = entry
			msgAttrData[idx] = make(map[int]*msgAttrParts)
		}

		// The field is parts[2]
		field := parts[2]

		switch field {
		case "Id":
			entry.ID = value
		case "MessageBody":
			entry.MessageBody = value
		case "DelaySeconds":
			entry.DelaySeconds, _ = strconv.Atoi(value)
		case "MessageDeduplicationId":
			entry.MessageDeduplicationID = value
		case "MessageGroupId":
			entry.MessageGroupID = value
		case "ReceiptHandle":
			entry.ReceiptHandle = value
		case "VisibilityTimeout":
			entry.VisibilityTimeout, _ = strconv.Atoi(value)
		case "MessageAttribute":
			// Pattern: <Prefix>.<Idx>.MessageAttribute.<AttrIdx>.<SubField>[.ValueSubField]
			if len(parts) < 5 {
				continue
			}
			attrIdx, err := strconv.Atoi(parts[3])
			if err != nil {
				continue
			}

			attrEntry, ok := msgAttrData[idx][attrIdx]
			if !ok {
				attrEntry = &msgAttrParts{}
				msgAttrData[idx][attrIdx] = attrEntry
			}

			subField := parts[4]
			switch subField {
			case "Name":
				attrEntry.name = value
			case "Value":
				if len(parts) >= 6 {
					valueSub := parts[5]
					switch valueSub {
					case "DataType":
						attrEntry.dataType = value
					case "StringValue":
						attrEntry.stringValue = value
					case "BinaryValue":
						attrEntry.binaryValue = value
					}
				}
			}
		}
	}

	// Assemble message attributes from parsed parts
	for idx, attrMap := range msgAttrData {
		entry, ok := entries[idx]
		if !ok {
			continue
		}
		for attrIdx := 1; attrIdx <= len(attrMap); attrIdx++ {
			ap, ok := attrMap[attrIdx]
			if !ok || ap.name == "" || ap.dataType == "" {
				continue
			}
			entry.MessageAttributes[ap.name] = MessageAttributeInput{
				DataType:    ap.dataType,
				StringValue: ap.stringValue,
				BinaryValue: ap.binaryValue,
			}
		}
	}

	// Convert map to sorted slice (handles non-contiguous indices)
	if len(entries) == 0 {
		return nil
	}

	indices := make([]int, 0, len(entries))
	for i := range entries {
		indices = append(indices, i)
	}
	sort.Ints(indices)

	result := make([]QueryBatchEntry, 0, len(entries))
	for _, i := range indices {
		if entry, ok := entries[i]; ok {
			result = append(result, *entry)
		}
	}

	return result
}

// isBatchEntryPrefix returns true if the key prefix indicates a batch entry.
func isBatchEntryPrefix(prefix string) bool {
	switch prefix {
	case "SendMessageBatchRequestEntry",
		"DeleteMessageBatchRequestEntry",
		"ChangeMessageVisibilityBatchRequestEntry":
		return true
	default:
		return false
	}
}

// isReservedQueryParam returns true if the key is a reserved SQS query parameter.
func isReservedQueryParam(key string) bool {
	switch key {
	case "Action", "action",
		"Version", "version",
		"QueueUrl", "queueUrl",
		"QueueName", "queueName",
		"MessageBody", "messageBody",
		"DelaySeconds", "delaySeconds",
		"VisibilityTimeout", "visibilityTimeout",
		"MaxNumberOfMessages", "maxNumberOfMessages",
		"WaitTimeSeconds", "waitTimeSeconds",
		"ReceiptHandle", "receiptHandle",
		"AttributeName", "attributeName",
		"MessageAttributeName", "messageAttributeName",
		"Attribute", "attribute",
		"TagKey", "tagKey",
		"TagValue", "tagValue",
		"Prefix", "prefix",
		"NextToken", "nextToken",
		"MaxResults", "maxResults",
		"MessageDeduplicationId", "messageDeduplicationId",
		"MessageGroupId", "messageGroupId",
		"MessageSystemAttribute", "messageSystemAttribute":
		return true
	default:
		return false
	}
}

// isKnownParam returns true if the key is a known SQS parameter (not an attribute).
func isKnownParam(key string) bool {
	return isReservedQueryParam(key) ||
		strings.Contains(key, ".") ||
		strings.HasPrefix(key, "AWS") ||
		strings.HasPrefix(key, "X-Amz")
}

// GetParam retrieves a parameter value, checking both capitalized and lowercase variants.
func (r *QueryRequest) GetParam(name string) string {
	if v := r.Params.Get(name); v != "" {
		return v
	}
	// Try lowercase variant
	return r.Params.Get(strings.ToLower(name))
}

// GetQueueURL extracts the QueueUrl parameter from the request.
func (r *QueryRequest) GetQueueURL() string {
	return r.GetParam("QueueUrl")
}

// GetQueueName extracts the QueueName parameter from the request.
func (r *QueryRequest) GetQueueName() string {
	return r.GetParam("QueueName")
}

// GetMessageBody extracts the MessageBody parameter from the request.
func (r *QueryRequest) GetMessageBody() string {
	return r.GetParam("MessageBody")
}

// GetDelaySeconds extracts the DelaySeconds parameter, defaulting to 0.
func (r *QueryRequest) GetDelaySeconds() int {
	s := r.GetParam("DelaySeconds")
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}

// GetVisibilityTimeout extracts the VisibilityTimeout parameter, defaulting to -1 (use queue default).
func (r *QueryRequest) GetVisibilityTimeout() int {
	s := r.GetParam("VisibilityTimeout")
	if s == "" {
		return -1
	}
	v, _ := strconv.Atoi(s)
	return v
}

// GetMaxNumberOfMessages extracts MaxNumberOfMessages, defaulting to 1.
func (r *QueryRequest) GetMaxNumberOfMessages() int {
	s := r.GetParam("MaxNumberOfMessages")
	if s == "" {
		return 1
	}
	v, _ := strconv.Atoi(s)
	return v
}

// GetWaitTimeSeconds extracts WaitTimeSeconds, defaulting to -1 (use queue default).
func (r *QueryRequest) GetWaitTimeSeconds() int {
	s := r.GetParam("WaitTimeSeconds")
	if s == "" {
		return -1
	}
	v, _ := strconv.Atoi(s)
	return v
}

// GetReceiptHandle extracts the ReceiptHandle parameter.
func (r *QueryRequest) GetReceiptHandle() string {
	return r.GetParam("ReceiptHandle")
}

// GetPrefix extracts the Prefix parameter for ListQueues.
func (r *QueryRequest) GetPrefix() string {
	return r.GetParam("Prefix")
}

// GetAttributeNames extracts the AttributeName.N parameters for GetQueueAttributes.
func (r *QueryRequest) GetAttributeNames() []string {
	var names []string
	for i := 1; i <= maxAttributeParams; i++ {
		key := fmt.Sprintf("AttributeName.%d", i)
		if v := r.Params.Get(key); v != "" {
			names = append(names, v)
		}
	}
	// Also check flat AttributeName
	if v := r.Params.Get("AttributeName"); v != "" {
		names = append(names, v)
	}
	return names
}

// GetMessageAttributes extracts message attributes from form parameters.
// Query Protocol uses "MessageAttribute.N.Name", "MessageAttribute.N.Value.DataType",
// "MessageAttribute.N.Value.StringValue", "MessageAttribute.N.Value.BinaryValue".
func (r *QueryRequest) GetMessageAttributes() map[string]types.MessageAttribute {
	attrs := make(map[string]types.MessageAttribute)

	// Collect all MessageAttribute.N.* keys
	type attrParts struct {
		name        string
		dataType    string
		stringValue string
		binaryValue string
	}

	parts := make(map[int]*attrParts)

	for key, vals := range r.Params {
		if len(vals) == 0 {
			continue
		}
		value := vals[0]

		// Must start with MessageAttribute.
		if !strings.HasPrefix(key, "MessageAttribute.") {
			continue
		}

		// Split: MessageAttribute.N.Name, MessageAttribute.N.Value.DataType, etc.
		segments := strings.Split(key, ".")
		if len(segments) < 3 {
			continue
		}

		idx, err := strconv.Atoi(segments[1])
		if err != nil {
			continue
		}

		entry, ok := parts[idx]
		if !ok {
			entry = &attrParts{}
			parts[idx] = entry
		}

		// The field is segments[2] (and possibly segments[3] for Value.X)
		field := segments[2]
		switch field {
		case "Name":
			entry.name = value
		case "Value":
			if len(segments) >= 4 {
				subField := segments[3]
				switch subField {
				case "DataType":
					entry.dataType = value
				case "StringValue":
					entry.stringValue = value
				case "BinaryValue":
					entry.binaryValue = value
				}
			}
		}
	}

	for i := 1; i <= len(parts); i++ {
		entry, ok := parts[i]
		if !ok || entry.name == "" || entry.dataType == "" {
			continue
		}

		attr := types.MessageAttribute{
			DataType:    entry.dataType,
			StringValue: entry.stringValue,
		}
		if entry.binaryValue != "" {
			decoded, err := base64.StdEncoding.DecodeString(entry.binaryValue)
			if err == nil {
				attr.BinaryValue = decoded
			}
		}
		attrs[entry.name] = attr
	}

	return attrs
}

// GetMessageAttributeNames extracts MessageAttributeName.N parameters for ReceiveMessage.
func (r *QueryRequest) GetMessageAttributeNames() []string {
	var names []string
	for i := 1; i <= maxAttributeParams; i++ {
		key := fmt.Sprintf("MessageAttributeName.%d", i)
		if v := r.Params.Get(key); v != "" {
			names = append(names, v)
		}
	}
	// Also check flat MessageAttributeName
	if v := r.Params.Get("MessageAttributeName"); v != "" {
		names = append(names, v)
	}
	return names
}

// GetAttributes returns the queue attributes parsed from the request.
func (r *QueryRequest) GetAttributes() map[string]string {
	return r.Attributes
}

// GetMessageSystemAttributes extracts message system attributes from form parameters.
// Query Protocol uses "MessageSystemAttribute.N.Name", "MessageSystemAttribute.N.Value.DataType",
// "MessageSystemAttribute.N.Value.StringValue", "MessageSystemAttribute.N.Value.BinaryValue".
func (r *QueryRequest) GetMessageSystemAttributes() map[string]types.MessageSystemAttribute {
	attrs := make(map[string]types.MessageSystemAttribute)

	type attrParts struct {
		name        string
		dataType    string
		stringValue string
		binaryValue string
	}

	parts := make(map[int]*attrParts)

	for key, vals := range r.Params {
		if len(vals) == 0 {
			continue
		}
		value := vals[0]

		if !strings.HasPrefix(key, "MessageSystemAttribute.") {
			continue
		}

		segments := strings.Split(key, ".")
		if len(segments) < 3 {
			continue
		}

		idx, err := strconv.Atoi(segments[1])
		if err != nil {
			continue
		}

		entry, ok := parts[idx]
		if !ok {
			entry = &attrParts{}
			parts[idx] = entry
		}

		field := segments[2]
		switch field {
		case "Name":
			entry.name = value
		case "Value":
			if len(segments) >= 4 {
				subField := segments[3]
				switch subField {
				case "DataType":
					entry.dataType = value
				case "StringValue":
					entry.stringValue = value
				case "BinaryValue":
					entry.binaryValue = value
				}
			}
		}
	}

	for i := 1; i <= len(parts); i++ {
		entry, ok := parts[i]
		if !ok || entry.name == "" || entry.dataType == "" {
			continue
		}

		attr := types.MessageSystemAttribute{
			DataType:    entry.dataType,
			StringValue: entry.stringValue,
		}
		if entry.binaryValue != "" {
			decoded, err := base64.StdEncoding.DecodeString(entry.binaryValue)
			if err == nil {
				attr.BinaryValue = decoded
			}
		}
		attrs[entry.name] = attr
	}

	return attrs
}

// GetTags extracts tags from form parameters.
// Query Protocol uses "Tag.N.Key" and "Tag.N.Value" pairs.
func (r *QueryRequest) GetTags() map[string]string {
	tags := make(map[string]string)

	for i := 1; i <= 50; i++ {
		keyKey := fmt.Sprintf("Tag.%d.Key", i)
		valueKey := fmt.Sprintf("Tag.%d.Value", i)

		key := r.Params.Get(keyKey)
		value := r.Params.Get(valueKey)

		if key != "" {
			tags[key] = value
		}
	}

	return tags
}

// GetTagKeys extracts TagKey.N parameters for UntagQueue.
func (r *QueryRequest) GetTagKeys() []string {
	var keys []string
	for i := 1; i <= 50; i++ {
		key := fmt.Sprintf("TagKey.%d", i)
		if v := r.Params.Get(key); v != "" {
			keys = append(keys, v)
		}
	}
	return keys
}

// GetMessageDeduplicationId extracts the MessageDeduplicationId parameter.
func (r *QueryRequest) GetMessageDeduplicationId() string {
	return r.GetParam("MessageDeduplicationId")
}

// GetMessageGroupId extracts the MessageGroupId parameter.
func (r *QueryRequest) GetMessageGroupId() string {
	return r.GetParam("MessageGroupId")
}

// GetBatchEntries returns the batch entries parsed from the request.
func (r *QueryRequest) GetBatchEntries() []QueryBatchEntry {
	return r.BatchEntries
}

// GetAction returns the action name from the request.
func (r *QueryRequest) GetAction() string {
	return r.Action
}

// GetSourceArn extracts the SourceArn parameter for message move tasks.
func (r *QueryRequest) GetSourceArn() string {
	return r.GetParam("SourceArn")
}

// GetDestinationArn extracts the DestinationArn parameter for message move tasks.
func (r *QueryRequest) GetDestinationArn() string {
	return r.GetParam("DestinationArn")
}

// GetTaskHandle extracts the TaskHandle parameter for cancel message move task.
func (r *QueryRequest) GetTaskHandle() string {
	return r.GetParam("TaskHandle")
}

// GetMaxNumberOfMessagesPerSecond extracts the rate limit for message move tasks.
func (r *QueryRequest) GetMaxNumberOfMessagesPerSecond() int {
	s := r.GetParam("MaxNumberOfMessagesPerSecond")
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}
