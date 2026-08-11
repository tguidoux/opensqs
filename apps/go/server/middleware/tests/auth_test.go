package middleware_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tguidoux/opensqs/apps/go/server/middleware"
	"github.com/tguidoux/opensqs/pkgs/v1/logger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/credentials"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// testCredential is a fixed credential used across all tests.
var testCredential = &credentials.Credential{
	ID:              "test-id",
	Label:           "test",
	AccessKeyID:     "AKIATEST1234567890AB",
	SecretAccessKey: "testsecret1234567890abcdef1234567890abcd",
	Region:          "us-east-1",
	AccountID:       "123456789012",
}

// testLogger creates a logger for tests.
func testLogger() logger.LoggerInterface {
	return logger.New("test", logger.UncontextualLoggerType)
}

// testHandler is a simple handler that responds 200 OK.
func testHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

// fakeCredStore is a minimal CredentialStore for testing.
type fakeCredStore struct {
	creds map[string]*credentials.Credential
}

func newFakeCredStore(creds ...*credentials.Credential) *fakeCredStore {
	m := &fakeCredStore{creds: make(map[string]*credentials.Credential)}
	for _, c := range creds {
		m.creds[c.AccessKeyID] = c
	}
	return m
}

func (s *fakeCredStore) Create(label, region, accountID string) (*credentials.Credential, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *fakeCredStore) List() ([]*credentials.Credential, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *fakeCredStore) Get(id string) (*credentials.Credential, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *fakeCredStore) GetByAccessKeyID(accessKeyID string) (*credentials.Credential, error) {
	c, ok := s.creds[accessKeyID]
	if !ok {
		return nil, fmt.Errorf("not found: %s", accessKeyID)
	}
	return c, nil
}
func (s *fakeCredStore) Delete(id string) error { return fmt.Errorf("not implemented") }
func (s *fakeCredStore) Close() error           { return nil }

// ---------------------------------------------------------------------------
// SigV4 signing helpers (mirrors the middleware's verification logic)
// ---------------------------------------------------------------------------

const (
	testAlgorithm   = "AWS4-HMAC-SHA256"
	testService     = "sqs"
	testRegion      = "us-east-1"
	testDate        = "20240115"
	testAmzDate     = "20240115T120000Z"
	testHost        = "sqs.us-east-1.amazonaws.com"
	testContentType = "application/x-amz-query-1.0"
)

// signSigV4 computes a valid SigV4 signature for the given request parameters.
// This mirrors what the AWS SDK would produce.
func signSigV4(method, path string, query url.Values, headers map[string]string, body []byte, secret string) (authHeader, amzDate, contentHash string) {
	// Compute payload hash
	contentHash = hashHex(body)

	// Ensure required headers are set
	if headers == nil {
		headers = make(map[string]string)
	}
	headers["host"] = testHost
	headers["x-amz-date"] = testAmzDate
	headers["x-amz-content-sha256"] = contentHash

	// Build signed headers list (sorted)
	signedHeaderNames := make([]string, 0, len(headers))
	for k := range headers {
		signedHeaderNames = append(signedHeaderNames, strings.ToLower(k))
	}
	sort.Strings(signedHeaderNames)
	signedHeaders := strings.Join(signedHeaderNames, ";")

	// Build canonical headers
	var canonicalHdr strings.Builder
	for _, name := range signedHeaderNames {
		// Find the header value (case-insensitive lookup)
		var val string
		for k, v := range headers {
			if strings.ToLower(k) == name {
				val = strings.TrimSpace(v)
				break
			}
		}
		canonicalHdr.WriteString(name)
		canonicalHdr.WriteByte(':')
		canonicalHdr.WriteString(val)
		canonicalHdr.WriteByte('\n')
	}

	// Build canonical query string
	canonicalQuery := canonicalQueryStr(query)

	// Build canonical URI
	canonicalURI := canonicalURIForPath(path)

	// Build canonical request
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method,
		canonicalURI,
		canonicalQuery,
		canonicalHdr.String(),
		signedHeaders,
		contentHash,
	)

	// Build credential scope
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", testDate, testRegion, testService)

	// Build string to sign
	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s",
		testAlgorithm,
		testAmzDate,
		credentialScope,
		hashHex([]byte(canonicalRequest)),
	)

	// Derive signing key
	signingKey := deriveKey(secret, testDate, testRegion, testService)

	// Compute signature
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	// Build Authorization header
	authHeader = fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		testAlgorithm,
		testCredential.AccessKeyID,
		credentialScope,
		signedHeaders,
		signature,
	)

	return authHeader, testAmzDate, contentHash
}

func canonicalQueryStr(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	type kv struct{ key, val string }
	var pairs []kv
	for key, vals := range values {
		for _, val := range vals {
			pairs = append(pairs, kv{encodeURI(key), encodeURI(val)})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].key != pairs[j].key {
			return pairs[i].key < pairs[j].key
		}
		return pairs[i].val < pairs[j].val
	})
	var b strings.Builder
	for i, p := range pairs {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(p.key)
		b.WriteByte('=')
		b.WriteString(p.val)
	}
	return b.String()
}

