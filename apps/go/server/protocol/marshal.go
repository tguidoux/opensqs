package protocol

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// ---------------------------------------------------------------------------
// XML Response Types (Query Protocol)
// ---------------------------------------------------------------------------

// CreateQueueResponse is the XML response for CreateQueue.
type CreateQueueResponse struct {
	XMLName  xml.Name `xml:"CreateQueueResponse"`
	QueueURL string   `xml:"CreateQueueResult>QueueUrl"`
	ResponseMetadata
}

// DeleteQueueResponse is the XML response for DeleteQueue.
type DeleteQueueResponse struct {
	XMLName xml.Name `xml:"DeleteQueueResponse"`
	ResponseMetadata
}

// GetQueueURLResponse is the XML response for GetQueueUrl.
type GetQueueURLResponse struct {
	XMLName  xml.Name `xml:"GetQueueUrlResponse"`
	QueueURL string   `xml:"GetQueueUrlResult>QueueUrl"`
	ResponseMetadata
}

// ListQueuesResponse is the XML response for ListQueues.
type ListQueuesResponse struct {
	XMLName   xml.Name `xml:"ListQueuesResponse"`
	QueueURLs []string `xml:"ListQueuesResult>QueueUrl"`
	ResponseMetadata
}

// SendMessageResponse is the XML response for SendMessage.
type SendMessageResponse struct {
	XMLName                      xml.Name `xml:"SendMessageResponse"`
	MessageID                    string   `xml:"SendMessageResult>MessageId"`
	MD5OfMessageBody             string   `xml:"SendMessageResult>MD5OfMessageBody"`
	MD5OfMessageAttributes       string   `xml:"SendMessageResult>MD5OfMessageAttributes,omitempty"`
	MD5OfMessageSystemAttributes string   `xml:"SendMessageResult>MD5OfMessageSystemAttributes,omitempty"`
	SequenceNumber               string   `xml:"SendMessageResult>SequenceNumber,omitempty"`
	ResponseMetadata
}

// ReceiveMessageResponse is the XML response for ReceiveMessage.
type ReceiveMessageResponse struct {
	XMLName  xml.Name     `xml:"ReceiveMessageResponse"`
	Messages []XMLMessage `xml:"ReceiveMessageResult>Message"`
	ResponseMetadata
}

// XMLMessage represents a single message in an XML ReceiveMessage response.
type XMLMessage struct {
	MessageID                    string            `xml:"MessageId"`
	ReceiptHandle                string            `xml:"ReceiptHandle"`
	MD5OfBody                    string            `xml:"MD5OfBody"`
	Body                         string            `xml:"Body"`
	MD5OfMessageAttributes       string            `xml:"MD5OfMessageAttributes,omitempty"`
	MD5OfMessageSystemAttributes string            `xml:"MD5OfMessageSystemAttributes,omitempty"`
	SequenceNumber               string            `xml:"SequenceNumber,omitempty"`
	Attributes                   []XMLAttribute    `xml:"Attribute"`
	MessageAttributes            []XMLMsgAttribute `xml:"MessageAttribute"`
}

// XMLAttribute is a key-value attribute in XML responses.
type XMLAttribute struct {
	Name  string `xml:"Name"`
	Value string `xml:"Value"`
}

// XMLMsgAttribute is a message attribute in XML responses.
type XMLMsgAttribute struct {
	Name  string       `xml:"Name"`
	Value XMLAttrValue `xml:"Value"`
}

// XMLAttrValue is the value element of a message attribute.
type XMLAttrValue struct {
	DataType    string `xml:"DataType"`
	StringValue string `xml:"StringValue,omitempty"`
	BinaryValue string `xml:"BinaryValue,omitempty"`
}

// DeleteMessageResponse is the XML response for DeleteMessage.
type DeleteMessageResponse struct {
	XMLName xml.Name `xml:"DeleteMessageResponse"`
	ResponseMetadata
}

// ChangeMessageVisibilityResponse is the XML response for ChangeMessageVisibility.
type ChangeMessageVisibilityResponse struct {
	XMLName xml.Name `xml:"ChangeMessageVisibilityResponse"`
	ResponseMetadata
}

