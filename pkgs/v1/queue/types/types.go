package types

import (
	"fmt"
	"time"
)

// Message represents an SQS message stored in a queue.
type Message struct {
	MessageID                        string
	ReceiptHandle                    string
	MD5OfBody                        string
	MD5OfMessageAttributes           string
	Body                             string
	MessageAttributes                map[string]MessageAttribute
	Attributes                       map[string]string
	MD5OfMessageSystemAttributes     string
	SystemAttributes                 map[string]MessageSystemAttribute
	SentTimestamp                    time.Time
	ReceivedTimestamp                time.Time
	FirstReceivedTimestamp           time.Time
	ApproximateReceiveCount          int
	ApproximateFirstReceiveTimestamp time.Time
	SequenceNumber                   string
	MessageDeduplicationID           string
	MessageGroupID                   string
	VisibilityDeadline               time.Time
	IsVisible                        bool
}

// MessageAttribute represents an SQS message attribute (user-defined).
type MessageAttribute struct {
	DataType    string
	StringValue string
	BinaryValue []byte
}

// MessageSystemAttribute represents an SQS message system attribute (AWS-defined).
type MessageSystemAttribute struct {
	DataType    string
	StringValue string
	BinaryValue []byte
}

// ReceiptHandleInfo contains the internal data encoded in a receipt handle.
type ReceiptHandleInfo struct {
	QueueName        string
	MessageID        string
	ReceiveTimestamp time.Time
	RandomNonce      string
}

// QueueAttributes is a map of attribute name to value.
type QueueAttributes map[string]string

// SQSError represents an SQS API error with all metadata needed for responses.
type SQSError interface {
	error
	Code() string
	HTTPStatusCode() int
	ErrorType() string
	Message() string
}

// ConcreteSQSError implements the SQSError interface.
type ConcreteSQSError struct {
	CodeValue         string
	HTTPStatusValue   int
	ErrorTypeValue    string
	ErrorMessageValue string
}

func (e *ConcreteSQSError) Error() string {
	return fmt.Sprintf("%s: %s", e.CodeValue, e.ErrorMessageValue)
}

func (e *ConcreteSQSError) Code() string        { return e.CodeValue }
func (e *ConcreteSQSError) HTTPStatusCode() int { return e.HTTPStatusValue }
func (e *ConcreteSQSError) ErrorType() string   { return e.ErrorTypeValue }
func (e *ConcreteSQSError) Message() string     { return e.ErrorMessageValue }

// Error factory functions for common SQS errors.

func NewReceiptHandleIsInvalid(msg string) *ConcreteSQSError {
	return &ConcreteSQSError{
		CodeValue:         "ReceiptHandleIsInvalid",
		HTTPStatusValue:   400,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}

func NewInvalidParameterValue(msg string) *ConcreteSQSError {
	return &ConcreteSQSError{
		CodeValue:         "InvalidParameterValue",
		HTTPStatusValue:   400,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}
