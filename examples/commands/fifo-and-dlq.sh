#!/usr/bin/env bash
#
# OpenSQS — FIFO queue and Dead-Letter Queue (DLQ) examples
#
# Purpose:
#   Demonstrates creating a FIFO queue with a DLQ, sending ordered
#   messages within a message group, receiving them in order, and
#   testing the redrive policy when maxReceiveCount is exceeded.
#
# Usage:
#   chmod +x examples/commands/fifo-and-dlq.sh
#   ./examples/commands/fifo-and-dlq.sh
#
# See: examples/commands/README.md

set -euo pipefail

OPENSQS_HOST="localhost"
OPENSQS_PORT="9324"
ACCOUNT_ID="123456789012"
REGION="us-east-1"

# Credentials — match the ones in examples/docker/config.with-creds.yaml
# If running without auth, these can be any dummy value.
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-AKIAIOSFODNN7EXAMPLE}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY}"

ENDPOINT="http://${OPENSQS_HOST}:${OPENSQS_PORT}"

echo "============================================"
echo "  OpenSQS FIFO + DLQ Example"
echo "============================================"
echo ""

# ─── 1. Create the dead-letter queue (FIFO) ──────────────────────────────────

echo ">>> Creating DLQ 'orders-dlq.fifo'..."
DLQ_URL=$(aws --endpoint-url "${ENDPOINT}" \
  --region "${REGION}" \
  sqs create-queue \
  --queue-name orders-dlq.fifo \
  --attributes "FifoQueue=true,ContentBasedDeduplication=true" \
  --query 'QueueUrl' --output text)
echo "    DLQ URL: ${DLQ_URL}"
echo ""

# ─── 2. Get the DLQ ARN ──────────────────────────────────────────────────────

DLQ_ARN=$(aws --endpoint-url "${ENDPOINT}" \
  --region "${REGION}" \
  sqs get-queue-attributes \
  --queue-url "${DLQ_URL}" \
  --attribute-names QueueArn \
  --query 'Attributes.QueueArn' --output text)
echo "    DLQ ARN: ${DLQ_ARN}"
echo ""

# ─── 3. Create the main FIFO queue with a redrive policy ─────────────────────

echo ">>> Creating FIFO queue 'orders.fifo' with redrive policy..."

# Build the redrive policy JSON
REDDRIVE_POLICY="{\"deadLetterTargetArn\":\"${DLQ_ARN}\",\"maxReceiveCount\":\"3\"}"

# Use a temp file for attributes to avoid shell quoting issues with the JSON redrive policy
ATTRS_FILE=$(mktemp)
python3 -c "
import json
attrs = {
    'FifoQueue': 'true',
    'ContentBasedDeduplication': 'true',
    'RedrivePolicy': '${REDDRIVE_POLICY}'
}
print(json.dumps(attrs))
" > "${ATTRS_FILE}"

QUEUE_URL=$(aws --endpoint-url "${ENDPOINT}" \
  --region "${REGION}" \
  sqs create-queue \
  --queue-name orders.fifo \
  --attributes file://"${ATTRS_FILE}" \
  --query 'QueueUrl' --output text)
rm -f "${ATTRS_FILE}"
echo "    Queue URL: ${QUEUE_URL}"
echo ""

# ─── 4. Send ordered messages in the same message group ──────────────────────

echo ">>> Sending 3 ordered messages (messageGroupId: order-batch-1)..."
for i in 1 2 3; do
  MSG_ID=$(aws --endpoint-url "${ENDPOINT}" \
    --region "${REGION}" \
    sqs send-message \
    --queue-url "${QUEUE_URL}" \
    --message-body "{\"step\":${i},\"action\":\"process\"}" \
    --message-group-id "order-batch-1" \
    --message-deduplication-id "msg-${i}" \
    --query 'MessageId' --output text)
  echo "    Sent message ${i}: ${MSG_ID}"
done
echo ""

# ─── 5. Receive messages (should arrive in order) ────────────────────────────

echo ">>> Receiving messages (should be in order 1, 2, 3)..."
for i in 1 2 3; do
  MSG=$(aws --endpoint-url "${ENDPOINT}" \
    --region "${REGION}" \
    sqs receive-message \
    --queue-url "${QUEUE_URL}" \
    --max-number-of-messages 1 \
    --wait-time-seconds 5)
  BODY=$(echo "${MSG}" | jq -r '.Messages[0].Body')
  echo "    Received: ${BODY}"

  # Don't delete — let visibility timeout expire to trigger DLQ
  RECEIPT=$(echo "${MSG}" | jq -r '.Messages[0].ReceiptHandle')

  # Change visibility to 0 to make it immediately visible again
  # (simulating failed processing)
  aws --endpoint-url "${ENDPOINT}" \
    --region "${REGION}" \
    sqs change-message-visibility \
    --queue-url "${QUEUE_URL}" \
    --receipt-handle "${RECEIPT}" \
    --visibility-timeout 0 > /dev/null 2>&1 || true
done
echo ""

echo ">>> After 3 receives without deletion, messages should be in the DLQ."
echo "    Check the DLQ in the Web UI: http://${OPENSQS_HOST}:9325"
echo ""

# ─── 6. Clean up ─────────────────────────────────────────────────────────────

echo ">>> Cleaning up..."
aws --endpoint-url "${ENDPOINT}" \
  --region "${REGION}" \
  sqs delete-queue --queue-url "${QUEUE_URL}" 2>/dev/null || true
aws --endpoint-url "${ENDPOINT}" \
  --region "${REGION}" \
  sqs delete-queue --queue-url "${DLQ_URL}" 2>/dev/null || true
echo "    Done."
echo ""

echo "============================================"
echo "  FIFO + DLQ example complete!"
echo "============================================"
