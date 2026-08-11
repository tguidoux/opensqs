package credentials

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/dgraph-io/badger/v4"
)

const badgerKeyPrefix = "credentials:"

// BadgerCredentialStore is a BadgerDB-backed implementation of CredentialStore.
// It uses the same *badger.DB instance as the message store, with a
// "credentials:" key prefix to isolate credential data.
type BadgerCredentialStore struct {
	db *badger.DB
}

// NewBadgerCredentialStore creates a new BadgerDB credential store.
func NewBadgerCredentialStore(db *badger.DB) *BadgerCredentialStore {
	return &BadgerCredentialStore{db: db}
}

// Create generates and stores a new credential.
func (s *BadgerCredentialStore) Create(label, region, accountID string) (*Credential, error) {
	cred := &Credential{
		ID:              GenerateID(),
		Label:           label,
		AccessKeyID:     GenerateAccessKeyID(),
		SecretAccessKey: GenerateSecretAccessKey(),
		Region:          region,
		AccountID:       accountID,
		CreatedAt:       time.Now().UTC(),
	}

	data, err := json.Marshal(cred)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credential: %w", err)
	}

	key := badgerKeyPrefix + cred.ID
	err = s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), data)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to store credential: %w", err)
	}

	return cred, nil
}

// List returns all stored credentials without secret access keys.
func (s *BadgerCredentialStore) List() ([]*Credential, error) {
	var result []*Credential

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(badgerKeyPrefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var c Credential
				if err := json.Unmarshal(val, &c); err != nil {
					return err
				}
				// Strip the secret for list view
				c.SecretAccessKey = ""
				result = append(result, &c)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}

	// Sort by CreatedAt descending (newest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}

// Get returns a single credential by ID, including the secret access key.
func (s *BadgerCredentialStore) Get(id string) (*Credential, error) {
	key := badgerKeyPrefix + id
	var cred Credential

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return fmt.Errorf("credential not found: %s", id)
			}
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &cred)
		})
	})
	if err != nil {
		return nil, err
	}

	return &cred, nil
}

// GetByAccessKeyID returns a credential by its Access Key ID, including the secret.
func (s *BadgerCredentialStore) GetByAccessKeyID(accessKeyID string) (*Credential, error) {
	var cred *Credential

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(badgerKeyPrefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var c Credential
				if err := json.Unmarshal(val, &c); err != nil {
					return err
				}
				if c.AccessKeyID == accessKeyID {
					cred = &c
				}
				return nil
			})
			if err != nil {
				return err
			}
			if cred != nil {
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query credential: %w", err)
	}
	if cred == nil {
		return nil, fmt.Errorf("credential not found for access key ID: %s", accessKeyID)
	}
	return cred, nil
}

// Delete removes a credential by ID.
func (s *BadgerCredentialStore) Delete(id string) error {
	key := badgerKeyPrefix + id
	err := s.db.Update(func(txn *badger.Txn) error {
		// Check if key exists first
		_, err := txn.Get([]byte(key))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return fmt.Errorf("credential not found: %s", id)
			}
			return err
		}
		return txn.Delete([]byte(key))
	})
	if err != nil {
		return fmt.Errorf("failed to delete credential: %w", err)
	}
	return nil
}

// Close is a no-op — the *badger.DB is managed by the caller.
func (s *BadgerCredentialStore) Close() error {
	return nil
}
