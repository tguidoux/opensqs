package config

import environment "github.com/tguidoux/opensqs/pkgs/v1/environment"

const (
	LocalAWSS3EndpointURL  = "http://localhost:9000"
	LocalAWSSQSEndpointURL = "http://localhost:9324"
	LocalAWSSSMEndpointURL = "http://localhost:8000"

	AoostarAWSS3EndpointURL  = "http://minio.aoostar.local:9000"
	AoostarAWSSQSEndpointURL = "http://opensqs.aoostar.local:9324"
	AoostarAWSSSMEndpointURL = "http://ssm.aoostar.local:8000"
)

func GetRegion(env environment.Environment) string {
	switch env {
	case environment.AOOSTAR:
		return ""
	case environment.LOCAL:
		return ""
	default:
		return "us-east-1"
	}
}

// GetSSMRegion returns the same region as GetRegion.
// Kept as a separate function for semantic clarity.
func GetSSMRegion(env environment.Environment) string {
	return GetRegion(env)
}

// GetS3Region always returns "us-east-1" since S3 is global.
func GetS3Region(env environment.Environment) string {
	return "us-east-1"
}

func GetSQSRegion(env environment.Environment) string {
	return GetRegion(env)
}

func GetAWSS3EndpointURL(env environment.Environment) string {
	switch env {
	case environment.AOOSTAR:
		return AoostarAWSS3EndpointURL
	case environment.LOCAL:
		return LocalAWSS3EndpointURL
	default:
		return ""
	}
}

func GetAWSSQSEndpointURL(env environment.Environment) string {
	switch env {
	case environment.AOOSTAR:
		return AoostarAWSSQSEndpointURL
	case environment.LOCAL:
		return LocalAWSSQSEndpointURL
	default:
		return ""
	}
}

func GetAWSSSMEndpointURL(env environment.Environment) string {
	switch env {
	case environment.AOOSTAR:
		return AoostarAWSSSMEndpointURL
	case environment.LOCAL:
		return LocalAWSSSMEndpointURL
	default:
		return ""
	}
}