// GetQueueAttributesResponse is the XML response for GetQueueAttributes.
type GetQueueAttributesResponse struct {
	XMLName    xml.Name       `xml:"GetQueueAttributesResponse"`
	Attributes []XMLAttribute `xml:"GetQueueAttributesResult>Attribute"`
	ResponseMetadata
}

// SetQueueAttributesResponse is the XML response for SetQueueAttributes.
type SetQueueAttributesResponse struct {
	XMLName xml.Name `xml:"SetQueueAttributesResponse"`
	ResponseMetadata
}

// PurgeQueueResponse is the XML response for PurgeQueue.
type PurgeQueueResponse struct {
	XMLName xml.Name `xml:"PurgeQueueResponse"`
	ResponseMetadata
}

// SendMessageBatchResponse is the XML response for SendMessageBatch.
type SendMessageBatchResponse struct {
	XMLName xml.Name                      `xml:"SendMessageBatchResponse"`
	Entries []SendMessageBatchResultEntry `xml:"SendMessageBatchResult>SendMessageBatchResultEntry"`
	Errors  []BatchResultErrorEntry       `xml:"BatchResultErrorEntry"`
	ResponseMetadata
}

// SendMessageBatchResultEntry is a successful entry in a SendMessageBatch response.
type SendMessageBatchResultEntry struct {
	ID                           string `xml:"Id"`
	MessageID                    string `xml:"MessageId"`
	MD5OfMessageBody             string `xml:"MD5OfMessageBody"`
	MD5OfMessageAttributes       string `xml:"MD5OfMessageAttributes,omitempty"`
	MD5OfMessageSystemAttributes string `xml:"MD5OfMessageSystemAttributes,omitempty"`
	SequenceNumber               string `xml:"SequenceNumber,omitempty"`
}

// DeleteMessageBatchResponse is the XML response for DeleteMessageBatch.
type DeleteMessageBatchResponse struct {
	XMLName xml.Name                        `xml:"DeleteMessageBatchResponse"`
	Entries []DeleteMessageBatchResultEntry `xml:"DeleteMessageBatchResult>DeleteMessageBatchResultEntry"`
	Errors  []BatchResultErrorEntry         `xml:"BatchResultErrorEntry"`
	ResponseMetadata
}

// DeleteMessageBatchResultEntry is a successful entry in a DeleteMessageBatch response.
type DeleteMessageBatchResultEntry struct {
	ID string `xml:"Id"`
}

// ChangeMessageVisibilityBatchResponse is the XML response for ChangeMessageVisibilityBatch.
type ChangeMessageVisibilityBatchResponse struct {
	XMLName xml.Name                                  `xml:"ChangeMessageVisibilityBatchResponse"`
	Entries []ChangeMessageVisibilityBatchResultEntry `xml:"ChangeMessageVisibilityBatchResult>ChangeMessageVisibilityBatchResultEntry"`
	Errors  []BatchResultErrorEntry                   `xml:"BatchResultErrorEntry"`
	ResponseMetadata
}

// ChangeMessageVisibilityBatchResultEntry is a successful entry in a ChangeMessageVisibilityBatch response.
type ChangeMessageVisibilityBatchResultEntry struct {
	ID string `xml:"Id"`
}

// BatchResultErrorEntry is a failed entry in a batch response.
type BatchResultErrorEntry struct {
	ID          string `xml:"Id"`
	Code        string `xml:"Code"`
	Message     string `xml:"Message"`
	SenderFault bool   `xml:"SenderFault"`
}

// ListQueueTagsResponse is the XML response for ListQueueTags.
type ListQueueTagsResponse struct {
	XMLName xml.Name      `xml:"ListQueueTagsResponse"`
	Tags    []XMLTagEntry `xml:"ListQueueTagsResult>Tag"`
	ResponseMetadata
}

// XMLTagEntry is a key-value tag in XML responses.
type XMLTagEntry struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

