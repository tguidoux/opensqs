package tests

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/apps/go/server/protocol"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// ---------------------------------------------------------------------------
// Query Protocol Parser Tests
// ---------------------------------------------------------------------------

func TestParseQueryRequest_CreateQueue(t *testing.T) {
	body := "Action=CreateQueue&QueueName=my-queue&Attribute.1.Name=VisibilityTimeout&Attribute.1.Value=30"

	req, err := protocol.ParseQueryRequest(body)
	require.NoError(t, err)

	assert.Equal(t, "CreateQueue", req.GetAction())
	assert.Equal(t, "my-queue", req.GetQueueName())
	assert.Equal(t, "30", req.Attributes["VisibilityTimeout"])
}

func TestParseQueryRequest_SendMessage(t *testing.T) {
	body := "Action=SendMessage&QueueUrl=http://localhost/12345/my-queue&MessageBody=hello+world&DelaySeconds=5"

	req, err := protocol.ParseQueryRequest(body)
	require.NoError(t, err)

	assert.Equal(t, "SendMessage", req.GetAction())
	assert.Equal(t, "http://localhost/12345/my-queue", req.GetQueueURL())
	assert.Equal(t, "hello world", req.GetMessageBody())
	assert.Equal(t, 5, req.GetDelaySeconds())
}

func TestParseQueryRequest_ReceiveMessage(t *testing.T) {
	body := "Action=ReceiveMessage&QueueUrl=http://localhost/12345/my-queue&MaxNumberOfMessages=5&VisibilityTimeout=60&WaitTimeSeconds=10"

	req, err := protocol.ParseQueryRequest(body)
	require.NoError(t, err)

	assert.Equal(t, "ReceiveMessage", req.GetAction())
	assert.Equal(t, 5, req.GetMaxNumberOfMessages())
	assert.Equal(t, 60, req.GetVisibilityTimeout())
	assert.Equal(t, 10, req.GetWaitTimeSeconds())
}

func TestParseQueryRequest_DeleteMessage(t *testing.T) {
	body := "Action=DeleteMessage&QueueUrl=http://localhost/12345/my-queue&ReceiptHandle=abc123"

	req, err := protocol.ParseQueryRequest(body)
	require.NoError(t, err)

	assert.Equal(t, "DeleteMessage", req.GetAction())
	assert.Equal(t, "abc123", req.GetReceiptHandle())
}

func TestParseQueryRequest_ListQueues(t *testing.T) {
	body := "Action=ListQueues&Prefix=my"

	req, err := protocol.ParseQueryRequest(body)
	require.NoError(t, err)

	assert.Equal(t, "ListQueues", req.GetAction())
	assert.Equal(t, "my", req.GetPrefix())
}

func TestParseQueryRequest_GetQueueAttributes(t *testing.T) {
	body := "Action=GetQueueAttributes&QueueUrl=http://localhost/12345/my-queue&AttributeName.1=VisibilityTimeout&AttributeName.2=QueueArn"

	req, err := protocol.ParseQueryRequest(body)
	require.NoError(t, err)

	assert.Equal(t, "GetQueueAttributes", req.GetAction())
	names := req.GetAttributeNames()
	assert.Len(t, names, 2)
	assert.Equal(t, "VisibilityTimeout", names[0])
	assert.Equal(t, "QueueArn", names[1])
}

func TestParseQueryRequest_DefaultValues(t *testing.T) {
	body := "Action=ReceiveMessage&QueueUrl=http://localhost/12345/my-queue"

	req, err := protocol.ParseQueryRequest(body)
	require.NoError(t, err)

	assert.Equal(t, 1, req.GetMaxNumberOfMessages())
	assert.Equal(t, -1, req.GetVisibilityTimeout())
	assert.Equal(t, -1, req.GetWaitTimeSeconds())
	assert.Equal(t, 0, req.GetDelaySeconds())
}

func TestParseQueryRequest_BatchEntries(t *testing.T) {
	body := "Action=SendMessageBatch&QueueUrl=http://localhost/12345/my-queue" +
		"&SendMessageBatchRequestEntry.1.Id=msg1&SendMessageBatchRequestEntry.1.MessageBody=hello" +
		"&SendMessageBatchRequestEntry.1.DelaySeconds=3" +
		"&SendMessageBatchRequestEntry.2.Id=msg2&SendMessageBatchRequestEntry.2.MessageBody=world"

	req, err := protocol.ParseQueryRequest(body)
	require.NoError(t, err)

	assert.Equal(t, "SendMessageBatch", req.GetAction())
	require.Len(t, req.BatchEntries, 2)

	assert.Equal(t, "msg1", req.BatchEntries[0].ID)
	assert.Equal(t, "hello", req.BatchEntries[0].MessageBody)
	assert.Equal(t, 3, req.BatchEntries[0].DelaySeconds)

	assert.Equal(t, "msg2", req.BatchEntries[1].ID)
	assert.Equal(t, "world", req.BatchEntries[1].MessageBody)
}

