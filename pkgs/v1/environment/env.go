// Package env defines the Environment type used across OpenSQS services
// to distinguish between LOCAL, STAGING, PROD, and AOOSTAR deployments.
package env

type Environment string

const (
	PROD    Environment = "prod"
	STAGING Environment = "staging"
	LOCAL   Environment = "local"
	AOOSTAR Environment = "aoostar"
)
