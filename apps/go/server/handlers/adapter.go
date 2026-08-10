package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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
func (a *QueryRequestAdapter) GetWaitTimeSeconds() int     { return a.QueryRequest.GetWaitTimeSeconds() }
func (a *QueryRequestAdapter) GetReceiptHandle() string    { return a.QueryRequest.GetReceiptHandle() }
func (a *QueryRequestAdapter) GetPrefix() string           { return a.QueryRequest.GetPrefix() }
func (a *QueryRequestAdapter) GetAttributeNames() []string { return a.QueryRequest.GetAttributeNames() }
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
func (a *JSONRequestAdapter) GetWaitTimeSeconds() int     { return a.JSONRequest.GetWaitTimeSeconds() }
func (a *JSONRequestAdapter) GetReceiptHandle() string    { return a.JSONRequest.GetReceiptHandle() }
func (a *JSONRequestAdapter) GetPrefix() string           { return a.JSONRequest.GetPrefix() }
func (a *JSONRequestAdapter) GetAttributeNames() []string { return a.JSONRequest.GetAttributeNames() }
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

// convertQueryMsgAttrs converts protocol.MessageAttributeInput map to types.MessageAttribute map.
func convertQueryMsgAttrs(input map[string]protocol.MessageAttributeInput) map[string]types.MessageAttribute {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]types.MessageAttribute, len(input))
	for name, attr := range input {
		ma := types.MessageAttribute{
			DataType:    attr.DataType,
			StringValue: attr.StringValue,
		}
		if attr.BinaryValue != "" {
			if decoded, err := base64.StdEncoding.DecodeString(attr.BinaryValue); err == nil {
				ma.BinaryValue = decoded
			}
		}
		result[name] = ma
	}
	return result
}

// convertBatchErrorsXML converts handler BatchError slice to XML response type.
func convertBatchErrorsXML(errors []BatchError) []protocol.BatchResultErrorEntry {
	if len(errors) == 0 {
		return nil
	}
	result := make([]protocol.BatchResultErrorEntry, 0, len(errors))
	for _, e := range errors {
		result = append(result, protocol.BatchResultErrorEntry{
			ID:          e.ID,
			Code:        e.Code,
			Message:     e.Message,
			SenderFault: e.SenderFault,
		})
	}
	return result
}

// convertBatchErrorsJSON converts handler BatchError slice to JSON response type.
func convertBatchErrorsJSON(errors []BatchError) []protocol.JSONBatchErrorEntry {
	if len(errors) == 0 {
		return nil
	}
	result := make([]protocol.JSONBatchErrorEntry, 0, len(errors))
	for _, e := range errors {
		result = append(result, protocol.JSONBatchErrorEntry{
			ID:          e.ID,
			Code:        e.Code,
			Message:     e.Message,
			SenderFault: e.SenderFault,
		})
	}
	return result
}

// DetectProtocol determines the protocol type from the HTTP request.
func DetectProtocol(r *http.Request) (ProtocolType, string) {
	contentType := r.Header.Get("Content-Type")
	targetHeader := r.Header.Get("X-Amz-Target")

	// JSON protocol is identified by X-Amz-Target header
	if targetHeader != "" {
		return JSONProtocol, targetHeader
	}

	// Strip charset from content type (e.g., "application/x-www-form-urlencoded; charset=utf-8")
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	// Default to Query protocol for form-urlencoded
	if contentType == types.QueryProtocolContentType || contentType == "" {
		return QueryProtocol, ""
	}

	// Check if it's JSON content type
	if contentType == types.JSONProtocolContentType {
		return JSONProtocol, ""
	}

	// Default to Query protocol
	return QueryProtocol, ""
}

// MarshalResponse converts a handler Response to XML or JSON bytes based on protocol.
func MarshalResponse(resp *Response, proto ProtocolType) ([]byte, error) {
	switch proto {
	case JSONProtocol:
		return marshalJSONResponse(resp)
	default:
		return marshalXMLResponse(resp)
	}
}

