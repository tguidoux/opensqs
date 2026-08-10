package id

// Package id provides utilities for generating unique identifiers.

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// NewUUID generates a version 4 UUID string (e.g. "550e8400-e29b-41d4-a716-446655440000").
// Uses crypto/rand for cryptographic randomness. Falls back to a timestamp-based
// ID if random generation fails.
func NewUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp if crypto/rand fails
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	// Set version 4 and variant bits per RFC 4122
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// NewHexID generates a 16-byte hex-encoded identifier (32 characters).
// Uses crypto/rand for cryptographic randomness. Falls back to a timestamp-based
// ID if random generation fails.
func NewHexID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp if crypto/rand fails
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
