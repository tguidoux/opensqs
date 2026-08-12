package handlers

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tguidoux/opensqs/apps/go/server/protocol"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// Message attribute names returned in ReceiveMessage responses.
const (
	AttrSentTimestamp                    = "SentTimestamp"
	AttrApproximateReceiveCount          = "ApproximateReceiveCount"
	AttrApproximateFirstReceiveTimestamp = "ApproximateFirstReceiveTimestamp"
)

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
			decoded, err := base64.StdEncoding.DecodeString(attr.BinaryValue)
			if err == nil {
				ma.BinaryValue = decoded
			}
			// If base64 decode fails, the attribute is silently skipped.
			// This matches AWS SQS behavior where invalid attribute values are ignored.
		}
		result[name] = ma
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
	if mediaType, _, found := strings.Cut(contentType, ";"); found {
		contentType = strings.TrimSpace(mediaType)
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
			Name: AttrSentTimestamp, Value: formatTimestamp(msg.SentTimestamp),
		})
	}
	if msg.ApproximateReceiveCount > 0 {
		xmlMsg.Attributes = append(xmlMsg.Attributes, protocol.XMLAttribute{
			Name: AttrApproximateReceiveCount, Value: formatInt(msg.ApproximateReceiveCount),
		})
	}
	if !msg.FirstReceivedTimestamp.IsZero() {
		xmlMsg.Attributes = append(xmlMsg.Attributes, protocol.XMLAttribute{
			Name: AttrApproximateFirstReceiveTimestamp, Value: formatTimestamp(msg.FirstReceivedTimestamp),
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
		jsonMsg.Attributes[AttrSentTimestamp] = formatTimestamp(msg.SentTimestamp)
	}
	if msg.ApproximateReceiveCount > 0 {
		jsonMsg.Attributes[AttrApproximateReceiveCount] = formatInt(msg.ApproximateReceiveCount)
	}
	if !msg.FirstReceivedTimestamp.IsZero() {
		jsonMsg.Attributes[AttrApproximateFirstReceiveTimestamp] = formatTimestamp(msg.FirstReceivedTimestamp)
	}

	return jsonMsg
}

// formatTimestamp converts a time.Time to milliseconds since epoch string.
func formatTimestamp(t time.Time) string {
	return strconv.FormatInt(t.UnixMilli(), 10)
}

// formatInt converts an int to a string.
func formatInt(i int) string {
	return strconv.FormatInt(int64(i), 10)
}
