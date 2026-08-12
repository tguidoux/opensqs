package tls_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tlsconfig "github.com/tguidoux/opensqs/apps/go/server/tls"
)

// generateTestCert creates a self-signed certificate and key pair for testing.
func generateTestCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	tmpDir := t.TempDir()
	certFile = filepath.Join(tmpDir, "cert.pem")
	keyFile = filepath.Join(tmpDir, "key.pem")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	err = os.WriteFile(certFile, certPEM, 0644)
	require.NoError(t, err)

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	err = os.WriteFile(keyFile, keyPEM, 0600)
	require.NoError(t, err)

	return certFile, keyFile
}

func TestLoadTLSConfigDisabled(t *testing.T) {
	cfg, err := tlsconfig.LoadTLSConfig("", "")
	require.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestLoadFromConfigDisabled(t *testing.T) {
	cfg, err := tlsconfig.LoadFromConfig(tlsconfig.Config{Enabled: false})
	require.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestLoadTLSConfigMissingCert(t *testing.T) {
	_, err := tlsconfig.LoadTLSConfig("", "key.pem")
	assert.Error(t, err)
}

func TestLoadTLSConfigMissingKey(t *testing.T) {
	_, err := tlsconfig.LoadTLSConfig("cert.pem", "")
	assert.Error(t, err)
}

func TestLoadTLSConfigValid(t *testing.T) {
	certFile, keyFile := generateTestCert(t)

	cfg, err := tlsconfig.LoadTLSConfig(certFile, keyFile)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Len(t, cfg.Certificates, 1)
}

func TestLoadFromConfigEnabled(t *testing.T) {
	certFile, keyFile := generateTestCert(t)

	cfg, err := tlsconfig.LoadFromConfig(tlsconfig.Config{
		Enabled:  true,
		CertFile: certFile,
		KeyFile:  keyFile,
	})
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Len(t, cfg.Certificates, 1)
}

func TestLoadTLSConfigInvalidFiles(t *testing.T) {
	_, err := tlsconfig.LoadTLSConfig("/nonexistent/cert.pem", "/nonexistent/key.pem")
	assert.Error(t, err)
}