// TagQueueResponse is the XML response for TagQueue.
type TagQueueResponse struct {
	XMLName xml.Name `xml:"TagQueueResponse"`
	ResponseMetadata
}

// UntagQueueResponse is the XML response for UntagQueue.
type UntagQueueResponse struct {
	XMLName xml.Name `xml:"UntagQueueResponse"`
	ResponseMetadata
}

// AddPermissionResponse is the XML response for AddPermission.
type AddPermissionResponse struct {
	XMLName xml.Name `xml:"AddPermissionResponse"`
	ResponseMetadata
}

// RemovePermissionResponse is the XML response for RemovePermission.
type RemovePermissionResponse struct {
	XMLName xml.Name `xml:"RemovePermissionResponse"`
	ResponseMetadata
}

// ListDeadLetterSourceQueuesResponse is the XML response for ListDeadLetterSourceQueues.
type ListDeadLetterSourceQueuesResponse struct {
	XMLName   xml.Name `xml:"ListDeadLetterSourceQueuesResponse"`
	QueueURLs []string `xml:"ListDeadLetterSourceQueuesResult>QueueUrl"`
	ResponseMetadata
}

// StartMessageMoveTaskResponse is the XML response for StartMessageMoveTask.
type StartMessageMoveTaskResponse struct {
	XMLName    xml.Name `xml:"StartMessageMoveTaskResponse"`
	TaskHandle string   `xml:"StartMessageMoveTaskResult>TaskHandle"`
	ResponseMetadata
}

// CancelMessageMoveTaskResponse is the XML response for CancelMessageMoveTask.
type CancelMessageMoveTaskResponse struct {
	XMLName       xml.Name `xml:"CancelMessageMoveTaskResponse"`
	MovedMessages int      `xml:"CancelMessageMoveTaskResult>ApproximateNumberOfMessagesMoved"`
	ResponseMetadata
}

// ListMessageMoveTasksResponse is the XML response for ListMessageMoveTasks.
type ListMessageMoveTasksResponse struct {
	XMLName xml.Name            `xml:"ListMessageMoveTasksResponse"`
	Results []XMLMoveTaskResult `xml:"ListMessageMoveTasksResult>Results>member"`
	ResponseMetadata
}

// XMLMoveTaskResult represents a single move task result in XML responses.
type XMLMoveTaskResult struct {
	TaskHandle                   string `xml:"TaskHandle"`
	SourceArn                    string `xml:"SourceArn"`
	DestinationArn               string `xml:"DestinationArn"`
	Status                       string `xml:"Status"`
	MaxNumberOfMessagesPerSecond int    `xml:"MaxNumberOfMessagesPerSecond,omitempty"`
	MovedMessages                int    `xml:"ApproximateNumberOfMessagesMoved"`
}

// ResponseMetadata contains the request ID.
type ResponseMetadata struct {
	RequestID string `xml:"ResponseMetadata>RequestId"`
}

// ---------------------------------------------------------------------------
// JSON Response Types (JSON Protocol)
// ---------------------------------------------------------------------------

// JSONCreateQueueResponse is the JSON response for CreateQueue.
type JSONCreateQueueResponse struct {
	QueueURL  string `json:"QueueUrl"`
	RequestID string `json:"RequestId"`
}

// JSONGetQueueURLResponse is the JSON response for GetQueueUrl.
type JSONGetQueueURLResponse struct {
	QueueURL  string `json:"QueueUrl"`
	RequestID string `json:"RequestId"`
}

// JSONListQueuesResponse is the JSON response for ListQueues.
type JSONListQueuesResponse struct {
	QueueURLs []string `json:"QueueUrls"`
	RequestID string   `json:"RequestId"`
}

// JSONSendMessageResponse is the JSON response for SendMessage.
type JSONSendMessageResponse struct {
	MessageID                    string `json:"MessageId"`
	MD5OfMessageBody             string `json:"MD5OfMessageBody"`
	MD5OfMessageAttributes       string `json:"MD5OfMessageAttributes,omitempty"`
	MD5OfMessageSystemAttributes string `json:"MD5OfMessageSystemAttributes,omitempty"`
	SequenceNumber               string `json:"SequenceNumber,omitempty"`
	RequestID                    string `json:"RequestId"`
}

