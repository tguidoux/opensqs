# Docker Examples

Docker examples for running OpenSQS as a containerized service.

## Available Examples

### 1. No Authentication (`Dockerfile.no-auth`)

Runs OpenSQS with `auth.enabled: false`. Any client can send and receive messages without providing credentials.

**Use when:**
- Local development where you don't want to manage credentials
- CI/CD pipelines that need a quick throwaway queue
- Testing and prototyping

```bash
# Build
docker build -f examples/docker/Dockerfile.no-auth -t opensqs-no-auth .

# Run
docker run -p 9324:9324 -p 9325:9325 opensqs-no-auth

# Test (no credentials needed)
export AWS_ENDPOINT_URL=http://localhost:9324
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1
aws sqs create-queue --queue-name my-queue
```

### 2. With Initial Credentials (`Dockerfile.with-creds`)

Runs OpenSQS with `auth.enabled: true` and pre-seeded credentials. The server starts with a known access key / secret key pair so your AWS CLI or SDK can authenticate from the first request.

**Use when:**
- Local development with realistic auth flow
- Integration tests that need stable credentials
- Teams that want to share a known credential set

```bash
# Build
docker build -f examples/docker/Dockerfile.with-creds -t opensqs-with-creds .

# Run
docker run -p 9324:9324 -p 9325:9325 opensqs-with-creds

# Test with the pre-seeded credentials
export AWS_ENDPOINT_URL=http://localhost:9324
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
export AWS_DEFAULT_REGION=us-east-1
aws sqs create-queue --queue-name my-queue
```

## Using the Pre-built Image Directly

You can skip the Dockerfile and mount a config file directly:

```bash
# No auth
docker run -p 9324:9324 -p 9325:9325 \
  -v $(pwd)/examples/docker/config.no-auth.yaml:/etc/opensqs/config.yaml:ro \
  ghcr.io/tguidoux/opensqs/opensqs-server:latest \
  --config /etc/opensqs/config.yaml

# With creds
docker run -p 9324:9324 -p 9325:9325 \
  -v $(pwd)/examples/docker/config.with-creds.yaml:/etc/opensqs/config.yaml:ro \
  ghcr.io/tguidoux/opensqs/opensqs-server:latest \
  --config /etc/opensqs/config.yaml
```

## Configuration Files

| File | Auth | Storage | Description |
|------|------|---------|-------------|
| `config.no-auth.yaml` | Disabled | Memory | No credentials required |
| `config.with-creds.yaml` | Enabled (pre-seeded) | Memory | Known key pair from startup |

## Customizing

To use your own credentials, edit `config.with-creds.yaml`:

```yaml
auth:
  enabled: true
  initialCredentials:
    - label: "my-team-key"
      accessKeyId: "YOUR_ACCESS_KEY_ID"
      secretAccessKey: "YOUR_SECRET_ACCESS_KEY"
```

For persistent storage, change `storageType` to `sqlite` or `badger` and mount a volume:

```bash
docker run -p 9324:9324 -p 9325:9325 \
  -v $(pwd)/my-config.yaml:/etc/opensqs/config.yaml:ro \
  -v opensqs-data:/data \
  ghcr.io/tguidoux/opensqs/opensqs-server:latest \
  --config /etc/opensqs/config.yaml
```

## Ports

| Port | Service |
|------|---------|
| 9324 | SQS API |
| 9325 | Web UI |
| 9326 | Metrics (if enabled in config) |

## See Also

- [Docker Compose Examples](../docker-compose/) — Multi-service orchestration
- [Configuration Reference](../../docs/configuration.md) — All config options
- [Quick Start Commands](../commands/quick-start.sh) — AWS CLI examples
