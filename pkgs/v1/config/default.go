package config

import environment "github.com/tguidoux/opensqs/pkgs/v1/environment"

const LocalAWSSQSEndpointURL = "http://localhost:9324"

func GetRegion(env environment.Environment) string {
	switch env {
	case environment.LOCAL:
		return ""
	default:
		return "us-east-1"
	}
}

func GetSQSRegion(env environment.Environment) string {
	return GetRegion(env)
}

func GetAWSSQSEndpointURL(env environment.Environment) string {
	switch env {
	case environment.LOCAL:
		return LocalAWSSQSEndpointURL
	default:
		return ""
	}
}