// marshalXMLResponse converts a handler Response to XML bytes.
func marshalXMLResponse(resp *Response) ([]byte, error) {
	switch resp.Action {
	case types.ActionCreateQueue:
		return protocol.MarshalXMLResponse(protocol.CreateQueueResponse{
			QueueURL:         resp.QueueURL,
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionDeleteQueue:
		return protocol.MarshalXMLResponse(protocol.DeleteQueueResponse{
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionGetQueueUrl:
		return protocol.MarshalXMLResponse(protocol.GetQueueURLResponse{
			QueueURL:         resp.QueueURL,
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionListQueues:
		return protocol.MarshalXMLResponse(protocol.ListQueuesResponse{
			QueueURLs:        resp.QueueURLs,
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionSendMessage:
		xmlMsg := buildXMLMessage(resp.Message)
		return protocol.MarshalXMLResponse(protocol.SendMessageResponse{
			MessageID:                    xmlMsg.MessageID,
			MD5OfMessageBody:             xmlMsg.MD5OfBody,
			MD5OfMessageAttributes:       xmlMsg.MD5OfMessageAttributes,
			MD5OfMessageSystemAttributes: xmlMsg.MD5OfMessageSystemAttributes,
			SequenceNumber:               xmlMsg.SequenceNumber,
			ResponseMetadata:             protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionReceiveMessage:
		messages := make([]protocol.XMLMessage, 0, len(resp.Messages))
		for _, msg := range resp.Messages {
			messages = append(messages, buildXMLMessage(msg))
		}
		return protocol.MarshalXMLResponse(protocol.ReceiveMessageResponse{
			Messages:         messages,
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionDeleteMessage:
		return protocol.MarshalXMLResponse(protocol.DeleteMessageResponse{
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionChangeMessageVisibility:
		return protocol.MarshalXMLResponse(protocol.ChangeMessageVisibilityResponse{
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionGetQueueAttributes:
		attrs := make([]protocol.XMLAttribute, 0, len(resp.Attributes))
		for k, v := range resp.Attributes {
			attrs = append(attrs, protocol.XMLAttribute{Name: k, Value: v})
		}
		return protocol.MarshalXMLResponse(protocol.GetQueueAttributesResponse{
			Attributes:       attrs,
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionSetQueueAttributes:
		return protocol.MarshalXMLResponse(protocol.SetQueueAttributesResponse{
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionPurgeQueue:
		return protocol.MarshalXMLResponse(protocol.PurgeQueueResponse{
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionSendMessageBatch:
		entries := make([]protocol.SendMessageBatchResultEntry, 0, len(resp.BatchResults))
		for _, r := range resp.BatchResults {
			entries = append(entries, protocol.SendMessageBatchResultEntry{
				ID:                           r.ID,
				MessageID:                    r.MessageID,
				MD5OfMessageBody:             r.MD5OfBody,
				MD5OfMessageAttributes:       r.MD5OfMessageAttributes,
				MD5OfMessageSystemAttributes: r.MD5OfMessageSystemAttributes,
				SequenceNumber:               r.SequenceNumber,
			})
		}
		batchErrors := convertBatchErrorsXML(resp.BatchErrors)
		return protocol.MarshalXMLResponse(protocol.SendMessageBatchResponse{
			Entries:          entries,
			Errors:           batchErrors,
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionDeleteMessageBatch:
		entries := make([]protocol.DeleteMessageBatchResultEntry, 0, len(resp.BatchResults))
		for _, r := range resp.BatchResults {
			entries = append(entries, protocol.DeleteMessageBatchResultEntry{ID: r.ID})
		}
		batchErrors := convertBatchErrorsXML(resp.BatchErrors)
		return protocol.MarshalXMLResponse(protocol.DeleteMessageBatchResponse{
			Entries:          entries,
			Errors:           batchErrors,
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionChangeMessageVisibilityBatch:
		entries := make([]protocol.ChangeMessageVisibilityBatchResultEntry, 0, len(resp.BatchResults))
		for _, r := range resp.BatchResults {
			entries = append(entries, protocol.ChangeMessageVisibilityBatchResultEntry{ID: r.ID})
		}
		batchErrors := convertBatchErrorsXML(resp.BatchErrors)
		return protocol.MarshalXMLResponse(protocol.ChangeMessageVisibilityBatchResponse{
			Entries:          entries,
			Errors:           batchErrors,
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionTagQueue:
		return protocol.MarshalXMLResponse(protocol.TagQueueResponse{
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionUntagQueue:
		return protocol.MarshalXMLResponse(protocol.UntagQueueResponse{
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionListQueueTags:
		tags := make([]protocol.XMLTagEntry, 0, len(resp.Tags))
		for k, v := range resp.Tags {
			tags = append(tags, protocol.XMLTagEntry{Key: k, Value: v})
		}
		return protocol.MarshalXMLResponse(protocol.ListQueueTagsResponse{
			Tags:             tags,
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionAddPermission:
		return protocol.MarshalXMLResponse(protocol.AddPermissionResponse{
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionRemovePermission:
		return protocol.MarshalXMLResponse(protocol.RemovePermissionResponse{
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionListDeadLetterSourceQueues:
		return protocol.MarshalXMLResponse(protocol.ListDeadLetterSourceQueuesResponse{
			QueueURLs:        resp.QueueURLs,
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionStartMessageMoveTask:
		return protocol.MarshalXMLResponse(protocol.StartMessageMoveTaskResponse{
			TaskHandle:       resp.MoveTaskResult.TaskHandle,
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionCancelMessageMoveTask:
		return protocol.MarshalXMLResponse(protocol.CancelMessageMoveTaskResponse{
			MovedMessages:    resp.MoveTaskResult.MovedMessages,
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	case types.ActionListMessageMoveTasks:
		results := make([]protocol.XMLMoveTaskResult, 0, len(resp.MoveTaskResults))
		for _, r := range resp.MoveTaskResults {
			results = append(results, protocol.XMLMoveTaskResult{
				TaskHandle:                   r.TaskHandle,
				SourceArn:                    r.SourceArn,
				DestinationArn:               r.DestinationArn,
				Status:                       r.Status,
				MaxNumberOfMessagesPerSecond: r.MaxNumberOfMessagesPerSecond,
				MovedMessages:                r.MovedMessages,
			})
		}
		return protocol.MarshalXMLResponse(protocol.ListMessageMoveTasksResponse{
			Results:          results,
			ResponseMetadata: protocol.ResponseMetadata{RequestID: resp.RequestID},
		})
	default:
		return nil, fmt.Errorf("unknown action for XML marshaling: %s", resp.Action)
	}
}

// marshalJSONResponse converts a handler Response to JSON bytes.
func marshalJSONResponse(resp *Response) ([]byte, error) {
	switch resp.Action {
	case types.ActionCreateQueue:
		return protocol.MarshalJSONResponse(protocol.JSONCreateQueueResponse{
			QueueURL:  resp.QueueURL,
			RequestID: resp.RequestID,
		})
	case types.ActionDeleteQueue:
		return protocol.MarshalJSONResponse(protocol.JSONDeleteQueueResponse{
			RequestID: resp.RequestID,
		})
	case types.ActionGetQueueUrl:
		return protocol.MarshalJSONResponse(protocol.JSONGetQueueURLResponse{
			QueueURL:  resp.QueueURL,
			RequestID: resp.RequestID,
		})
	case types.ActionListQueues:
		return protocol.MarshalJSONResponse(protocol.JSONListQueuesResponse{
			QueueURLs: resp.QueueURLs,
			RequestID: resp.RequestID,
		})
	case types.ActionSendMessage:
		return protocol.MarshalJSONResponse(protocol.JSONSendMessageResponse{
			MessageID:                    resp.Message.MessageID,
			MD5OfMessageBody:             resp.Message.MD5OfBody,
			MD5OfMessageAttributes:       resp.Message.MD5OfMessageAttributes,
			MD5OfMessageSystemAttributes: resp.Message.MD5OfMessageSystemAttributes,
			SequenceNumber:               resp.Message.SequenceNumber,
			RequestID:                    resp.RequestID,
		})
	case types.ActionReceiveMessage:
		messages := make([]protocol.JSONMessage, 0, len(resp.Messages))
		for _, msg := range resp.Messages {
			messages = append(messages, buildJSONMessage(msg))
		}
		return protocol.MarshalJSONResponse(protocol.JSONReceiveMessageResponse{
			Messages:  messages,
			RequestID: resp.RequestID,
		})
	case types.ActionDeleteMessage:
		return protocol.MarshalJSONResponse(protocol.JSONDeleteMessageResponse{
			RequestID: resp.RequestID,
		})
	case types.ActionChangeMessageVisibility:
		return protocol.MarshalJSONResponse(protocol.JSONChangeMessageVisibilityResponse{
			RequestID: resp.RequestID,
		})
	case types.ActionGetQueueAttributes:
		return protocol.MarshalJSONResponse(protocol.JSONGetQueueAttributesResponse{
			Attributes: resp.Attributes,
			RequestID:  resp.RequestID,
		})
	case types.ActionSetQueueAttributes:
		return protocol.MarshalJSONResponse(protocol.JSONSetQueueAttributesResponse{
			RequestID: resp.RequestID,
		})
	case types.ActionPurgeQueue:
		return protocol.MarshalJSONResponse(protocol.JSONPurgeQueueResponse{
			RequestID: resp.RequestID,
		})
	case types.ActionSendMessageBatch:
		successful := make([]protocol.JSONBatchResultEntry, 0, len(resp.BatchResults))
		for _, r := range resp.BatchResults {
			successful = append(successful, protocol.JSONBatchResultEntry{
				ID:                           r.ID,
				MessageID:                    r.MessageID,
				MD5OfMessageBody:             r.MD5OfBody,
				MD5OfMessageAttributes:       r.MD5OfMessageAttributes,
				MD5OfMessageSystemAttributes: r.MD5OfMessageSystemAttributes,
				SequenceNumber:               r.SequenceNumber,
			})
		}
		failed := convertBatchErrorsJSON(resp.BatchErrors)
		return protocol.MarshalJSONResponse(protocol.JSONSendMessageBatchResponse{
			Successful: successful,
			Failed:     failed,
			RequestID:  resp.RequestID,
		})
	case types.ActionDeleteMessageBatch:
		successful := make([]protocol.JSONBatchResultEntry, 0, len(resp.BatchResults))
		for _, r := range resp.BatchResults {
			successful = append(successful, protocol.JSONBatchResultEntry{ID: r.ID})
		}
		failed := convertBatchErrorsJSON(resp.BatchErrors)
		return protocol.MarshalJSONResponse(protocol.JSONDeleteMessageBatchResponse{
			Successful: successful,
			Failed:     failed,
			RequestID:  resp.RequestID,
		})
	case types.ActionChangeMessageVisibilityBatch:
		successful := make([]protocol.JSONBatchResultEntry, 0, len(resp.BatchResults))
		for _, r := range resp.BatchResults {
			successful = append(successful, protocol.JSONBatchResultEntry{ID: r.ID})
		}
		failed := convertBatchErrorsJSON(resp.BatchErrors)
		return protocol.MarshalJSONResponse(protocol.JSONChangeMessageVisibilityBatchResponse{
			Successful: successful,
			Failed:     failed,
			RequestID:  resp.RequestID,
		})
	case types.ActionTagQueue:
		return protocol.MarshalJSONResponse(protocol.JSONTagQueueResponse{
			RequestID: resp.RequestID,
		})
	case types.ActionUntagQueue:
		return protocol.MarshalJSONResponse(protocol.JSONUntagQueueResponse{
			RequestID: resp.RequestID,
		})
	case types.ActionListQueueTags:
		return protocol.MarshalJSONResponse(protocol.JSONListQueueTagsResponse{
			Tags:      resp.Tags,
			RequestID: resp.RequestID,
		})
	case types.ActionAddPermission:
		return protocol.MarshalJSONResponse(protocol.JSONAddPermissionResponse{
			RequestID: resp.RequestID,
		})
	case types.ActionRemovePermission:
		return protocol.MarshalJSONResponse(protocol.JSONRemovePermissionResponse{
			RequestID: resp.RequestID,
		})
	case types.ActionListDeadLetterSourceQueues:
		return protocol.MarshalJSONResponse(protocol.JSONListDeadLetterSourceQueuesResponse{
			QueueURLs: resp.QueueURLs,
			RequestID: resp.RequestID,
		})
	case types.ActionStartMessageMoveTask:
		return protocol.MarshalJSONResponse(protocol.JSONStartMessageMoveTaskResponse{
			TaskHandle: resp.MoveTaskResult.TaskHandle,
			RequestID:  resp.RequestID,
		})
	case types.ActionCancelMessageMoveTask:
		return protocol.MarshalJSONResponse(protocol.JSONCancelMessageMoveTaskResponse{
			MovedMessages: resp.MoveTaskResult.MovedMessages,
			RequestID:     resp.RequestID,
		})
	case types.ActionListMessageMoveTasks:
		results := make([]protocol.JSONMoveTaskResult, 0, len(resp.MoveTaskResults))
		for _, r := range resp.MoveTaskResults {
			results = append(results, protocol.JSONMoveTaskResult{
				TaskHandle:                   r.TaskHandle,
				SourceArn:                    r.SourceArn,
				DestinationArn:               r.DestinationArn,
				Status:                       r.Status,
				MaxNumberOfMessagesPerSecond: r.MaxNumberOfMessagesPerSecond,
				MovedMessages:                r.MovedMessages,
			})
		}
		return protocol.MarshalJSONResponse(protocol.JSONListMessageMoveTasksResponse{
			Results:   results,
			RequestID: resp.RequestID,
		})
	default:
		return nil, fmt.Errorf("unknown action for JSON marshaling: %s", resp.Action)
	}
}

// buildXMLMessage converts a types.Message to an XMLMessage for XML responses.
func buildXMLMessage(msg *types.Message) protocol.XMLMessage {
	xmlMsg := protocol.XMLMessage{
		MessageID:                    msg.MessageID,
		ReceiptHandle:                msg.ReceiptHandle,
		MD5OfBody:                    msg.MD5OfBody,
		MD5OfMessageAttributes:       msg.MD5OfMessageAttributes,
		MD5OfMessageSystemAttributes: msg.MD5OfMessageSystemAttributes,
		Body:                         msg.Body,
		SequenceNumber:               msg.SequenceNumber,
	}

	// Convert message attributes
	for name, attr := range msg.MessageAttributes {
		var binaryStr string
		if len(attr.BinaryValue) > 0 {
			binaryStr = base64.StdEncoding.EncodeToString(attr.BinaryValue)
		}
		xmlMsg.MessageAttributes = append(xmlMsg.MessageAttributes, protocol.XMLMsgAttribute{
			Name: name,
			Value: protocol.XMLAttrValue{
				DataType:    attr.DataType,
				StringValue: attr.StringValue,
				BinaryValue: binaryStr,
			},
		})
	}

	if !msg.SentTimestamp.IsZero() {
		xmlMsg.Attributes = append(xmlMsg.Attributes, protocol.XMLAttribute{
			Name: "SentTimestamp", Value: formatTimestamp(msg.SentTimestamp),
		})
	}
	if msg.ApproximateReceiveCount > 0 {
		xmlMsg.Attributes = append(xmlMsg.Attributes, protocol.XMLAttribute{
			Name: "ApproximateReceiveCount", Value: formatInt(msg.ApproximateReceiveCount),
		})
	}
	if !msg.FirstReceivedTimestamp.IsZero() {
		xmlMsg.Attributes = append(xmlMsg.Attributes, protocol.XMLAttribute{
			Name: "ApproximateFirstReceiveTimestamp", Value: formatTimestamp(msg.FirstReceivedTimestamp),
		})
	}

	return xmlMsg
}

// buildJSONMessage converts a types.Message to a JSONMessage for JSON responses.
func buildJSONMessage(msg *types.Message) protocol.JSONMessage {
	jsonMsg := protocol.JSONMessage{
		MessageID:                    msg.MessageID,
		ReceiptHandle:                msg.ReceiptHandle,
		MD5OfBody:                    msg.MD5OfBody,
		MD5OfMessageAttributes:       msg.MD5OfMessageAttributes,
		MD5OfMessageSystemAttributes: msg.MD5OfMessageSystemAttributes,
		Body:                         msg.Body,
		SequenceNumber:               msg.SequenceNumber,
		Attributes:                   make(map[string]string),
	}

	// Convert message attributes
	if len(msg.MessageAttributes) > 0 {
		jsonMsg.MessageAttributes = make(map[string]protocol.JSONMsgAttribute)
		for name, attr := range msg.MessageAttributes {
			var binaryStr string
			if len(attr.BinaryValue) > 0 {
				binaryStr = base64.StdEncoding.EncodeToString(attr.BinaryValue)
			}
			jsonMsg.MessageAttributes[name] = protocol.JSONMsgAttribute{
				DataType:    attr.DataType,
				StringValue: attr.StringValue,
				BinaryValue: binaryStr,
			}
		}
	}

	if !msg.SentTimestamp.IsZero() {
		jsonMsg.Attributes["SentTimestamp"] = formatTimestamp(msg.SentTimestamp)
	}
	if msg.ApproximateReceiveCount > 0 {
		jsonMsg.Attributes["ApproximateReceiveCount"] = formatInt(msg.ApproximateReceiveCount)
	}
	if !msg.FirstReceivedTimestamp.IsZero() {
		jsonMsg.Attributes["ApproximateFirstReceiveTimestamp"] = formatTimestamp(msg.FirstReceivedTimestamp)
	}

	return jsonMsg
}

// formatTimestamp converts a time.Time to milliseconds since epoch string.
func formatTimestamp(t interface{ UnixMilli() int64 }) string {
	return strconv.FormatInt(t.UnixMilli(), 10)
}

// formatInt converts an int to a string.
func formatInt(i int) string {
	return strconv.FormatInt(int64(i), 10)
}
