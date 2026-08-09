package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tguidoux/opensqs/pkgs/v1/config"
	environment "github.com/tguidoux/opensqs/pkgs/v1/environment"
)

func TestGetRegion(t *testing.T) {
	tests := []struct {
		name     string
		env      environment.Environment
		expected string
	}{
		{
			name:     "LOCAL environment returns empty string",
			env:      environment.LOCAL,
			expected: "",
		},
		{
			name:     "AOOSTAR environment returns empty string",
			env:      environment.AOOSTAR,
			expected: "",
		},
		{
			name:     "STAGING environment returns us-east-1",
			env:      environment.STAGING,
			expected: "us-east-1",
		},
		{
			name:     "PROD environment returns us-east-1",
			env:      environment.PROD,
			expected: "us-east-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.GetRegion(tt.env)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSSMRegion(t *testing.T) {
	tests := []struct {
		name     string
		env      environment.Environment
		expected string
	}{
		{
			name:     "LOCAL environment returns empty string",
			env:      environment.LOCAL,
			expected: "",
		},
		{
			name:     "AOOSTAR environment returns empty string",
			env:      environment.AOOSTAR,
			expected: "",
		},
		{
			name:     "STAGING environment returns us-east-1",
			env:      environment.STAGING,
			expected: "us-east-1",
		},
		{
			name:     "PROD environment returns us-east-1",
			env:      environment.PROD,
			expected: "us-east-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.GetSSMRegion(tt.env)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetS3Region(t *testing.T) {
	tests := []struct {
		name     string
		env      environment.Environment
		expected string
	}{
		{
			name:     "LOCAL environment returns us-east-1",
			env:      environment.LOCAL,
			expected: "us-east-1",
		},
		{
			name:     "AOOSTAR environment returns us-east-1",
			env:      environment.AOOSTAR,
			expected: "us-east-1",
		},
		{
			name:     "STAGING environment returns us-east-1",
			env:      environment.STAGING,
			expected: "us-east-1",
		},
		{
			name:     "PROD environment returns us-east-1",
			env:      environment.PROD,
			expected: "us-east-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.GetS3Region(tt.env)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSQSRegion(t *testing.T) {
	tests := []struct {
		name     string
		env      environment.Environment
		expected string
	}{
		{
			name:     "LOCAL environment returns empty string",
			env:      environment.LOCAL,
			expected: "",
		},
		{
			name:     "AOOSTAR environment returns empty string",
			env:      environment.AOOSTAR,
			expected: "",
		},
		{
			name:     "STAGING environment returns us-east-1",
			env:      environment.STAGING,
			expected: "us-east-1",
		},
		{
			name:     "PROD environment returns us-east-1",
			env:      environment.PROD,
			expected: "us-east-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.GetSQSRegion(tt.env)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAWSS3EndpointURL(t *testing.T) {
	tests := []struct {
		name     string
		env      environment.Environment
		expected string
	}{
		{
			name:     "LOCAL environment returns local endpoint",
			env:      environment.LOCAL,
			expected: config.LOCAL_AWS_S3_ENDPOINT_URL,
		},
		{
			name:     "AOOSTAR environment returns aoostar endpoint",
			env:      environment.AOOSTAR,
			expected: config.AOOSTAR_AWS_S3_ENDPOINT_URL,
		},
		{
			name:     "STAGING environment returns empty string",
			env:      environment.STAGING,
			expected: "",
		},
		{
			name:     "PROD environment returns empty string",
			env:      environment.PROD,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.GetAWSS3EndpointURL(tt.env)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAWSSQSEndpointURL(t *testing.T) {
	tests := []struct {
		name     string
		env      environment.Environment
		expected string
	}{
		{
			name:     "LOCAL environment returns local endpoint",
			env:      environment.LOCAL,
			expected: config.LOCAL_AWS_SQS_ENDPOINT_URL,
		},
		{
			name:     "AOOSTAR environment returns aoostar endpoint",
			env:      environment.AOOSTAR,
			expected: config.AOOSTAR_AWS_SQS_ENDPOINT_URL,
		},
		{
			name:     "STAGING environment returns empty string",
			env:      environment.STAGING,
			expected: "",
		},
		{
			name:     "PROD environment returns empty string",
			env:      environment.PROD,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.GetAWSSQSEndpointURL(tt.env)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAWSSSMEndpointURL(t *testing.T) {
	tests := []struct {
		name     string
		env      environment.Environment
		expected string
	}{
		{
			name:     "LOCAL environment returns local endpoint",
			env:      environment.LOCAL,
			expected: config.LOCAL_AWS_SSM_ENDPOINT_URL,
		},
		{
			name:     "AOOSTAR environment returns aoostar endpoint",
			env:      environment.AOOSTAR,
			expected: config.AOOSTAR_AWS_SSM_ENDPOINT_URL,
		},
		{
			name:     "STAGING environment returns empty string",
			env:      environment.STAGING,
			expected: "",
		},
		{
			name:     "PROD environment returns empty string",
			env:      environment.PROD,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.GetAWSSSMEndpointURL(tt.env)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConstants(t *testing.T) {
	// Test that constants have expected values
	assert.Equal(t, "http://localhost:9000", config.LOCAL_AWS_S3_ENDPOINT_URL)
	assert.Equal(t, "http://localhost:9324", config.LOCAL_AWS_SQS_ENDPOINT_URL)
	assert.Equal(t, "http://localhost:8000", config.LOCAL_AWS_SSM_ENDPOINT_URL)

	assert.Equal(t, "http://192.168.1.119:9000", config.AOOSTAR_AWS_S3_ENDPOINT_URL)
	assert.Equal(t, "http://192.168.1.153:9324", config.AOOSTAR_AWS_SQS_ENDPOINT_URL)
	assert.Equal(t, "http://192.168.1.153:8000", config.AOOSTAR_AWS_SSM_ENDPOINT_URL)
}
