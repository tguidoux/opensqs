package handlers

import (
	"github.com/tguidoux/opensqs/apps/go/server/protocol"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// QueryRequestAdapter wraps a protocol.QueryRequest to implement the Request interface.
type QueryRequestAdapter struct {
	*protocol.QueryRequest
}

func (a *QueryRequestAdapter) GetAction() string      { return a.QueryRequest.GetAction() }
func (a *QueryRequestAdapter) GetQueueURL() string    { return a.QueryRequest.GetQueueURL() }
func (a *QueryRequestAdapter) GetQueueName() string   { return a.QueryRequest.GetQueueName() }
func (a *QueryRequestAdapter) GetMessageBody() string { return a.QueryRequest.GetMessageBody() }
func (a *QueryRequestAdapter) GetDelaySeconds() int   { return a.QueryRequest.GetDelaySeconds() }
func (a *QueryRequestAdapter) GetVisibilityTimeout() int {
	return a.QueryRequest.GetVisibilityTimeout()
}
func (a *QueryRequestAdapter) GetMaxNumberOfMessages() int {
	return a.QueryRequest.GetMaxNumberOfMessages()
}
func (a *QueryRequestAdapter) GetWaitTimeSeconds() int  { return a.QueryRequest.GetWaitTimeSeconds() }
func (a *QueryRequestAdapter) GetReceiptHandle() string { return a.QueryRequest.GetReceiptHandle() }
func (a *QueryRequestAdapter) GetPrefix() string        { return a.QueryRequest.GetPrefix() }
func (a *QueryRequestAdapter) GetAttributeNames() []string {
	return a.QueryRequest.GetAttributeNames()
}
func (a *QueryRequestAdapter) GetMessageAttributes() map[string]types.MessageAttribute {
	return a.QueryRequest.GetMessageAttributes()
}
func (a *QueryRequestAdapter) GetMessageAttributeNames() []string {
	return a.QueryRequest.GetMessageAttributeNames()
}
func (a *QueryRequestAdapter) GetAttributes() map[string]string {
	return a.QueryRequest.GetAttributes()
}
func (a *QueryRequestAdapter) GetTags() map[string]string { return a.QueryRequest.GetTags() }
func (a *QueryRequestAdapter) GetTagKeys() []string       { return a.QueryRequest.GetTagKeys() }
func (a *QueryRequestAdapter) GetMessageDeduplicationID() string {
	return a.QueryRequest.GetMessageDeduplicationId()
}
func (a *QueryRequestAdapter) GetMessageGroupID() string {
	return a.QueryRequest.GetMessageGroupId()
}
func (a *QueryRequestAdapter) GetMessageSystemAttributes() map[string]types.MessageSystemAttribute {
	return a.QueryRequest.GetMessageSystemAttributes()
}
func (a *QueryRequestAdapter) GetSourceArn() string {
	return a.QueryRequest.GetSourceArn()
}
func (a *QueryRequestAdapter) GetDestinationArn() string {
	return a.QueryRequest.GetDestinationArn()
}
func (a *QueryRequestAdapter) GetTaskHandle() string {
	return a.QueryRequest.GetTaskHandle()
}
func (a *QueryRequestAdapter) GetMaxNumberOfMessagesPerSecond() int {
	return a.QueryRequest.GetMaxNumberOfMessagesPerSecond()
}
func (a *QueryRequestAdapter) GetBatchEntries() []BatchEntry {
	queryEntries := a.QueryRequest.GetBatchEntries()
	entries := make([]BatchEntry, len(queryEntries))
	for i, e := range queryEntries {
		entries[i] = BatchEntry{
			ID:                      e.ID,
			MessageBody:             e.MessageBody,
			DelaySeconds:            e.DelaySeconds,
			ReceiptHandle:           e.ReceiptHandle,
			VisibilityTimeout:       e.VisibilityTimeout,
			MessageAttributes:       convertQueryMsgAttrs(e.MessageAttributes),
			MessageDeduplicationID:  e.MessageDeduplicationID,
			MessageGroupID:          e.MessageGroupID,
			MessageSystemAttributes: e.MessageSystemAttributes,
		}
	}
	return entries
}