func TestParseQueryRequest_EmptyBody(t *testing.T) {
	req, err := protocol.ParseQueryRequest("")
	require.NoError(t, err)

	assert.Equal(t, "", req.GetAction())
}

// ---------------------------------------------------------------------------
// JSON Protocol Parser Tests
// ---------------------------------------------------------------------------

func TestParseJSONRequest_CreateQueue(t *testing.T) {
	body := `{"QueueName": "my-queue", "Attributes": {"VisibilityTimeout": "30"}}`

	req, err := protocol.ParseJSONRequest("AmazonSQS.CreateQueue", []byte(body))
	require.NoError(t, err)

	assert.Equal(t, "CreateQueue", req.GetAction())
	assert.Equal(t, "my-queue", req.GetQueueName())

	attrs := req.GetAttributes()
	assert.Equal(t, "30", attrs["VisibilityTimeout"])
}

func TestParseJSONRequest_SendMessage(t *testing.T) {
	body := `{"QueueUrl": "http://localhost/12345/my-queue", "MessageBody": "hello world", "DelaySeconds": 5}`

	req, err := protocol.ParseJSONRequest("AmazonSQS.SendMessage", []byte(body))
	require.NoError(t, err)

	assert.Equal(t, "SendMessage", req.GetAction())
	assert.Equal(t, "http://localhost/12345/my-queue", req.GetQueueURL())
	assert.Equal(t, "hello world", req.GetMessageBody())
	assert.Equal(t, 5, req.GetDelaySeconds())
}

func TestParseJSONRequest_ReceiveMessage(t *testing.T) {
	body := `{"QueueUrl": "http://localhost/12345/my-queue", "MaxNumberOfMessages": 5, "VisibilityTimeout": 60, "WaitTimeSeconds": 10}`

	req, err := protocol.ParseJSONRequest("AmazonSQS.ReceiveMessage", []byte(body))
	require.NoError(t, err)

	assert.Equal(t, "ReceiveMessage", req.GetAction())
	assert.Equal(t, 5, req.GetMaxNumberOfMessages())
	assert.Equal(t, 60, req.GetVisibilityTimeout())
	assert.Equal(t, 10, req.GetWaitTimeSeconds())
}

func TestParseJSONRequest_DeleteMessage(t *testing.T) {
	body := `{"QueueUrl": "http://localhost/12345/my-queue", "ReceiptHandle": "abc123"}`

	req, err := protocol.ParseJSONRequest("AmazonSQS.DeleteMessage", []byte(body))
	require.NoError(t, err)

	assert.Equal(t, "DeleteMessage", req.GetAction())
	assert.Equal(t, "abc123", req.GetReceiptHandle())
}

func TestParseJSONRequest_ListQueues(t *testing.T) {
	body := `{"Prefix": "my"}`

	req, err := protocol.ParseJSONRequest("AmazonSQS.ListQueues", []byte(body))
	require.NoError(t, err)

	assert.Equal(t, "ListQueues", req.GetAction())
	assert.Equal(t, "my", req.GetPrefix())
}

func TestParseJSONRequest_GetQueueAttributes(t *testing.T) {
	body := `{"QueueUrl": "http://localhost/12345/my-queue", "AttributeNames": ["VisibilityTimeout", "QueueArn"]}`

	req, err := protocol.ParseJSONRequest("AmazonSQS.GetQueueAttributes", []byte(body))
	require.NoError(t, err)

	assert.Equal(t, "GetQueueAttributes", req.GetAction())
	names := req.GetAttributeNames()
	assert.Len(t, names, 2)
	assert.Equal(t, "VisibilityTimeout", names[0])
	assert.Equal(t, "QueueArn", names[1])
}

func TestParseJSONRequest_DefaultValues(t *testing.T) {
	body := `{"QueueUrl": "http://localhost/12345/my-queue"}`

	req, err := protocol.ParseJSONRequest("AmazonSQS.ReceiveMessage", []byte(body))
	require.NoError(t, err)

	assert.Equal(t, 1, req.GetMaxNumberOfMessages())
	assert.Equal(t, -1, req.GetVisibilityTimeout())
	assert.Equal(t, -1, req.GetWaitTimeSeconds())
}

