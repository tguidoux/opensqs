package middleware

import "net/http"

// statusResponseWriter wraps an http.ResponseWriter to capture
// the HTTP status code and the number of bytes written.
// It is used by the request logging middleware to record
// the response status and size after the handler completes.
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
	wroteHeader  bool
}

// newStatusResponseWriter creates a new statusResponseWriter
// with a default status code of 200.
func newStatusResponseWriter(w http.ResponseWriter) *statusResponseWriter {
	return &statusResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// WriteHeader captures the status code and delegates to the wrapped ResponseWriter.
func (w *statusResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.statusCode = code
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

// Write captures the number of bytes written and delegates to the wrapped ResponseWriter.
func (w *statusResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.wroteHeader = true
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += n
	return n, err
}

// Flush delegates to the wrapped ResponseWriter if it supports flushing.
func (w *statusResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Unwrap returns the underlying http.ResponseWriter, enabling
// http.ResponseController to access extended interfaces (Hijack, Flusher, etc.)
// on the wrapped writer.
func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
