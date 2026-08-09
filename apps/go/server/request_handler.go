package main

import (
	"io"
	"net/http"

	"github.com/tguidoux/opensqs/apps/go/server/handlers"
	"github.com/tguidoux/opensqs/apps/go/server/protocol"
	"github.com/tguidoux/opensqs/pkgs/v1/logger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// handleSQSRequest processes an incoming SQS API request.
// It detects the protocol (Query or JSON), parses the request,
// dispatches to the action handler, and writes the response.
func handleSQSRequest(w http.ResponseWriter, r *http.Request, handler *handlers.Handler, log logger.LoggerInterface) {
	// Detect protocol
	protoType, targetHeader := handlers.DetectProtocol(r)

	var req handlers.Request
	var proto handlers.ProtocolType = protoType

	if protoType == handlers.JSONProtocol {
		// JSON protocol: parse body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeErrorResponse(w, queue.NewInternalError("failed to read request body"), proto, "")
			return
		}
		defer r.Body.Close()

		jsonReq, parseErr := protocol.ParseJSONRequest(targetHeader, body)
		if parseErr != nil {
			writeErrorResponse(w, queue.NewInvalidParameterValue(parseErr.Error()), proto, "")
			return
		}
		req = &handlers.JSONRequestAdapter{JSONRequest: jsonReq}
	} else {
		// Query protocol: params can be in URL query string (GET) or body (POST)
		var queryStr string
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				writeErrorResponse(w, queue.NewInternalError("failed to read request body"), proto, "")
				return
			}
			defer r.Body.Close()
			queryStr = string(body)
		} else {
			queryStr = r.URL.RawQuery
		}

		queryReq, parseErr := protocol.ParseQueryRequest(queryStr)
		if parseErr != nil {
			writeErrorResponse(w, queue.NewInvalidParameterValue(parseErr.Error()), proto, "")
			return
		}
		req = &handlers.QueryRequestAdapter{QueryRequest: queryReq}
	}

	// Handle the request
	resp, err := handler.HandleRequest(r.Context(), req, proto)
	if err != nil {
		requestID := types.EmptyRequestID
		if resp != nil {
			requestID = resp.RequestID
		}
		writeErrorResponse(w, err, proto, requestID)
		return
	}

	// Marshal and write the response
	data, err := handlers.MarshalResponse(resp, proto)
	if err != nil {
		writeErrorResponse(w, queue.NewInternalError("failed to marshal response"), proto, resp.RequestID)
		return
	}

	// Set content type based on protocol
	if proto == handlers.JSONProtocol {
		w.Header().Set("Content-Type", types.JSONContentType)
	} else {
		w.Header().Set("Content-Type", types.XMLContentType)
	}
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