// JSONReceiveMessageResponse is the JSON response for ReceiveMessage.
type JSONReceiveMessageResponse struct {
	Messages  []JSONMessage `json:"Messages,omitempty"`
	RequestID string        `json:"RequestId"`
}

// JSONMessage represents a single message in a JSON ReceiveMessage response.
type JSONMessage struct {
	MessageID                    string                      `json:"MessageId"`
	ReceiptHandle                string                      `json:"ReceiptHandle"`
	MD5OfBody                    string                      `json:"MD5OfBody"`
	Body                         string                      `json:"Body"`
	MD5OfMessageAttributes       string                      `json:"MD5OfMessageAttributes,omitempty"`
	MD5OfMessageSystemAttributes string                      `json:"MD5OfMessageSystemAttributes,omitempty"`
	SequenceNumber               string                      `json:"SequenceNumber,omitempty"`
	Attributes                   map[string]string           `json:"Attributes,omitempty"`
	MessageAttributes            map[string]JSONMsgAttribute `json:"MessageAttributes,omitempty"`
}

// JSONMsgAttribute is a message attribute in JSON responses.
type JSONMsgAttribute struct {
	DataType    string `json:"DataType"`
	StringValue string `json:"StringValue,omitempty"`
	BinaryValue string `json:"BinaryValue,omitempty"`
}

// JSONDeleteMessageResponse is the JSON response for DeleteMessage.
type JSONDeleteMessageResponse struct {
	RequestID string `json:"RequestId"`
}

// JSONChangeMessageVisibilityResponse is the JSON response for ChangeMessageVisibility.
type JSONChangeMessageVisibilityResponse struct {
	RequestID string `json:"RequestId"`
}

// JSONGetQueueAttributesResponse is the JSON response for GetQueueAttributes.
type JSONGetQueueAttributesResponse struct {
	Attributes map[string]string `json:"Attributes"`
	RequestID  string            `json:"RequestId"`
}

// JSONSetQueueAttributesResponse is the JSON response for SetQueueAttributes.
type JSONSetQueueAttributesResponse struct {
	RequestID string `json:"RequestId"`
}

// JSONPurgeQueueResponse is the JSON response for PurgeQueue.
type JSONPurgeQueueResponse struct {
	RequestID string `json:"RequestId"`
}

// JSONDeleteQueueResponse is the JSON response for DeleteQueue.
type JSONDeleteQueueResponse struct {
	RequestID string `json:"RequestId"`
}

// JSONSendMessageBatchResponse is the JSON response for SendMessageBatch.
type JSONSendMessageBatchResponse struct {
	Successful []JSONBatchResultEntry `json:"Successful,omitempty"`
	Failed     []JSONBatchErrorEntry  `json:"Failed,omitempty"`
	RequestID  string                 `json:"RequestId"`
}

// JSONDeleteMessageBatchResponse is the JSON response for DeleteMessageBatch.
type JSONDeleteMessageBatchResponse struct {
	Successful []JSONBatchResultEntry `json:"Successful,omitempty"`
	Failed     []JSONBatchErrorEntry  `json:"Failed,omitempty"`
	RequestID  string                 `json:"RequestId"`
}

// JSONChangeMessageVisibilityBatchResponse is the JSON response for ChangeMessageVisibilityBatch.
type JSONChangeMessageVisibilityBatchResponse struct {
	Successful []JSONBatchResultEntry `json:"Successful,omitempty"`
	Failed     []JSONBatchErrorEntry  `json:"Failed,omitempty"`
	RequestID  string                 `json:"RequestId"`
}

