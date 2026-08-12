package protocol

import (
	"encoding/xml"
	"fmt"

	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// SQSErrorResponse represents an SQS error response that can be serialized
// to both XML (Query Protocol) and JSON (JSON Protocol).
type SQSErrorResponse struct {
	XMLName    xml.Name `xml:"ErrorResponse" json:"-"`
	Type       string   `xml:"Type" json:"Type"`
	Code       string   `xml:"Code" json:"Code"`
	Message    string   `xml:"Message" json:"Message"`
	RequestID  string   `xml:"RequestId" json:"RequestId"`
	HTTPStatus int      `xml:"-" json:"-"`
}

// NewErrorResponse creates an SQSErrorResponse from an SQSError.
func NewErrorResponse(err types.SQSError, requestID string) *SQSErrorResponse {
	if requestID == "" {
		requestID = types.EmptyRequestID
	}
	return &SQSErrorResponse{
		Type:       err.ErrorType(),
		Code:       err.Code(),
		Message:    err.Message(),
		RequestID:  requestID,
		HTTPStatus: err.HTTPStatusCode(),
	}
}

// ToXML returns the XML representation of the error response.
func (e *SQSErrorResponse) ToXML() ([]byte, error) {
	data, err := xml.MarshalIndent(e, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal error response to XML: %w", err)
	}
	return append([]byte(xml.Header), data...), nil
}

// ToJSON returns the JSON representation of the error response.
func (e *SQSErrorResponse) ToJSON() ([]byte, error) {
	// SQS JSON error format uses a flat structure with __type field
	jsonErr := struct {
		Type      string `json:"__type"`
		Message   string `json:"message"`
		RequestID string `json:"RequestId"`
	}{
		Type:      fmt.Sprintf("%s#%s", "com.amazonaws.sqs", e.Code),
		Message:   e.Message,
		RequestID: e.RequestID,
	}

	data, err := jsonMarshal(jsonErr)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal error response to JSON: %w", err)
	}
	return data, nil
}
