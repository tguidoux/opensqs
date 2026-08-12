package handlers

import (
	"github.com/tguidoux/opensqs/apps/go/server/protocol"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// JSONRequestAdapter wraps a protocol.JSONRequest to implement the Request interface.
type JSONRequestAdapter struct {
	*protocol.JSONRequest
}

func (a *JSONRequestAdapter) GetAction() string         { return a.JSONRequest.GetAction() }
func (a *JSONRequestAdapter) GetQueueURL() string       { return a.JSONRequest.GetQueueURL() }
func (a *JSONRequestAdapter) GetQueueName() string      { return a.JSONRequest.GetQueueName() }
func (a *JSONRequestAdapter) GetMessageBody() string    { return a.JSONRequest.GetMessageBody() }
func (a *JSONRequestAdapter) GetDelaySeconds() int      { return a.JSONRequest.GetDelaySeconds() }
func (a *JSONRequestAdapter) GetVisibilityTimeout() int { return a.JSONRequest.GetVisibilityTimeout() }
func (a *JSONRequestAdapter) GetMaxNumberOfMessages() int {
	return a.JSONRequest.GetMaxNumberOfMessages()
}
func (a *JSONRequestAdapter) GetWaitTimeSeconds() int  { return a.JSONRequest.GetWaitTimeSeconds() }
func (a *JSONRequestAdapter) GetReceiptHandle() string { return a.JSONRequest.GetReceiptHandle() }
func (a *JSONRequestAdapter) GetPrefix() string        { return a.JSONRequest.GetPrefix() }
func (a *JSONRequestAdapter) GetAttributeNames() []string {
	return a.JSONRequest.GetAttributeNames()
}
func (a *JSONRequestAdapter) GetMessageAttributes() map[string]types.MessageAttribute {
	return a.JSONRequest.GetMessageAttributes()
}
func (a *JSONRequestAdapter) GetMessageAttributeNames() []string {
	return a.JSONRequest.GetMessageAttributeNames()
}
func (a *JSONRequestAdapter) GetAttributes() map[string]string { return a.JSONRequest.GetAttributes() }
func (a *JSONRequestAdapter) GetTags() map[string]string       { return a.JSONRequest.GetTags() }
func (a *JSONRequestAdapter) GetTagKeys() []string             { return a.JSONRequest.GetTagKeys() }
func (a *JSONRequestAdapter) GetMessageDeduplicationID() string {
	return a.JSONRequest.GetMessageDeduplicationId()
}
func (a *JSONRequestAdapter) GetMessageGroupID() string {
	return a.JSONRequest.GetMessageGroupId()
}
func (a *JSONRequestAdapter) GetSourceArn() string {
	return a.JSONRequest.GetSourceArn()
}
func (a *JSONRequestAdapter) GetDestinationArn() string {
	return a.JSONRequest.GetDestinationArn()
}
func (a *JSONRequestAdapter) GetTaskHandle() string {
	return a.JSONRequest.GetTaskHandle()
}
func (a *JSONRequestAdapter) GetMaxNumberOfMessagesPerSecond() int {
	return a.JSONRequest.GetMaxNumberOfMessagesPerSecond()
}
func (a *JSONRequestAdapter) GetMessageSystemAttributes() map[string]types.MessageSystemAttribute {
	return a.JSONRequest.GetMessageSystemAttributes()
}
func (a *JSONRequestAdapter) GetBatchEntries() []BatchEntry {
	jsonEntries := a.JSONRequest.GetBatchEntries()
	entries := make([]BatchEntry, len(jsonEntries))
	for i, e := range jsonEntries {
		entries[i] = BatchEntry{
			ID:                      e.ID,
			MessageBody:             e.MessageBody,
			DelaySeconds:            e.DelaySeconds,
			ReceiptHandle:           e.ReceiptHandle,
			VisibilityTimeout:       e.VisibilityTimeout,
			MessageDeduplicationID:  e.MessageDeduplicationID,
			MessageGroupID:          e.MessageGroupID,
			MessageSystemAttributes: e.MessageSystemAttributes,
		}
	}
	return entries
}