func TestParseJSONRequest_BatchEntries(t *testing.T) {
	body := `{
		"QueueUrl": "http://localhost/12345/my-queue",
		"Entries": [
			{"Id": "msg1", "MessageBody": "hello", "DelaySeconds": 3},
			{"Id": "msg2", "MessageBody": "world"}
		]
	}`

	req, err := protocol.ParseJSONRequest("AmazonSQS.SendMessageBatch", []byte(body))
	require.NoError(t, err)

	assert.Equal(t, "SendMessageBatch", req.GetAction())
	entries := req.GetBatchEntries()
	require.Len(t, entries, 2)

	assert.Equal(t, "msg1", entries[0].ID)
	assert.Equal(t, "hello", entries[0].MessageBody)
	assert.Equal(t, 3, entries[0].DelaySeconds)

	assert.Equal(t, "msg2", entries[1].ID)
	assert.Equal(t, "world", entries[1].MessageBody)
}

func TestParseJSONRequest_EmptyBody(t *testing.T) {
	req, err := protocol.ParseJSONRequest("AmazonSQS.ListQueues", []byte(""))
	require.NoError(t, err)

	assert.Equal(t, "ListQueues", req.GetAction())
}

func TestParseJSONRequest_InvalidJSON(t *testing.T) {
	_, err := protocol.ParseJSONRequest("AmazonSQS.CreateQueue", []byte("{invalid json"))
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Error Response Tests
// ---------------------------------------------------------------------------

func TestNewErrorResponse(t *testing.T) {
	sqsErr := types.NewInvalidParameterValue("test error")
	resp := protocol.NewErrorResponse(sqsErr, "req-123")

	assert.Equal(t, "InvalidParameterValue", resp.Code)
	assert.Equal(t, "test error", resp.Message)
	assert.Equal(t, "Sender", resp.Type)
	assert.Equal(t, "req-123", resp.RequestID)
	assert.Equal(t, 400, resp.HTTPStatus)
}

func TestNewErrorResponse_EmptyRequestID(t *testing.T) {
	sqsErr := types.NewInvalidParameterValue("test error")
	resp := protocol.NewErrorResponse(sqsErr, "")

	assert.Equal(t, types.EmptyRequestID, resp.RequestID)
}

func TestErrorResponse_ToXML(t *testing.T) {
	sqsErr := types.NewInvalidParameterValue("test error")
	resp := protocol.NewErrorResponse(sqsErr, "req-123")

	data, err := resp.ToXML()
	require.NoError(t, err)

	xmlStr := string(data)
	assert.Contains(t, xmlStr, `<?xml version="1.0" encoding="UTF-8"?>`)
	assert.Contains(t, xmlStr, "<ErrorResponse>")
	assert.Contains(t, xmlStr, "<Code>InvalidParameterValue</Code>")
	assert.Contains(t, xmlStr, "<Message>test error</Message>")
	assert.Contains(t, xmlStr, "<Type>Sender</Type>")
	assert.Contains(t, xmlStr, "<RequestId>req-123</RequestId>")
}

func TestErrorResponse_ToJSON(t *testing.T) {
	sqsErr := types.NewInvalidParameterValue("test error")
	resp := protocol.NewErrorResponse(sqsErr, "req-123")

	data, err := resp.ToJSON()
	require.NoError(t, err)

	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"__type"`)
	assert.Contains(t, jsonStr, "InvalidParameterValue")
	assert.Contains(t, jsonStr, "test error")
	assert.Contains(t, jsonStr, "req-123")
}

// ---------------------------------------------------------------------------
// XML Response Marshalling Tests
// ---------------------------------------------------------------------------

func TestMarshalXMLResponse_CreateQueue(t *testing.T) {
	resp := protocol.CreateQueueResponse{
		QueueURL: "http://localhost/12345/my-queue",
		ResponseMetadata: protocol.ResponseMetadata{
			RequestID: "req-123",
		},
	}

	data, err := protocol.MarshalXMLResponse(resp)
	require.NoError(t, err)

	xmlStr := string(data)
	assert.Contains(t, xmlStr, "<CreateQueueResponse>")
	assert.Contains(t, xmlStr, "<QueueUrl>http://localhost/12345/my-queue</QueueUrl>")
	assert.Contains(t, xmlStr, "<RequestId>req-123</RequestId>")
}

func TestMarshalXMLResponse_ListQueues(t *testing.T) {
	resp := protocol.ListQueuesResponse{
		QueueURLs: []string{
			"http://localhost/12345/q1",
			"http://localhost/12345/q2",
		},
		ResponseMetadata: protocol.ResponseMetadata{
			RequestID: "req-456",
		},
	}

	data, err := protocol.MarshalXMLResponse(resp)
	require.NoError(t, err)

	xmlStr := string(data)
	assert.Contains(t, xmlStr, "<ListQueuesResponse>")
	assert.Contains(t, xmlStr, "<QueueUrl>http://localhost/12345/q1</QueueUrl>")
	assert.Contains(t, xmlStr, "<QueueUrl>http://localhost/12345/q2</QueueUrl>")
}

func TestMarshalXMLResponse_SendMessage(t *testing.T) {
	resp := protocol.SendMessageResponse{
		MessageID:        "msg-123",
		MD5OfMessageBody: "d41d8cd98f00b204e9800998ecf8427e",
		ResponseMetadata: protocol.ResponseMetadata{
			RequestID: "req-789",
		},
	}

	data, err := protocol.MarshalXMLResponse(resp)
	require.NoError(t, err)

	xmlStr := string(data)
	assert.Contains(t, xmlStr, "<SendMessageResponse>")
	assert.Contains(t, xmlStr, "<MessageId>msg-123</MessageId>")
	assert.Contains(t, xmlStr, "<MD5OfMessageBody>d41d8cd98f00b204e9800998ecf8427e</MD5OfMessageBody>")
}

func TestMarshalXMLResponse_ReceiveMessage(t *testing.T) {
	resp := protocol.ReceiveMessageResponse{
		Messages: []protocol.XMLMessage{
			{
				MessageID:     "msg-123",
				ReceiptHandle: "handle-abc",
				MD5OfBody:     "d41d8cd98f00b204e9800998ecf8427e",
				Body:          "hello world",
				Attributes: []protocol.XMLAttribute{
					{Name: "SentTimestamp", Value: "1234567890"},
				},
			},
		},
		ResponseMetadata: protocol.ResponseMetadata{
			RequestID: "req-abc",
		},
	}

	data, err := protocol.MarshalXMLResponse(resp)
	require.NoError(t, err)

	xmlStr := string(data)
	assert.Contains(t, xmlStr, "<ReceiveMessageResponse>")
	assert.Contains(t, xmlStr, "<MessageId>msg-123</MessageId>")
	assert.Contains(t, xmlStr, "<ReceiptHandle>handle-abc</ReceiptHandle>")
	assert.Contains(t, xmlStr, "<Body>hello world</Body>")
	assert.Contains(t, xmlStr, "<Name>SentTimestamp</Name>")
}

func TestMarshalXMLResponse_GetQueueAttributes(t *testing.T) {
	resp := protocol.GetQueueAttributesResponse{
		Attributes: []protocol.XMLAttribute{
			{Name: "VisibilityTimeout", Value: "30"},
			{Name: "QueueArn", Value: "arn:aws:sqs:us-east-1:12345:my-queue"},
		},
		ResponseMetadata: protocol.ResponseMetadata{
			RequestID: "req-def",
		},
	}

	data, err := protocol.MarshalXMLResponse(resp)
	require.NoError(t, err)

	xmlStr := string(data)
	assert.Contains(t, xmlStr, "<GetQueueAttributesResponse>")
	assert.Contains(t, xmlStr, "<Name>VisibilityTimeout</Name>")
	assert.Contains(t, xmlStr, "<Value>30</Value>")
	assert.Contains(t, xmlStr, "arn:aws:sqs:us-east-1:12345:my-queue")
}

func TestMarshalXMLResponse_EmptyListQueues(t *testing.T) {
	resp := protocol.ListQueuesResponse{
		QueueURLs: nil,
		ResponseMetadata: protocol.ResponseMetadata{
			RequestID: "req-empty",
		},
	}

	data, err := protocol.MarshalXMLResponse(resp)
	require.NoError(t, err)

	xmlStr := string(data)
	assert.Contains(t, xmlStr, "<ListQueuesResponse>")
	// Should still have the result element, just no QueueUrl children
	var parsed protocol.ListQueuesResponse
	err = xml.Unmarshal(data, &parsed)
	require.NoError(t, err)
	assert.Empty(t, parsed.QueueURLs)
}

// ---------------------------------------------------------------------------
// JSON Response Marshalling Tests
// ---------------------------------------------------------------------------

func TestMarshalJSONResponse_CreateQueue(t *testing.T) {
	resp := protocol.JSONCreateQueueResponse{
		QueueURL:  "http://localhost/12345/my-queue",
		RequestID: "req-123",
	}

	data, err := protocol.MarshalJSONResponse(resp)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"QueueUrl"`)
	assert.Contains(t, jsonStr, "http://localhost/12345/my-queue")
	assert.Contains(t, jsonStr, `"RequestId"`)
	assert.Contains(t, jsonStr, "req-123")
}

