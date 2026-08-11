package credentials_test

import (
	"database/sql"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/badger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/credentials"
	_ "modernc.org/sqlite"
)

// newTestDB creates a temporary SQLite database for testing.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := "/tmp/opensqs-cred-test-" + strconv.FormatInt(time.Now().UnixNano(), 36) + ".db"
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Close()
		os.Remove(dbPath)
	})
	return db
}

// newTestBadgerDB creates a temporary BadgerDB instance for testing
// and returns the underlying *badger.DB.
func newTestBadgerDB(t *testing.T) *badger.BadgerDB {
	t.Helper()
	db, err := badger.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Close()
	})
	return db
}

// runStoreTests runs the common CRUD test suite against any CredentialStore.
func runStoreTests(t *testing.T, store credentials.CredentialStore) {
	t.Helper()

	// Create
	cred, err := store.Create("test-label", "us-east-1", "123456789012")
	require.NoError(t, err)
	require.NotNil(t, cred)
	assert.NotEmpty(t, cred.ID)
	assert.Equal(t, "test-label", cred.Label)
	assert.Equal(t, "us-east-1", cred.Region)
	assert.Equal(t, "123456789012", cred.AccountID)
	assert.NotEmpty(t, cred.AccessKeyID)
	assert.NotEmpty(t, cred.SecretAccessKey)
	assert.False(t, cred.CreatedAt.IsZero())

	// Get
	fetched, err := store.Get(cred.ID)
	require.NoError(t, err)
	assert.Equal(t, cred.ID, fetched.ID)
	assert.Equal(t, cred.AccessKeyID, fetched.AccessKeyID)
	assert.Equal(t, cred.SecretAccessKey, fetched.SecretAccessKey)

	// List
	list, err := store.List()
	require.NoError(t, err)
	assert.Len(t, list, 1)
	// List should NOT include the secret
	assert.Empty(t, list[0].SecretAccessKey)
	assert.Equal(t, cred.AccessKeyID, list[0].AccessKeyID)

	// Delete
	err = store.Delete(cred.ID)
	require.NoError(t, err)

	// Verify deleted
	list, err = store.List()
	require.NoError(t, err)
	assert.Empty(t, list)

	// Get after delete should error
	_, err = store.Get(cred.ID)
	assert.Error(t, err)

	// Delete non-existent should error
	err = store.Delete("nonexistent")
	assert.Error(t, err)
}

func TestMemoryStore(t *testing.T) {
	store := credentials.NewMemoryCredentialStore()
	defer store.Close()
	runStoreTests(t, store)
}

func TestSQLiteStore(t *testing.T) {
	db := newTestDB(t)
	store, err := credentials.NewSQLiteCredentialStore(db)
	require.NoError(t, err)
	defer store.Close()
	runStoreTests(t, store)
}

func TestBadgerStore(t *testing.T) {
	db := newTestBadgerDB(t)
	store := credentials.NewBadgerCredentialStore(db.DB())
	defer store.Close()
	runStoreTests(t, store)
}

func TestGenerateAccessKeyID(t *testing.T) {
	key := credentials.GenerateAccessKeyID()
	assert.True(t, strings.HasPrefix(key, "AKIA"))
	assert.Len(t, key, 20) // "AKIA" + 16 chars
}

func TestGenerateAccessKeyIDUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key := credentials.GenerateAccessKeyID()
		assert.False(t, seen[key], "duplicate key generated: %s", key)
		seen[key] = true
	}
}

func TestGenerateSecretAccessKey(t *testing.T) {
	secret := credentials.GenerateSecretAccessKey()
	assert.Len(t, secret, 40) // 20 bytes → 40 hex chars
}

func TestGenerateSecretAccessKeyUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		secret := credentials.GenerateSecretAccessKey()
		assert.False(t, seen[secret], "duplicate secret generated")
		seen[secret] = true
	}
}

func TestGenerateID(t *testing.T) {
	id := credentials.GenerateID()
	assert.Len(t, id, 16) // 8 bytes → 16 hex chars
}

func TestGenerateIDUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := credentials.GenerateID()
		assert.False(t, seen[id], "duplicate ID generated: %s", id)
		seen[id] = true
	}
}

func TestListMultipleCredentials(t *testing.T) {
	store := credentials.NewMemoryCredentialStore()
	defer store.Close()

	for i := 0; i < 5; i++ {
		_, err := store.Create("label", "us-east-1", "123456789012")
		require.NoError(t, err)
	}

	list, err := store.List()
	require.NoError(t, err)
	assert.Len(t, list, 5)

	// All secrets should be empty
	for _, c := range list {
		assert.Empty(t, c.SecretAccessKey)
	}
}

func TestCreateMultipleLabels(t *testing.T) {
	store := credentials.NewMemoryCredentialStore()
	defer store.Close()

	cred1, err := store.Create("prod", "us-east-1", "111111111111")
	require.NoError(t, err)

	cred2, err := store.Create("dev", "eu-west-1", "222222222222")
	require.NoError(t, err)

	assert.NotEqual(t, cred1.ID, cred2.ID)
	assert.NotEqual(t, cred1.AccessKeyID, cred2.AccessKeyID)
	assert.NotEqual(t, cred1.SecretAccessKey, cred2.SecretAccessKey)
	assert.Equal(t, "prod", cred1.Label)
	assert.Equal(t, "dev", cred2.Label)
	assert.Equal(t, "eu-west-1", cred2.Region)
	assert.Equal(t, "222222222222", cred2.AccountID)
}
