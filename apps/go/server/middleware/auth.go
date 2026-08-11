package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/tguidoux/opensqs/pkgs/v1/logger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/credentials"
)

// Package auth implements credential-based authentication middleware for SQS API requests.
// When enabled, incoming requests must include valid AWS credentials matching a credential
// in the store. Two authentication methods are supported:
//
//  1. Legacy Query Protocol: AWSAccessKeyId and AWSSecretAccessKey form parameters
//     (used by awslocal and older AWS SDKs with --no-sign-request style auth).
//
//  2. AWS Signature Version 4: The standard AWS SigV4 algorithm. The middleware
//     parses the Authorization header, reconstructs the canonical request, derives
//     the signing key using the stored secret, and verifies the signature.
//     This allows standard aws-cli and AWS SDK clients to work without modification.

const (
	// sigV4Algorithm is the AWS Signature Version 4 algorithm identifier.
	sigV4Algorithm = "AWS4-HMAC-SHA256"
	// sqsServiceName is the AWS service name for SQS used in SigV4 scope.
	sqsServiceName = "sqs"
	// maxBodySize limits how much of the request body the middleware reads for
	// signature verification. Matches the request handler's limit.
	maxBodySize = 1 << 20 // 1 MiB
)

// authErrorResponse is the XML error response for authentication failures.
type authErrorResponse struct {
	XMLName   xml.Name `xml:"ErrorResponse"`
	Type      string   `xml:"Type"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	RequestID string   `xml:"RequestId"`
}

// Auth creates a middleware that validates credentials against the store.
// If the store is nil, the middleware is a no-op (auth disabled).
func Auth(credStore credentials.CredentialStore, log logger.LoggerInterface) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if credStore == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Try SigV4 first (Authorization header), then fall back to legacy query params.
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, sigV4Algorithm) {
				verifySigV4(w, r, credStore, log, next)
				return
			}

			verifyLegacyQuery(w, r, credStore, log, next)
		})
	}
}

// ---------------------------------------------------------------------------
// Legacy Query Protocol authentication
// ---------------------------------------------------------------------------

// verifyLegacyQuery validates requests using AWSAccessKeyId / AWSSecretAccessKey
// form parameters. This is the simplest auth method, used by awslocal and
// clients configured with legacy query auth.
func verifyLegacyQuery(w http.ResponseWriter, r *http.Request, credStore credentials.CredentialStore, log logger.LoggerInterface, next http.Handler) {
	accessKeyID, secretAccessKey := extractLegacyCredentials(r)
	if accessKeyID == "" || secretAccessKey == "" {
		writeAuthError(w, "MissingAuthenticationToken",
			"Request must contain a valid AWS Access Key ID and Secret Access Key.")
		log.Warningf("missing credentials for request %s %s", r.Method, r.URL.Path)
		return
	}

	cred, err := credStore.GetByAccessKeyID(accessKeyID)
	if err != nil {
		writeAuthError(w, "InvalidClientTokenId",
			"The AWS Access Key Id you provided does not exist in our records.")
		log.Warningf("invalid access key ID %q for request %s %s", accessKeyID, r.Method, r.URL.Path)
		return
	}

	if cred.SecretAccessKey != secretAccessKey {
		writeAuthError(w, "SignatureDoesNotMatch",
			"The request signature we calculated does not match the signature you provided.")
		log.Warningf("secret mismatch for access key ID %q for request %s %s", accessKeyID, r.Method, r.URL.Path)
		return
	}

	next.ServeHTTP(w, r)
}

// extractLegacyCredentials pulls AWSAccessKeyId and AWSSecretAccessKey from
// the URL query string or POST form body.
func extractLegacyCredentials(r *http.Request) (accessKeyID, secretAccessKey string) {
	accessKeyID = r.URL.Query().Get("AWSAccessKeyId")
	secretAccessKey = r.URL.Query().Get("AWSSecretAccessKey")

	if accessKeyID != "" && secretAccessKey != "" {
		return accessKeyID, secretAccessKey
	}

	// For POST with form body, parse the body to extract credentials.
	// We restore the body so downstream handlers can read it again.
	if r.Method == http.MethodPost && (accessKeyID == "" || secretAccessKey == "") {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
		if err == nil {
			// Restore body for downstream handlers
			r.Body = io.NopCloser(bytes.NewReader(body))
			values, parseErr := url.ParseQuery(string(body))
			if parseErr == nil {
				if accessKeyID == "" {
					accessKeyID = values.Get("AWSAccessKeyId")
				}
				if secretAccessKey == "" {
					secretAccessKey = values.Get("AWSSecretAccessKey")
				}
			}
		}
	}

	return accessKeyID, secretAccessKey
}

// ---------------------------------------------------------------------------
// AWS Signature Version 4 verification
// ---------------------------------------------------------------------------

// sigV4Components holds the parsed parts of the Authorization header.
type sigV4Components struct {
	AccessKeyID   string
	Date          string // YYYYMMDD
	Region        string
	Service       string
	SignedHeaders string
	Signature     string
	credentialStr string // full credential scope: date/region/service/aws4_request
}

// verifySigV4 verifies an AWS Signature Version 4 request.
// The algorithm:
//  1. Parse the Authorization header to extract credential scope, signed headers, and signature.
//  2. Read the request body (for the payload hash) and restore it for downstream handlers.
//  3. Build the canonical request from method, URI, query string, headers, and payload hash.
//  4. Build the string-to-sign: algorithm + timestamp + scope + SHA256(canonical request).
//  5. Derive the signing key: HMAC chain using "AWS4" + secret, date, region, service, "aws4_request".
//  6. Compute the signature and compare with the provided one.
func verifySigV4(w http.ResponseWriter, r *http.Request, credStore credentials.CredentialStore, log logger.LoggerInterface, next http.Handler) {
	// Parse the Authorization header
	parts, err := parseSigV4AuthHeader(r.Header.Get("Authorization"))
	if err != nil {
		writeAuthError(w, "IncompleteSignature", err.Error())
		log.Warningf("failed to parse SigV4 header for request %s %s: %v", r.Method, r.URL.Path, err)
		return
	}

	// Look up the credential by Access Key ID
	cred, err := credStore.GetByAccessKeyID(parts.AccessKeyID)
	if err != nil {
		writeAuthError(w, "InvalidClientTokenId",
			"The AWS Access Key Id you provided does not exist in our records.")
		log.Warningf("invalid access key ID %q for request %s %s", parts.AccessKeyID, r.Method, r.URL.Path)
		return
	}

	// Read the request body for the payload hash, then restore it.
	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
	if err != nil {
		writeAuthError(w, "InternalError", "Failed to read request body for signature verification.")
		log.Warningf("failed to read body for SigV4 verification: %v", err)
		return
	}
	// Restore the body so the downstream handler can read it.
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// The client may send X-Amz-Content-Sha256 header with the payload hash.
	// If present, use it; otherwise compute it from the body.
	payloadHash := r.Header.Get("X-Amz-Content-Sha256")
	if payloadHash == "" {
		payloadHash = hashSHA256(bodyBytes)
	} else {
		// When X-Amz-Content-Sha256 is present, verify that the actual body
		// hash matches the header value. This prevents body tampering.
		actualHash := hashSHA256(bodyBytes)
		if payloadHash != actualHash {
			writeAuthError(w, "XAmzContentSHA256Mismatch",
				"The provided 'x-amz-content-sha256' header does not match the SHA256 hash of the request body.")
			log.Warningf("X-Amz-Content-Sha256 mismatch for access key ID %q, request %s %s",
				parts.AccessKeyID, r.Method, r.URL.Path)
			return
		}
	}

	// Build the canonical request
	canonicalReq := buildCanonicalRequest(r, parts.SignedHeaders, payloadHash)

	// Build the string to sign
	amzDate := r.Header.Get("X-Amz-Date")
	if amzDate == "" {
		amzDate = r.Header.Get("Date")
	}
	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s",
		sigV4Algorithm,
		amzDate,
		parts.credentialStr,
		hashSHA256([]byte(canonicalReq)),
	)

	// Derive the signing key
	signingKey := deriveSigningKey(cred.SecretAccessKey, parts.Date, parts.Region, parts.Service)

	// Compute the signature
	computedSignature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	// Compare signatures (constant-time comparison)
	if !hmac.Equal([]byte(computedSignature), []byte(parts.Signature)) {
		writeAuthError(w, "SignatureDoesNotMatch",
			"The request signature we calculated does not match the signature you provided. "+
				"Check your AWS Secret Access Key and signing method.")
		log.Warningf("SigV4 signature mismatch for access key ID %q, request %s %s",
			parts.AccessKeyID, r.Method, r.URL.Path)
		return
	}

	next.ServeHTTP(w, r)
}

// parseSigV4AuthHeader parses an AWS SigV4 Authorization header.
// Expected format:
//
//	AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/sqs/aws4_request, SignedHeaders=content-type;host;x-amz-date, Signature=fe5f80f77d5fa3beca0340968c5a...
func parseSigV4AuthHeader(header string) (*sigV4Components, error) {
	header = strings.TrimSpace(header)

	if !strings.HasPrefix(header, sigV4Algorithm+" ") {
		return nil, fmt.Errorf("authorization header must start with %s", sigV4Algorithm)
	}
	header = strings.TrimPrefix(header, sigV4Algorithm+" ")

	// Split into components by comma
	var credential, signedHeaders, signature string
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "Credential="):
			credential = strings.TrimPrefix(part, "Credential=")
		case strings.HasPrefix(part, "SignedHeaders="):
			signedHeaders = strings.TrimPrefix(part, "SignedHeaders=")
		case strings.HasPrefix(part, "Signature="):
			signature = strings.TrimPrefix(part, "Signature=")
		}
	}

	if credential == "" || signedHeaders == "" || signature == "" {
		return nil, fmt.Errorf("authorization header is missing required components (Credential, SignedHeaders, Signature)")
	}

	// Parse credential scope: date/region/service/aws4_request
	credParts := strings.Split(credential, "/")
	if len(credParts) != 5 {
		return nil, fmt.Errorf("invalid credential scope: expected 5 parts, got %d", len(credParts))
	}
	if credParts[4] != "aws4_request" {
		return nil, fmt.Errorf("invalid credential scope: must end with aws4_request, got %q", credParts[4])
	}

	return &sigV4Components{
		AccessKeyID:   credParts[0],
		Date:          credParts[1],
		Region:        credParts[2],
		Service:       credParts[3],
		SignedHeaders: signedHeaders,
		Signature:     signature,
		credentialStr: fmt.Sprintf("%s/%s/%s/%s", credParts[1], credParts[2], credParts[3], credParts[4]),
	}, nil
}

// buildCanonicalRequest constructs the AWS SigV4 canonical request.
// Format:
//
//	<HTTPMethod>\n
//	<CanonicalURI>\n
//	<CanonicalQueryString>\n
//	<CanonicalHeaders>\n
//	<SignedHeaders>\n
//	<HashedPayload>
func buildCanonicalRequest(r *http.Request, signedHeaders, payloadHash string) string {
	// 1. HTTP Method
	method := r.Method

	// 2. Canonical URI — the URI-encoded path (without query string).
	canonicalURI := canonicalURIPath(r.URL.Path)

	// 3. Canonical query string — sorted by key, URI-encoded.
	canonicalQuery := canonicalQueryString(r.URL.Query())

	// 4. Canonical headers — sorted by lowercase header name, trimmed values.
	canonicalHeaders, signedHeadersList := canonicalHeaders(r, signedHeaders)

	// 5. Signed headers — semicolon-separated lowercase header names.
	signedHeadersStr := strings.Join(signedHeadersList, ";")

	// 6. Payload hash
	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		signedHeadersStr,
		payloadHash,
	)
}

// canonicalURIPath returns the URI-encoded path component.
// AWS uses RFC 3986 encoding. The path "/" is returned as "/".
func canonicalURIPath(path string) string {
	if path == "" {
		return "/"
	}
	// URI-encode each path segment, preserving slashes.
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		segments[i] = encodeURI(seg)
	}
	return strings.Join(segments, "/")
}

// canonicalQueryString builds the canonical query string for SigV4.
// Parameters are sorted by key name, then by value. Both keys and values
// are URI-encoded.
func canonicalQueryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	// Collect all key-value pairs (a key can have multiple values)
	type kv struct {
		key, val string
	}
	var pairs []kv
	for key, vals := range values {
		for _, val := range vals {
			pairs = append(pairs, kv{encodeURI(key), encodeURI(val)})
		}
	}

	// Sort by key, then by value
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].key != pairs[j].key {
			return pairs[i].key < pairs[j].key
		}
		return pairs[i].val < pairs[j].val
	})

	// Build the query string
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

// canonicalHeaders builds the canonical headers string and returns it
// along with the sorted list of signed header names.
func canonicalHeaders(r *http.Request, signedHeaders string) (canonical string, headerNames []string) {
	// Parse the signed headers list
	signedList := strings.Split(signedHeaders, ";")
	sort.Strings(signedList)

	var b strings.Builder
	for _, name := range signedList {
		name = strings.ToLower(strings.TrimSpace(name))
		headerNames = append(headerNames, name)
		b.WriteString(name)
		b.WriteByte(':')
		// Get all values for this header, trim whitespace, join with commas
		values := r.Header.Values(name)
		if len(values) == 0 {
			// Some headers like "host" might not be in r.Header — get from r.Host
			if name == "host" && r.Host != "" {
				b.WriteString(strings.TrimSpace(r.Host))
			}
		} else {
			// Trim each value and join with commas
			trimmed := make([]string, len(values))
			for i, v := range values {
				trimmed[i] = strings.TrimSpace(v)
			}
			b.WriteString(strings.Join(trimmed, ","))
		}
		b.WriteByte('\n')
	}

	return b.String(), headerNames
}

// deriveSigningKey computes the AWS SigV4 signing key.
// Key derivation chain:
//
//	kDate    = HMAC("AWS4" + secret, date)
//	kRegion  = HMAC(kDate, region)
//	kService = HMAC(kRegion, service)
//	kSigning = HMAC(kService, "aws4_request")
func deriveSigningKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

// ---------------------------------------------------------------------------
// Crypto helpers
// ---------------------------------------------------------------------------

// hashSHA256 returns the hex-encoded SHA-256 hash of the input.
func hashSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// hmacSHA256 returns the HMAC-SHA256 of data using key.
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// encodeURI URI-encodes a string per RFC 3986.
// Unreserved characters: A-Z a-z 0-9 - _ . ~
// Everything else is percent-encoded.
func encodeURI(s string) string {
	var b strings.Builder
	for _, c := range s {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteRune(c)
		} else {
			// Percent-encode the UTF-8 bytes
			for _, b2 := range []byte(string(c)) {
				fmt.Fprintf(&b, "%%%02X", b2)
			}
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Error response helper
// ---------------------------------------------------------------------------

// writeAuthError writes an SQS-compatible XML error response for auth failures.
func writeAuthError(w http.ResponseWriter, code, message string) {
	resp := authErrorResponse{
		Type:      "Sender",
		Code:      code,
		Message:   message,
		RequestID: "00000000-0000-0000-0000-000000000000",
	}

	data, err := xml.MarshalIndent(resp, "", "  ")
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "%s: %s", code, message)
		return
	}

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(xml.Header))
	w.Write(data)
}