func TestMarshalJSONResponse_ListQueues(t *testing.T) {
	resp := protocol.JSONListQueuesResponse{
		QueueURLs: []string{
			"http://localhost/12345/q1",
			"http://localhost/12345/q2",
		},
		RequestID: "req-456",
	}

	data, err := protocol.MarshalJSONResponse(resp)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"QueueUrls"`)
	assert.Contains(t, jsonStr, "http://localhost/12345/q1")
	assert.Contains(t, jsonStr, "http://localhost/12345/q2")
}

func TestMarshalJSONResponse_SendMessage(t *testing.T) {
	resp := protocol.JSONSendMessageResponse{
		MessageID:        "msg-123",
		MD5OfMessageBody: "d41d8cd98f00b204e9800998ecf8427e",
		RequestID:        "req-789",
	}

	data, err := protocol.MarshalJSONResponse(resp)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"MessageId"`)
	assert.Contains(t, jsonStr, "msg-123")
	assert.Contains(t, jsonStr, `"MD5OfMessageBody"`)
}

func TestMarshalJSONResponse_ReceiveMessage(t *testing.T) {
	resp := protocol.JSONReceiveMessageResponse{
		Messages: []protocol.JSONMessage{
			{
				MessageID:     "msg-123",
				ReceiptHandle: "handle-abc",
				MD5OfBody:     "d41d8cd98f00b204e9800998ecf8427e",
				Body:          "hello world",
				Attributes: map[string]string{
					"SentTimestamp": "1234567890",
				},
			},
		},
		RequestID: "req-abc",
	}

	data, err := protocol.MarshalJSONResponse(resp)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"MessageId"`)
	assert.Contains(t, jsonStr, "msg-123")
	assert.Contains(t, jsonStr, `"ReceiptHandle"`)
	assert.Contains(t, jsonStr, "handle-abc")
	assert.Contains(t, jsonStr, `"Body"`)
	assert.Contains(t, jsonStr, "hello world")
}

func TestMarshalJSONResponse_GetQueueAttributes(t *testing.T) {
	resp := protocol.JSONGetQueueAttributesResponse{
		Attributes: map[string]string{
			"VisibilityTimeout": "30",
			"QueueArn":          "arn:aws:sqs:us-east-1:12345:my-queue",
		},
		RequestID: "req-def",
	}

	data, err := protocol.MarshalJSONResponse(resp)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"Attributes"`)
	assert.Contains(t, jsonStr, "VisibilityTimeout")
	assert.Contains(t, jsonStr, "30")
}

