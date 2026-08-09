package main

import (
	"fmt"

	tlsconfig "github.com/tguidoux/opensqs/apps/go/server/tls"
	environment "github.com/tguidoux/opensqs/pkgs/v1/environment"
)

// ServerConfig holds all configuration for the SQS server.
type ServerConfig struct {
	// Server is the HTTP server configuration.
	Server ServerHTTPConfig `yaml:"server"`
	// SQS is the SQS-specific configuration.
	SQS SQSConfig `yaml:"sqs"`
	// Log is the logging configuration.
	Log LogConfig `yaml:"log"`
	// Health is the health check server configuration.
	Health HealthConfig `yaml:"health"`
	// UI is the web dashboard configuration.
	UI UIConfig `yaml:"ui"`
	// Metrics is the Prometheus metrics server configuration.
	Metrics MetricsConfig `yaml:"metrics"`
	// Environment is the deployment environment.
	Environment environment.Environment `yaml:"environment"`
	// Queues holds startup queue configuration and auto-create settings.
	Queues QueuesConfig `yaml:"queues"`
	// RequestLogging controls structured HTTP request logging.
	RequestLogging RequestLoggingConfig `yaml:"requestLogging"`
	// RateLimit controls per-queue and global rate limiting.
	RateLimit RateLimitConfig `yaml:"rateLimit"`
}

// QueuesConfig holds queue startup and auto-create configuration.
type QueuesConfig struct {
	// AutoCreate enables automatic queue creation on first access.
	AutoCreate bool `yaml:"autoCreate"`
	// Startup is a list of queues to create at startup.
	Startup []StartupQueue `yaml:"startup"`
}

// HealthConfig holds health check server settings.
type HealthConfig struct {
	// Port is the port for the health check server (default: 8001).
	Port int `yaml:"port"`
	// TLS holds TLS configuration for the health check server.
	TLS TLSConfig `yaml:"tls"`
}

// UIConfig holds web dashboard settings.
type UIConfig struct {
	// Enabled controls whether the UI server starts.
	Enabled bool `yaml:"enabled"`
	// Port is the port for the UI server (default: 9325).
	Port int `yaml:"port"`
	// TLS holds TLS configuration for the UI server.
	TLS TLSConfig `yaml:"tls"`
}

// MetricsConfig holds Prometheus metrics server settings.
type MetricsConfig struct {
	// Enabled controls whether the metrics server starts.
	Enabled bool `yaml:"enabled"`
	// Port is the port for the metrics server (default: 9326).
	Port int `yaml:"port"`
	// TLS holds TLS configuration for the metrics server.
	TLS TLSConfig `yaml:"tls"`
}

// StartupQueue defines a queue to be created when the server starts.
type StartupQueue struct {
	// Name is the queue name (required).
	Name string `yaml:"name"`
	// Attributes is an optional set of queue attributes.
	// Any unspecified attributes use SQS defaults.
	Attributes *StartupQueueAttributes `yaml:"attributes,omitempty"`
}

// StartupQueueAttributes holds optional queue attributes for startup queues.
// All fields are pointers so we can distinguish "not set" from "set to zero".
type StartupQueueAttributes struct {
	VisibilityTimeout             *int  `yaml:"visibilityTimeout,omitempty"`
	DelaySeconds                  *int  `yaml:"delaySeconds,omitempty"`
	MaximumMessageSize            *int  `yaml:"maximumMessageSize,omitempty"`
	MessageRetentionPeriod        *int  `yaml:"messageRetentionPeriod,omitempty"`
	ReceiveMessageWaitTimeSeconds *int  `yaml:"receiveMessageWaitTimeSeconds,omitempty"`
	FifoQueue                     *bool `yaml:"fifoQueue,omitempty"`
	ContentBasedDeduplication     *bool `yaml:"contentBasedDeduplication,omitempty"`
}

// ServerHTTPConfig holds HTTP server settings.
type ServerHTTPConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	// TLS holds TLS configuration for the main SQS API server.
	TLS TLSConfig `yaml:"tls"`
}