func canonicalURIForPath(path string) string {
	if path == "" {
		return "/"
	}
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		segments[i] = encodeURI(seg)
	}
	return strings.Join(segments, "/")
}

func encodeURI(s string) string {
	var b strings.Builder
	for _, c := range s {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteRune(c)
		} else {
			for _, b2 := range []byte(string(c)) {
				fmt.Fprintf(&b, "%%%02X", b2)
			}
		}
	}
	return b.String()
}

func hashHex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func deriveKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

// buildSignedRequest creates an http.Request with a valid SigV4 signature.
func buildSignedRequest(method, path string, query url.Values, body []byte, secret string) *http.Request {
	headers := map[string]string{
		"content-type": testContentType,
	}

	authHeader, amzDate, contentHash := signSigV4(method, path, query, headers, body, secret)

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	u := &url.URL{
		Path:     path,
		RawQuery: query.Encode(),
	}

	req := httptest.NewRequest(method, u.String(), bodyReader)
	req.Host = testHost
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", contentHash)
	req.Header.Set("Content-Type", testContentType)

	return req
}

// ---------------------------------------------------------------------------
// Tests: Auth middleware with nil store (disabled)
// ---------------------------------------------------------------------------

func TestAuthNilStore(t *testing.T) {
	mw := middleware.Auth(nil, testLogger())
	wrapped := mw(testHandler())

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "ok", rr.Body.String())
}

// ---------------------------------------------------------------------------
// Tests: Legacy query protocol authentication
// ---------------------------------------------------------------------------

func TestAuthLegacyQuerySuccess(t *testing.T) {
	store := newFakeCredStore(testCredential)
	mw := middleware.Auth(store, testLogger())
	wrapped := mw(testHandler())

	// Build a POST request with legacy query params in the body
	form := url.Values{}
	form.Set("Action", "SendMessage")
	form.Set("AWSAccessKeyId", testCredential.AccessKeyID)
	form.Set("AWSSecretAccessKey", testCredential.SecretAccessKey)
	body := form.Encode()

	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "ok", rr.Body.String())
}

func TestAuthLegacyQueryInURL(t *testing.T) {
	store := newFakeCredStore(testCredential)
	mw := middleware.Auth(store, testLogger())
	wrapped := mw(testHandler())

	// Credentials in URL query string
	u := fmt.Sprintf("/?Action=ListQueues&AWSAccessKeyId=%s&AWSSecretAccessKey=%s",
		testCredential.AccessKeyID, testCredential.SecretAccessKey)
	req := httptest.NewRequest("GET", u, nil)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthLegacyQueryMissingCredentials(t *testing.T) {
	store := newFakeCredStore(testCredential)
	mw := middleware.Auth(store, testLogger())
	wrapped := mw(testHandler())

	form := url.Values{}
	form.Set("Action", "ListQueues")
	body := form.Encode()

	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "MissingAuthenticationToken")
}

func TestAuthLegacyQueryInvalidAccessKey(t *testing.T) {
	store := newFakeCredStore(testCredential)
	mw := middleware.Auth(store, testLogger())
	wrapped := mw(testHandler())

	form := url.Values{}
	form.Set("Action", "ListQueues")
	form.Set("AWSAccessKeyId", "AKIAINVALID00000000")
	form.Set("AWSSecretAccessKey", testCredential.SecretAccessKey)
	body := form.Encode()

	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "InvalidClientTokenId")
}

func TestAuthLegacyQueryWrongSecret(t *testing.T) {
	store := newFakeCredStore(testCredential)
	mw := middleware.Auth(store, testLogger())
	wrapped := mw(testHandler())

	form := url.Values{}
	form.Set("Action", "ListQueues")
	form.Set("AWSAccessKeyId", testCredential.AccessKeyID)
	form.Set("AWSSecretAccessKey", "wrongsecret")
	body := form.Encode()

	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "SignatureDoesNotMatch")
}

// ---------------------------------------------------------------------------
// Tests: SigV4 authentication
// ---------------------------------------------------------------------------

func TestAuthSigV4Success(t *testing.T) {
	store := newFakeCredStore(testCredential)
	mw := middleware.Auth(store, testLogger())
	wrapped := mw(testHandler())

	body := []byte("Action=SendMessage&MessageBody=hello")
	query := url.Values{}
	query.Set("Action", "SendMessage")

	req := buildSignedRequest("POST", "/", query, body, testCredential.SecretAccessKey)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "ok", rr.Body.String())
}

func TestAuthSigV4GetSuccess(t *testing.T) {
	store := newFakeCredStore(testCredential)
	mw := middleware.Auth(store, testLogger())
	wrapped := mw(testHandler())

	query := url.Values{}
	query.Set("Action", "ListQueues")

	req := buildSignedRequest("GET", "/", query, nil, testCredential.SecretAccessKey)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthSigV4InvalidSignature(t *testing.T) {
	store := newFakeCredStore(testCredential)
	mw := middleware.Auth(store, testLogger())
	wrapped := mw(testHandler())

	body := []byte("Action=SendMessage&MessageBody=hello")
	query := url.Values{}
	query.Set("Action", "SendMessage")

	// Sign with a wrong secret
	req := buildSignedRequest("POST", "/", query, body, "wrongsecret")
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "SignatureDoesNotMatch")
}

