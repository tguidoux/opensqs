package handlers

import (
	"fmt"

	"github.com/tguidoux/opensqs/apps/go/server/protocol"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// MarshalResponse converts a handler Response to XML or JSON bytes based on protocol.
func MarshalResponse(resp *Response, proto ProtocolType) ([]byte, error) {
	switch proto {
	case JSONProtocol:
		return marshalJSONResponse(resp)
	default:
		return marshalXMLResponse(resp)
	}
}

// convertBatchErrors converts handler BatchError slice to protocol-specific
// batch error entries using a constructor function to avoid duplicate code.
func convertBatchErrors[T any](errors []BatchError, makeEntry func(e BatchError) T) []T {
	if len(errors) == 0 {
		return nil
	}
	result := make([]T, 0, len(errors))
	for _, e := range errors {
		result = append(result, makeEntry(e))
	}
	return result
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
		batchErrors := convertBatchErrors(resp.BatchErrors, func(e BatchError) protocol.BatchResultErrorEntry {
			return protocol.BatchResultErrorEntry{ID: e.ID, Code: e.Code, Message: e.Message, SenderFault: e.SenderFault}
		})
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
		batchErrors := convertBatchErrors(resp.BatchErrors, func(e BatchError) protocol.BatchResultErrorEntry {
			return protocol.BatchResultErrorEntry{ID: e.ID, Code: e.Code, Message: e.Message, SenderFault: e.SenderFault}
		})
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
		batchErrors := convertBatchErrors(resp.BatchErrors, func(e BatchError) protocol.BatchResultErrorEntry {
			return protocol.BatchResultErrorEntry{ID: e.ID, Code: e.Code, Message: e.Message, SenderFault: e.SenderFault}
		})
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
		failed := convertBatchErrors(resp.BatchErrors, func(e BatchError) protocol.JSONBatchErrorEntry {
			return protocol.JSONBatchErrorEntry{ID: e.ID, Code: e.Code, Message: e.Message, SenderFault: e.SenderFault}
		})
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
		failed := convertBatchErrors(resp.BatchErrors, func(e BatchError) protocol.JSONBatchErrorEntry {
			return protocol.JSONBatchErrorEntry{ID: e.ID, Code: e.Code, Message: e.Message, SenderFault: e.SenderFault}
		})
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
		failed := convertBatchErrors(resp.BatchErrors, func(e BatchError) protocol.JSONBatchErrorEntry {
			return protocol.JSONBatchErrorEntry{ID: e.ID, Code: e.Code, Message: e.Message, SenderFault: e.SenderFault}
		})
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