// SQSConfig holds SQS-specific settings.
type SQSConfig struct {
	// NodeAddress is the external address clients use to reach the server (host:port).
	NodeAddress string `yaml:"nodeAddress"`
	// AccountID is the AWS account ID used in queue URLs.
	AccountID string `yaml:"accountId"`
	// Region is the AWS region used in ARNs.
	Region string `yaml:"region"`
	// StorageType is the message store type ("memory", "sqlite", or "badger").
	StorageType string `yaml:"storageType"`
	// SQLitePath is the path to the SQLite database file (used when StorageType is "sqlite").
	SQLitePath string `yaml:"sqlitePath"`
	// BadgerPath is the directory path for BadgerDB (used when StorageType is "badger").
	BadgerPath string `yaml:"badgerPath"`
	// StrictLimits enforces SQS limits strictly (true) or relaxed (false).
	StrictLimits bool `yaml:"strictLimits"`
	// ServerSecret is the secret key used for signing receipt handles.
	ServerSecret string `yaml:"serverSecret"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level string `yaml:"level"`
}

// TLSConfig holds TLS/HTTPS configuration for an HTTP server.
// It is a thin wrapper around the tls package's Config struct
// so it can be embedded in config structs via YAML.
type TLSConfig struct {
	// Enabled controls whether TLS is used for this server.
	Enabled bool `yaml:"enabled"`
	// CertFile is the path to the TLS certificate file (PEM format).
	CertFile string `yaml:"certFile"`
	// KeyFile is the path to the TLS private key file (PEM format).
	KeyFile string `yaml:"keyFile"`
}

// ToTLSConfig converts the YAML config struct to the tls package's Config.
func (c TLSConfig) ToTLSConfig() tlsconfig.Config {
	return tlsconfig.Config{
		Enabled:  c.Enabled,
		CertFile: c.CertFile,
		KeyFile:  c.KeyFile,
	}
}

// RequestLoggingConfig controls structured HTTP request logging.
type RequestLoggingConfig struct {
	// Enabled controls whether request logging middleware is active.
	Enabled bool `yaml:"enabled"`
}

// RateLimitConfig controls per-queue and global rate limiting.
type RateLimitConfig struct {
	// Enabled controls whether rate limiting middleware is active.
	Enabled bool `yaml:"enabled"`
	// RequestsPerSecond is the maximum number of requests per second.
	RequestsPerSecond float64 `yaml:"requestsPerSecond"`
	// Burst is the maximum number of requests allowed in a burst.
	Burst int `yaml:"burst"`
	// PerQueue controls whether rate limiting is per-queue (true) or global (false).
	PerQueue bool `yaml:"perQueue"`
}

// Validate implements config.ConfigI[ServerConfig].
func (c ServerConfig) Validate() error {
	if c.SQS.AccountID == "" {
		return fmt.Errorf("sqs.accountId is required")
	}
	if c.SQS.Region == "" {
		return fmt.Errorf("sqs.region is required")
	}
	if c.SQS.NodeAddress == "" {
		return fmt.Errorf("sqs.nodeAddress is required")
	}
	switch c.SQS.StorageType {
	case "", "memory", "sqlite", "badger":
	default:
		return fmt.Errorf("sqs.storageType must be one of: memory, sqlite, badger (got %q)", c.SQS.StorageType)
	}
	if c.SQS.StorageType == "sqlite" && c.SQS.SQLitePath == "" {
		return fmt.Errorf("sqs.sqlitePath is required when storageType is sqlite")
	}
	if c.SQS.StorageType == "badger" && c.SQS.BadgerPath == "" {
		return fmt.Errorf("sqs.badgerPath is required when storageType is badger")
	}
	if c.Server.Port < 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 0 and 65535")
	}
	if c.SQS.ServerSecret == "" {
		return fmt.Errorf("sqs.serverSecret is required")
	}
	return nil
}

// WithValidation implements config.ConfigI[ServerConfig].
func (c ServerConfig) WithValidation() ServerConfig {
	return c
}
