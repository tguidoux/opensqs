package credentials

import (
	"database/sql"
	"fmt"
	"time"
)

// SQLiteCredentialStore is a SQLite-backed implementation of CredentialStore.
// It uses the same *sql.DB instance as the message store.
type SQLiteCredentialStore struct {
	db *sql.DB
}

// NewSQLiteCredentialStore creates a new SQLite credential store.
// The credentials table is created if it does not exist.
func NewSQLiteCredentialStore(db *sql.DB) (*SQLiteCredentialStore, error) {
	s := &SQLiteCredentialStore{db: db}
	if err := s.init(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SQLiteCredentialStore) init() error {
	const createTable = `
	CREATE TABLE IF NOT EXISTS credentials (
		id                TEXT PRIMARY KEY,
		label             TEXT NOT NULL,
		access_key_id     TEXT NOT NULL,
		secret_access_key TEXT NOT NULL,
		region            TEXT NOT NULL,
		account_id        TEXT NOT NULL,
		created_at        TEXT NOT NULL
	)`
	_, err := s.db.Exec(createTable)
	if err != nil {
		return fmt.Errorf("failed to create credentials table: %w", err)
	}
	return nil
}

// Create generates and stores a new credential.
func (s *SQLiteCredentialStore) Create(label, region, accountID string) (*Credential, error) {
	cred := &Credential{
		ID:              GenerateID(),
		Label:           label,
		AccessKeyID:     GenerateAccessKeyID(),
		SecretAccessKey: GenerateSecretAccessKey(),
		Region:          region,
		AccountID:       accountID,
		CreatedAt:       time.Now().UTC(),
	}

	_, err := s.db.Exec(
		`INSERT INTO credentials (id, label, access_key_id, secret_access_key, region, account_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		cred.ID, cred.Label, cred.AccessKeyID, cred.SecretAccessKey, cred.Region, cred.AccountID, cred.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert credential: %w", err)
	}

	return cred, nil
}

// Import stores a credential with an explicitly provided access key ID and
// secret access key. Returns an error if a credential with the same access
// key ID already exists.
func (s *SQLiteCredentialStore) Import(label, accessKeyID, secretAccessKey, region, accountID string) (*Credential, error) {
	// Check for duplicate access key ID
	var existingID string
	err := s.db.QueryRow(`SELECT id FROM credentials WHERE access_key_id = ?`, accessKeyID).Scan(&existingID)
	if err == nil {
		return nil, fmt.Errorf("credential with access key ID %q already exists", accessKeyID)
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check for duplicate credential: %w", err)
	}

	cred := &Credential{
		ID:              GenerateID(),
		Label:           label,
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		Region:          region,
		AccountID:       accountID,
		CreatedAt:       time.Now().UTC(),
	}

	_, err = s.db.Exec(
		`INSERT INTO credentials (id, label, access_key_id, secret_access_key, region, account_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		cred.ID, cred.Label, cred.AccessKeyID, cred.SecretAccessKey, cred.Region, cred.AccountID, cred.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert credential: %w", err)
	}

	return cred, nil
}

// List returns all stored credentials without secret access keys.
func (s *SQLiteCredentialStore) List() ([]*Credential, error) {
	rows, err := s.db.Query(
		`SELECT id, label, access_key_id, region, account_id, created_at FROM credentials ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query credentials: %w", err)
	}
	defer rows.Close()

	var result []*Credential
	for rows.Next() {
		var c Credential
		var createdAtStr string
		if err := rows.Scan(&c.ID, &c.Label, &c.AccessKeyID, &c.Region, &c.AccountID, &createdAtStr); err != nil {
			return nil, fmt.Errorf("failed to scan credential: %w", err)
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		result = append(result, &c)
	}
	return result, rows.Err()
}

// Get returns a single credential by ID, including the secret access key.
func (s *SQLiteCredentialStore) Get(id string) (*Credential, error) {
	var c Credential
	var createdAtStr string
	err := s.db.QueryRow(
		`SELECT id, label, access_key_id, secret_access_key, region, account_id, created_at FROM credentials WHERE id = ?`,
		id,
	).Scan(&c.ID, &c.Label, &c.AccessKeyID, &c.SecretAccessKey, &c.Region, &c.AccountID, &createdAtStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("credential not found: %s", id)
		}
		return nil, fmt.Errorf("failed to query credential: %w", err)
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	return &c, nil
}

// GetByAccessKeyID returns a credential by its Access Key ID, including the secret.
func (s *SQLiteCredentialStore) GetByAccessKeyID(accessKeyID string) (*Credential, error) {
	var c Credential
	var createdAtStr string
	err := s.db.QueryRow(
		`SELECT id, label, access_key_id, secret_access_key, region, account_id, created_at FROM credentials WHERE access_key_id = ?`,
		accessKeyID,
	).Scan(&c.ID, &c.Label, &c.AccessKeyID, &c.SecretAccessKey, &c.Region, &c.AccountID, &createdAtStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("credential not found for access key ID: %s", accessKeyID)
		}
		return nil, fmt.Errorf("failed to query credential: %w", err)
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	return &c, nil
}

// Delete removes a credential by ID.
func (s *SQLiteCredentialStore) Delete(id string) error {
	result, err := s.db.Exec(`DELETE FROM credentials WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete credential: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("credential not found: %s", id)
	}
	return nil
}

// Close is a no-op — the *sql.DB is managed by the caller.
func (s *SQLiteCredentialStore) Close() error {
	return nil
}