func TestMarshalJSONResponse_NoHTMLEscape(t *testing.T) {
	resp := protocol.JSONCreateQueueResponse{
		QueueURL:  "http://localhost/12345/my-queue?param=value&other=2",
		RequestID: "req-123",
	}

	data, err := protocol.MarshalJSONResponse(resp)
	require.NoError(t, err)

	jsonStr := string(data)
	// Should NOT have HTML-escaped ampersands
	assert.False(t, strings.Contains(jsonStr, `\u0026`))
	assert.True(t, strings.Contains(jsonStr, "&"))
}

// ---------------------------------------------------------------------------
// Round-trip XML Tests
// ---------------------------------------------------------------------------

func TestXMLRoundTrip_CreateQueue(t *testing.T) {
	original := protocol.CreateQueueResponse{
		QueueURL: "http://localhost/12345/my-queue",
		ResponseMetadata: protocol.ResponseMetadata{
			RequestID: "req-123",
		},
	}

	data, err := protocol.MarshalXMLResponse(original)
	require.NoError(t, err)

	var parsed protocol.CreateQueueResponse
	err = xml.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, original.QueueURL, parsed.QueueURL)
	assert.Equal(t, original.RequestID, parsed.RequestID)
}

func TestXMLRoundTrip_SendMessage(t *testing.T) {
	original := protocol.SendMessageResponse{
		MessageID:        "msg-123",
		MD5OfMessageBody: "d41d8cd98f00b204e9800998ecf8427e",
		ResponseMetadata: protocol.ResponseMetadata{
			RequestID: "req-789",
		},
	}

	data, err := protocol.MarshalXMLResponse(original)
	require.NoError(t, err)

	var parsed protocol.SendMessageResponse
	err = xml.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, original.MessageID, parsed.MessageID)
	assert.Equal(t, original.MD5OfMessageBody, parsed.MD5OfMessageBody)
	assert.Equal(t, original.RequestID, parsed.RequestID)
}
