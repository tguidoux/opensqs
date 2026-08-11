package credentials

// Package credentials provides storage for AWS-style credentials
// (Access Key ID, Secret Access Key, Region, Account ID) generated
// via the OpenSQS web UI. Implementations include in-memory, SQLite,
// and BadgerDB backends, matching the server's storageType config.

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Credential represents a set of AWS-style credentials created via the UI.
// The SecretAccessKey is only returned by Create and Get — the List
// method omits it so it is never accidentally exposed in bulk.
type Credential struct {
	ID              string
	Label           string
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	AccountID       string
	CreatedAt       time.Time
}

// CredentialStore defines the interface for persisting credentials.
// Implementations must be safe for concurrent use.
type CredentialStore interface {
	// Create generates a new credential with the given label, region,
	// and account ID. The Access Key ID and Secret Access Key are
	// auto-generated. Returns the created credential (including the
	// secret, which is only available at creation time).
	Create(label, region, accountID string) (*Credential, error)

	// List returns all stored credentials. Secret access keys are
	// not included in the returned credentials.
	List() ([]*Credential, error)

	// Get returns a single credential by ID, including the secret
	// access key.
	Get(id string) (*Credential, error)

	// GetByAccessKeyID returns a credential by its Access Key ID,
	// including the secret access key. Used for request authentication.
	GetByAccessKeyID(accessKeyID string) (*Credential, error)

	// Delete removes a credential by ID.
	Delete(id string) error

	// Close releases any resources held by the store.
	Close() error
}

// GenerateAccessKeyID generates an AWS-style access key ID.
// Format: "AKIA" followed by 16 random uppercase alphanumeric characters.
func GenerateAccessKeyID() string {
	const prefix = "AKIA"
	const keyLen = 16
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, keyLen)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return prefix + string(b)
}

// GenerateSecretAccessKey generates an AWS-style secret access key.
// Returns a 40-character hex string (matching AWS secret key length).
func GenerateSecretAccessKey() string {
	b := make([]byte, 20) // 20 bytes → 40 hex chars
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}

// GenerateID generates a short unique identifier for a credential.
// Uses 8 random bytes (16 hex characters).
func GenerateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}
