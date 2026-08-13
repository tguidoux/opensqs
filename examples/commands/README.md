# Command Examples

Shell scripts with ready-to-use AWS CLI commands for interacting with OpenSQS. These scripts demonstrate the full SQS API workflow.

## Available Scripts

### 1. Quick Start (`quick-start.sh`)

Basic queue operations: create, send, receive, delete, list, get attributes, purge, and delete queue.

**Prerequisites:** OpenSQS running locally (see [Docker examples](../docker/) or [Docker Compose](../docker-compose/)).

```bash
# Start OpenSQS first (in another terminal)
docker compose -f examples/docker-compose/docker-compose.with-creds.yml up

# Run the quick start
chmod +x examples/commands/quick-start.sh
./examples/commands/quick-start.sh
```

**What it does:**
1. Creates a queue named `my-queue`
2. Sends a message with a message attribute
3. Receives the message
4. Deletes the message using the receipt handle
5. Lists all queues
6. Gets queue attributes
7. Purges the queue
8. Deletes the queue

### 2. FIFO + DLQ (`fifo-and-dlq.sh`)

Demonstrates FIFO queues with message groups, ordered delivery, and dead-letter queue (DLQ) redrive policies.

**Prerequisites:** OpenSQS running locally.

```bash
chmod +x examples/commands/fifo-and-dlq.sh
./examples/commands/fifo-and-dlq.sh
```

**What it does:**
1. Creates a FIFO dead-letter queue (`orders-dlq.fifo`)
2. Creates a FIFO main queue (`orders.fifo`) with a redrive policy (maxReceiveCount: 3)
3. Sends 3 ordered messages in the same message group
4. Receives messages (should arrive in order)
5. Simulates failed processing by not deleting messages
6. After 3 receives, messages should be redrived to the DLQ
7. Cleans up both queues

## Environment Variables

All scripts use these environment variables (with defaults):

| Variable | Default | Description |
|----------|---------|-------------|
| `AWS_ACCESS_KEY_ID` | `AKIAIOSFODNN7EXAMPLE` | Access key for auth |
| `AWS_SECRET_ACCESS_KEY` | `wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY` | Secret key for auth |

If running OpenSQS without auth, these can be any dummy value.

Override them before running:

```bash
export AWS_ACCESS_KEY_ID="your-key"
export AWS_SECRET_ACCESS_KEY="your-secret"
./examples/commands/quick-start.sh
```

## Manual AWS CLI Commands

### Basic Operations

```bash
export AWS_ENDPOINT_URL=http://localhost:9324
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
export AWS_DEFAULT_REGION=us-east-1

# Create a queue
aws sqs create-queue --queue-name my-queue

# Send a message
aws sqs send-message \
  --queue-url http://localhost:9324/123456789012/my-queue \
  --message-body "Hello OpenSQS!"

# Receive a message
aws sqs receive-message \
  --queue-url http://localhost:9324/123456789012/my-queue

# Delete a message (use receipt handle from receive)
aws sqs delete-message \
  --queue-url http://localhost:9324/123456789012/my-queue \
  --receipt-handle "<receipt-handle>"

# List queues
aws sqs list-queues

# Purge a queue
aws sqs purge-queue \
  --queue-url http://localhost:9324/123456789012/my-queue

# Delete a queue
aws sqs delete-queue \
  --queue-url http://localhost:9324/123456789012/my-queue
```

### FIFO Queue

```bash
# Create a FIFO queue
aws sqs create-queue \
  --queue-name my-queue.fifo \
  --attributes "FifoQueue=true,ContentBasedDeduplication=true"

# Send an ordered message
aws sqs send-message \
  --queue-url http://localhost:9324/123456789012/my-queue.fifo \
  --message-body '{"step":1}' \
  --message-group-id "batch-1" \
  --message-deduplication-id "msg-1"
```

### Batch Operations

```bash
# Send a batch
aws sqs send-message-batch \
  --queue-url http://localhost:9324/123456789012/my-queue \
  --entries '[{"Id":"msg1","MessageBody":"first"},{"Id":"msg2","MessageBody":"second"}]'

# Delete a batch
aws sqs delete-message-batch \
  --queue-url http://localhost:9324/123456789012/my-queue \
  --entries '[{"Id":"msg1","ReceiptHandle":"<handle1>"},{"Id":"msg2","ReceiptHandle":"<handle2>"}]'
```

### Queue Attributes

```bash
# Get all attributes
aws sqs get-queue-attributes \
  --queue-url http://localhost:9324/123456789012/my-queue \
  --attribute-names All

# Set visibility timeout
aws sqs set-queue-attributes \
  --queue-url http://localhost:9324/123456789012/my-queue \
  --attributes VisibilityTimeout=60
```

### Tagging

```bash
# Tag a queue
aws sqs tag-queue \
  --queue-url http://localhost:9324/123456789012/my-queue \
  --tags Environment=dev,Team=backend

# List tags
aws sqs list-queue-tags \
  --queue-url http://localhost:9324/123456789012/my-queue

# Remove a tag
aws sqs untag-queue \
  --queue-url http://localhost:9324/123456789012/my-queue \
  --tag-keys Environment
```

## See Also

- [Docker Examples](../docker/) — Run OpenSQS in Docker
- [Docker Compose Examples](../docker-compose/) — Multi-service orchestration
- [API Reference](../../docs/api-reference.md) — Full SQS API documentation
- [Go Playground Examples](../../apps/go/playground/) — Go library usage examples
