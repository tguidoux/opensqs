package main

import (
	"fmt"
	"net/http"

	"github.com/tguidoux/opensqs/apps/go/server/handlers"
	"github.com/tguidoux/opensqs/apps/go/server/protocol"
	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// writeErrorResponse writes an SQS error response in the appropriate format.
func writeErrorResponse(w http.ResponseWriter, err error, proto handlers.ProtocolType, requestID string) {
	sqsErr, ok := err.(types.SQSError)
	if !ok {
		sqsErr = queue.NewInternalError(err.Error())
	}

	errorResp := protocol.NewErrorResponse(sqsErr, requestID)

	var data []byte
	var contentType string
	var writeErr error

	if proto == handlers.JSONProtocol {
		data, writeErr = errorResp.ToJSON()
		contentType = types.JSONContentType
	} else {
		data, writeErr = errorResp.ToXML()
		contentType = types.XMLContentType
	}

	if writeErr != nil {
		// Last resort: write a plain text error
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Internal error: %s", sqsErr.Error())
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(sqsErr.HTTPStatusCode())
	w.Write(data)
}
