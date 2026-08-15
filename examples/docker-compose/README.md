# Docker Compose Examples

Docker Compose configurations for running OpenSQS with various storage and auth options.

## Available Examples

### 1. No Authentication (`docker-compose.no-auth.yml`)

Quick-start OpenSQS with auth disabled. Any client can send and receive messages without credentials.

**Use when:** Local development, prototyping, CI where you just need a working SQS endpoint.

```bash
docker compose -f examples/docker-compose/docker-compose.no-auth.yml up
```

```bash
# Test (no credentials needed)
export AWS_ENDPOINT_URL=http://localhost:9324
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1
aws sqs create-queue --queue-name my-queue
```

### 2. With Initial Credentials (`docker-compose.with-creds.yml`)

OpenSQS with authentication enabled and pre-seeded credentials. Your AWS CLI or SDK can use the known key pair from the first request.

**Use when:** Local development with realistic auth, integration tests with stable credentials.

```bash
docker compose -f examples/docker-compose/docker-compose.with-creds.yml up
```

```bash
# Test with pre-seeded credentials
export AWS_ENDPOINT_URL=http://localhost:9324
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
export AWS_DEFAULT_REGION=us-east-1
aws sqs create-queue --queue-name my-queue
```

### 3. SQLite Persistence (`docker-compose.sqlite.yml`)

OpenSQS with SQLite persistent storage and pre-seeded credentials. Messages and queues survive container restarts.

**Use when:** Development environments that need durability, testing message persistence.

```bash
docker compose -f examples/docker-compose/docker-compose.sqlite.yml up
```

Data is stored in a named Docker volume (`opensqs-data`) at `/data/opensqs.db`. To reset:

```bash
docker compose -f examples/docker-compose/docker-compose.sqlite.yml down -v
```

## Configuration Files

| File | Auth | Storage | Persistence |
|------|------|---------|-------------|
| `config.no-auth.yaml` | Disabled | Memory | No (data lost on restart) |
| `config.with-creds.yaml` | Enabled (pre-seeded) | Memory | No (data lost on restart) |
| `config.sqlite.yaml` | Enabled (pre-seeded) | SQLite | Yes (named volume) |

## Endpoints

All compose files expose:

| Port | Service | URL |
|------|---------|-----|
| 9324 | SQS API | `http://localhost:9324` |
| 9325 | Web UI | `http://localhost:9325` |
| 9326 | Metrics | `http://localhost:9326/metrics` |

## Running in the Background

```bash
# Start in detached mode
docker compose -f examples/docker-compose/docker-compose.with-creds.yml up -d

# View logs
docker compose -f examples/docker-compose/docker-compose.with-creds.yml logs -f

# Stop
docker compose -f examples/docker-compose/docker-compose.with-creds.yml down
```

## Customizing

### Using Your Own Credentials

Edit `config.with-creds.yaml` or `config.sqlite.yaml`:

```yaml
auth:
  enabled: true
  initialCredentials:
    - label: "my-team-key"
      accessKeyId: "YOUR_ACCESS_KEY_ID"
      secretAccessKey: "YOUR_SECRET_ACCESS_KEY"
```

### Using BadgerDB Instead of SQLite

Edit `config.sqlite.yaml`:

```yaml
sqs:
  storageType: "badger"
  badgerPath: "/data/badger"
  # Remove: sqlitePath
```

### Adding Startup Queues

Edit any config file:

```yaml
queues:
  autoCreate: false
  startup:
    - name: "orders"
      attributes:
        visibilityTimeout: 60
    - name: "notifications"
    - name: "dead-letter.fifo"
      attributes:
        fifoQueue: true
        contentBasedDeduplication: true
```

## See Also

- [Docker Examples](../docker/) — Single-container Dockerfiles
- [Kubernetes Examples](../kubernetes/) — K8s manifests
- [Configuration Reference](../../docs/configuration.md) — All config options
