package config

import environment "github.com/tguidoux/opensqs/pkgs/v1/environment"

const LOCAL_AWS_S3_ENDPOINT_URL string = "http://localhost:9000"
const LOCAL_AWS_SQS_ENDPOINT_URL string = "http://localhost:9324"
const LOCAL_AWS_SSM_ENDPOINT_URL string = "http://localhost:8000"

const AOOSTAR_AWS_S3_ENDPOINT_URL string = "http://minio.aoostar.local:9000"
const AOOSTAR_AWS_SQS_ENDPOINT_URL string = "http://opensqs.aoostar.local:9324"
const AOOSTAR_AWS_SSM_ENDPOINT_URL string = "http://ssm.aoostar.local:8000"

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

func GetSSMRegion(env environment.Environment) string {
	switch env {
	case environment.AOOSTAR:
		return ""
	case environment.LOCAL:
		return ""
	default:
		return "us-east-1"
	}
}

func GetS3Region(env environment.Environment) string {
	switch env {
	case environment.AOOSTAR:
		return "us-east-1"
	case environment.LOCAL:
		return "us-east-1"
	default:
		return "us-east-1"
	}
}

func GetSQSRegion(env environment.Environment) string {
	return GetRegion(env)
}

func GetAWSS3EndpointURL(env environment.Environment) string {
	switch env {
	case environment.AOOSTAR:
		return AOOSTAR_AWS_S3_ENDPOINT_URL
	case environment.LOCAL:
		return LOCAL_AWS_S3_ENDPOINT_URL
	default:
		return ""
	}
}

func GetAWSSQSEndpointURL(env environment.Environment) string {
	switch env {
	case environment.AOOSTAR:
		return AOOSTAR_AWS_SQS_ENDPOINT_URL
	case environment.LOCAL:
		return LOCAL_AWS_SQS_ENDPOINT_URL
	default:
		return ""
	}
}

func GetAWSSSMEndpointURL(env environment.Environment) string {
	switch env {
	case environment.AOOSTAR:
		return AOOSTAR_AWS_SSM_ENDPOINT_URL
	case environment.LOCAL:
		return LOCAL_AWS_SSM_ENDPOINT_URL
	default:
		return ""
	}
}