// JSONBatchResultEntry is a successful entry in a JSON batch response.
type JSONBatchResultEntry struct {
	ID                           string `json:"Id"`
	MessageID                    string `json:"MessageId,omitempty"`
	MD5OfMessageBody             string `json:"MD5OfMessageBody,omitempty"`
	MD5OfMessageAttributes       string `json:"MD5OfMessageAttributes,omitempty"`
	MD5OfMessageSystemAttributes string `json:"MD5OfMessageSystemAttributes,omitempty"`
	SequenceNumber               string `json:"SequenceNumber,omitempty"`
}

// JSONBatchErrorEntry is a failed entry in a JSON batch response.
type JSONBatchErrorEntry struct {
	ID          string `json:"Id"`
	Code        string `json:"Code"`
	Message     string `json:"Message"`
	SenderFault bool   `json:"SenderFault"`
}

// JSONListQueueTagsResponse is the JSON response for ListQueueTags.
type JSONListQueueTagsResponse struct {
	Tags      map[string]string `json:"Tags,omitempty"`
	RequestID string            `json:"RequestId"`
}

// JSONTagQueueResponse is the JSON response for TagQueue.
type JSONTagQueueResponse struct {
	RequestID string `json:"RequestId"`
}

// JSONUntagQueueResponse is the JSON response for UntagQueue.
type JSONUntagQueueResponse struct {
	RequestID string `json:"RequestId"`
}

// JSONAddPermissionResponse is the JSON response for AddPermission.
type JSONAddPermissionResponse struct {
	RequestID string `json:"RequestId"`
}

// JSONRemovePermissionResponse is the JSON response for RemovePermission.
type JSONRemovePermissionResponse struct {
	RequestID string `json:"RequestId"`
}

// JSONListDeadLetterSourceQueuesResponse is the JSON response for ListDeadLetterSourceQueues.
type JSONListDeadLetterSourceQueuesResponse struct {
	QueueURLs []string `json:"queueUrls"`
	RequestID string   `json:"RequestId"`
}

// JSONStartMessageMoveTaskResponse is the JSON response for StartMessageMoveTask.
type JSONStartMessageMoveTaskResponse struct {
	TaskHandle string `json:"TaskHandle"`
	RequestID  string `json:"RequestId"`
}

// JSONCancelMessageMoveTaskResponse is the JSON response for CancelMessageMoveTask.
type JSONCancelMessageMoveTaskResponse struct {
	MovedMessages int    `json:"ApproximateNumberOfMessagesMoved"`
	RequestID     string `json:"RequestId"`
}

// JSONListMessageMoveTasksResponse is the JSON response for ListMessageMoveTasks.
type JSONListMessageMoveTasksResponse struct {
	Results   []JSONMoveTaskResult `json:"Results"`
	RequestID string               `json:"RequestId"`
}

// JSONMoveTaskResult represents a single move task result in JSON responses.
type JSONMoveTaskResult struct {
	TaskHandle                   string `json:"TaskHandle"`
	SourceArn                    string `json:"SourceArn"`
	DestinationArn               string `json:"DestinationArn"`
	Status                       string `json:"Status"`
	MaxNumberOfMessagesPerSecond int    `json:"MaxNumberOfMessagesPerSecond,omitempty"`
	MovedMessages                int    `json:"ApproximateNumberOfMessagesMoved"`
}

// ---------------------------------------------------------------------------
// Marshalling Functions
// ---------------------------------------------------------------------------

// MarshalXMLResponse marshals an XML response struct to XML bytes.
func MarshalXMLResponse(v any) ([]byte, error) {
	data, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal XML response: %w", err)
	}
	return append([]byte(xml.Header), data...), nil
}

// jsonMarshal is a helper to marshal JSON without HTML escaping.
func jsonMarshal(v any) ([]byte, error) {
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// Trim trailing newline added by Encode
	result := buf.String()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return []byte(result), nil
}

// MarshalJSONResponse marshals a JSON response struct to JSON bytes.
func MarshalJSONResponse(v any) ([]byte, error) {
	return jsonMarshal(v)
}

// NewRequestID generates a request ID placeholder.
// In production, this should be a UUID.
func NewRequestID() string {
	return types.EmptyRequestID
}

// Ensure fmt import is used
var _ = fmt.Sprintf
