package queue

import (
	"fmt"

	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// SQSError is an alias for types.ConcreteSQSError for backward compatibility.
type SQSError = types.ConcreteSQSError

// Error factory functions for common SQS errors.

func NewInvalidAction(msg string) *SQSError {
	if msg == "" {
		msg = "The action is not valid for this endpoint."
	}
	return &SQSError{
		CodeValue:         "InvalidAction",
		HTTPStatusValue:   400,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}

func NewQueueDoesNotExist(msg string) *SQSError {
	if msg == "" {
		msg = "The specified queue does not exist."
	}
	return &SQSError{
		CodeValue:         "AWS.SimpleQueueService.NonExistentQueue",
		HTTPStatusValue:   400,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}

func NewInvalidParameterValue(msg string) *SQSError {
	return &SQSError{
		CodeValue:         "InvalidParameterValue",
		HTTPStatusValue:   400,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}

func NewInvalidAttributeName(msg string) *SQSError {
	return &SQSError{
		CodeValue:         "InvalidAttributeName",
		HTTPStatusValue:   400,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}

func NewInvalidQueryParameter(msg string) *SQSError {
	return &SQSError{
		CodeValue:         "InvalidQueryParameter",
		HTTPStatusValue:   400,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}

func NewMissingParameter(msg string) *SQSError {
	return &SQSError{
		CodeValue:         "MissingParameter",
		HTTPStatusValue:   400,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}

func NewQueueNameExists(msg string) *SQSError {
	if msg == "" {
		msg = "A queue already exists with the same name and a different value for attribute."
	}
	return &SQSError{
		CodeValue:         "QueueAlreadyExists",
		HTTPStatusValue:   400,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}

func NewTooManyEntriesInBatchRequest(msg string) *SQSError {
	return &SQSError{
		CodeValue:         "AWS.SimpleQueueService.TooManyEntriesInBatch",
		HTTPStatusValue:   400,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}

func NewBatchEntryIdsNotDistinct(msg string) *SQSError {
	return &SQSError{
		CodeValue:         "AWS.SimpleQueueService.BatchEntryIdsNotDistinct",
		HTTPStatusValue:   400,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}

func NewInvalidMessageContents(msg string) *SQSError {
	return &SQSError{
		CodeValue:         "InvalidMessageContents",
		HTTPStatusValue:   400,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}

func NewInvalidAttributeValue(msg string) *SQSError {
	return &SQSError{
		CodeValue:         "InvalidAttributeValue",
		HTTPStatusValue:   400,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}

func NewReceiptHandleIsInvalid(msg string) *SQSError {
	return &SQSError{
		CodeValue:         "ReceiptHandleIsInvalid",
		HTTPStatusValue:   400,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}

func NewOverLimit(msg string) *SQSError {
	return &SQSError{
		CodeValue:         "OverLimit",
		HTTPStatusValue:   403,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}

func NewInternalError(msg string) *SQSError {
	if msg == "" {
		msg = "Internal error."
	}
	return &SQSError{
		CodeValue:         "InternalError",
		HTTPStatusValue:   500,
		ErrorTypeValue:    "Receiver",
		ErrorMessageValue: msg,
	}
}

func NewInvalidMessageID(msg string) *SQSError {
	return &SQSError{
		CodeValue:         "InvalidMessageId",
		HTTPStatusValue:   400,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}

func NewUnsupportedOperation(msg string) *SQSError {
	return &SQSError{
		CodeValue:         "UnsupportedOperation",
		HTTPStatusValue:   400,
		ErrorTypeValue:    "Sender",
		ErrorMessageValue: msg,
	}
}

// Ensure the fmt import is used.
var _ = fmt.Sprintf