func TestAuthSigV4InvalidAccessKey(t *testing.T) {
	store := newFakeCredStore(testCredential)
	mw := middleware.Auth(store, testLogger())
	wrapped := mw(testHandler())

	body := []byte("Action=ListQueues")
	query := url.Values{}
	query.Set("Action", "ListQueues")

	// Build a request with a non-existent access key ID
	headers := map[string]string{"content-type": testContentType}
	authHeader, amzDate, contentHash := signSigV4("POST", "/", query, headers, body, testCredential.SecretAccessKey)

	// Replace the access key ID in the auth header with an invalid one
	authHeader = strings.Replace(authHeader, testCredential.AccessKeyID, "AKIAINVALID00000000", 1)

	req := httptest.NewRequest("POST", "/?Action=ListQueues", bytes.NewReader(body))
	req.Host = testHost
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", contentHash)
	req.Header.Set("Content-Type", testContentType)

	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "InvalidClientTokenId")
}

func TestAuthSigV4MalformedAuthHeader(t *testing.T) {
	store := newFakeCredStore(testCredential)
	mw := middleware.Auth(store, testLogger())
	wrapped := mw(testHandler())

	req := httptest.NewRequest("POST", "/", strings.NewReader("Action=ListQueues"))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=incomplete")
	req.Header.Set("X-Amz-Date", testAmzDate)

	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "IncompleteSignature")
}

func TestAuthSigV4BodyRestoredAfterVerification(t *testing.T) {
	store := newFakeCredStore(testCredential)

	// Handler that reads the body to verify it was restored
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, "Action=SendMessage&MessageBody=hello", string(body))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mw := middleware.Auth(store, testLogger())
	wrapped := mw(handler)

	body := []byte("Action=SendMessage&MessageBody=hello")
	query := url.Values{}
	query.Set("Action", "SendMessage")

	req := buildSignedRequest("POST", "/", query, body, testCredential.SecretAccessKey)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthSigV4WithQueryParams(t *testing.T) {
	store := newFakeCredStore(testCredential)
	mw := middleware.Auth(store, testLogger())
	wrapped := mw(testHandler())

	// Test with multiple query params that need canonical sorting
	query := url.Values{}
	query.Set("Action", "ReceiveMessage")
	query.Set("QueueUrl", "http://sqs.us-east-1.amazonaws.com/123456789012/my-queue")
	query.Set("MaxNumberOfMessages", "10")
	query.Set("WaitTimeSeconds", "20")

	body := []byte{}
	req := buildSignedRequest("GET", "/", query, body, testCredential.SecretAccessKey)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthSigV4TamperedBody(t *testing.T) {
	store := newFakeCredStore(testCredential)
	mw := middleware.Auth(store, testLogger())
	wrapped := mw(testHandler())

	// Sign one body, but send a different one
	body := []byte("Action=SendMessage&MessageBody=hello")
	query := url.Values{}
	query.Set("Action", "SendMessage")

	headers := map[string]string{"content-type": testContentType}
	authHeader, amzDate, contentHash := signSigV4("POST", "/", query, headers, body, testCredential.SecretAccessKey)

	// Send a tampered body
	tamperedBody := []byte("Action=SendMessage&MessageBody=hacked")
	req := httptest.NewRequest("POST", "/?Action=SendMessage", bytes.NewReader(tamperedBody))
	req.Host = testHost
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", contentHash)
	req.Header.Set("Content-Type", testContentType)

	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	// Should fail because the body hash won't match the X-Amz-Content-Sha256 header
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "XAmzContentSHA256Mismatch")
}

func TestAuthSigV4WithPath(t *testing.T) {
	store := newFakeCredStore(testCredential)
	mw := middleware.Auth(store, testLogger())
	wrapped := mw(testHandler())

	// Test with a path like /123456789012/my-queue
	query := url.Values{}
	query.Set("Action", "SendMessage")

	body := []byte("Action=SendMessage&MessageBody=hello")
	req := buildSignedRequest("POST", "/123456789012/my-queue", query, body, testCredential.SecretAccessKey)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// ---------------------------------------------------------------------------
// Tests: Error response format
// ---------------------------------------------------------------------------

func TestAuthErrorResponseFormat(t *testing.T) {
	store := newFakeCredStore(testCredential)
	mw := middleware.Auth(store, testLogger())
	wrapped := mw(testHandler())

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "ErrorResponse")
	assert.Contains(t, rr.Body.String(), "MissingAuthenticationToken")
	assert.Contains(t, rr.Body.String(), "Sender")
	assert.Equal(t, "text/xml", rr.Header().Get("Content-Type"))
}
