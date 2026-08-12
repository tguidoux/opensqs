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

func TestGetAWSSQSEndpointURL(t *testing.T) {
	tests := []struct {
		name     string
		env      environment.Environment
		expected string
	}{
		{
			name:     "LOCAL environment returns local endpoint",
			env:      environment.LOCAL,
			expected: config.LocalAWSSQSEndpointURL,
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

func TestConstants(t *testing.T) {
	assert.Equal(t, "http://localhost:9324", config.LocalAWSSQSEndpointURL)
}
