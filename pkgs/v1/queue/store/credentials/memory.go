package credentials

import (
	"fmt"
	"sync"
	"time"
)

// MemoryCredentialStore is an in-memory implementation of CredentialStore.
// Credentials are lost when the process exits.
type MemoryCredentialStore struct {
	mu          sync.RWMutex
	credentials map[string]*Credential
}

// NewMemoryCredentialStore creates a new in-memory credential store.
func NewMemoryCredentialStore() *MemoryCredentialStore {
	return &MemoryCredentialStore{
		credentials: make(map[string]*Credential),
	}
}

// Create generates and stores a new credential.
func (s *MemoryCredentialStore) Create(label, region, accountID string) (*Credential, error) {
	cred := &Credential{
		ID:              GenerateID(),
		Label:           label,
		AccessKeyID:     GenerateAccessKeyID(),
		SecretAccessKey: GenerateSecretAccessKey(),
		Region:          region,
		AccountID:       accountID,
		CreatedAt:       time.Now().UTC(),
	}

	s.mu.Lock()
	s.credentials[cred.ID] = cred
	s.mu.Unlock()

	return cred, nil
}

// List returns all stored credentials without secret access keys.
func (s *MemoryCredentialStore) List() ([]*Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Credential, 0, len(s.credentials))
	for _, c := range s.credentials {
		result = append(result, &Credential{
			ID:          c.ID,
			Label:       c.Label,
			AccessKeyID: c.AccessKeyID,
			Region:      c.Region,
			AccountID:   c.AccountID,
			CreatedAt:   c.CreatedAt,
			// SecretAccessKey intentionally omitted
		})
	}
	return result, nil
}

// Get returns a single credential by ID, including the secret access key.
func (s *MemoryCredentialStore) Get(id string) (*Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.credentials[id]
	if !ok {
		return nil, fmt.Errorf("credential not found: %s", id)
	}
	// Return a copy to prevent external mutation
	return &Credential{
		ID:              c.ID,
		Label:           c.Label,
		AccessKeyID:     c.AccessKeyID,
		SecretAccessKey: c.SecretAccessKey,
		Region:          c.Region,
		AccountID:       c.AccountID,
		CreatedAt:       c.CreatedAt,
	}, nil
}

// GetByAccessKeyID returns a credential by its Access Key ID, including the secret.
func (s *MemoryCredentialStore) GetByAccessKeyID(accessKeyID string) (*Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, c := range s.credentials {
		if c.AccessKeyID == accessKeyID {
			return &Credential{
				ID:              c.ID,
				Label:           c.Label,
				AccessKeyID:     c.AccessKeyID,
				SecretAccessKey: c.SecretAccessKey,
				Region:          c.Region,
				AccountID:       c.AccountID,
				CreatedAt:       c.CreatedAt,
			}, nil
		}
	}
	return nil, fmt.Errorf("credential not found for access key ID: %s", accessKeyID)
}

// Delete removes a credential by ID.
func (s *MemoryCredentialStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.credentials[id]; !ok {
		return fmt.Errorf("credential not found: %s", id)
	}
	delete(s.credentials, id)
	return nil
}

// Close is a no-op for the in-memory store.
func (s *MemoryCredentialStore) Close() error {
	return nil
}
