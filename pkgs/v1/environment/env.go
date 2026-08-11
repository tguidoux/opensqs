// Package env defines the Environment type used across OpenSQS services
// to distinguish between LOCAL, STAGING, and PROD deployments.
package env

type Environment string

const (
	PROD    Environment = "prod"
	STAGING Environment = "staging"
	LOCAL   Environment = "local"
)
