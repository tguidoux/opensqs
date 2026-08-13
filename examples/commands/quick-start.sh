#!/usr/bin/env bash
#
# OpenSQS — Quick start commands
#
# Purpose:
#   A collection of ready-to-use commands for running OpenSQS locally,
#   testing with the AWS CLI, and managing queues/messages.
#
# Usage:
#   chmod +x examples/commands/quick-start.sh
#   ./examples/commands/quick-start.sh
#
# See: examples/commands/README.md

set -euo pipefail

# ─── Configuration ────────────────────────────────────────────────────────────

OPENSQS_HOST="localhost"
OPENSQS_PORT="9324"
ACCOUNT_ID="123456789012"
REGION="us-east-1"

# Credentials — match the ones in examples/docker/config.with-creds.yaml
# If running without auth, these can be any dummy value.
AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-AKIAIOSFODNN7EXAMPLE}"
AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY}"

ENDPOINT="http://${OPENSQS_HOST}:${OPENSQS_PORT}"

# ─── Helpers ─────────────────────────────────────────────────────────────────

echo "============================================"
echo "  OpenSQS Quick Start Commands"
echo "============================================"
echo ""
echo "Endpoint: ${ENDPOINT}"
echo "Account:  ${ACCOUNT_ID}"
echo "Region:   ${REGION}"
echo ""

# ─── 1. Create a queue ───────────────────────────────────────────────────────

echo ">>> Creating queue 'my-queue'..."
QUEUE_URL=$(aws --endpoint-url "${ENDPOINT}" \
  --region "${REGION}" \
  --access-key-id "${AWS_ACCESS_KEY_ID}" \
  --secret-access-key "${AWS_SECRET_ACCESS_KEY}" \
  sqs create-queue --queue-name my-queue \
  --query 'QueueUrl' --output text)
echo "    Queue URL: ${QUEUE_URL}"
echo ""

# ─── 2. Send a message ───────────────────────────────────────────────────────

echo ">>> Sending a message..."
SEND_RESULT=$(aws --endpoint-url "${ENDPOINT}" \
  --region "${REGION}" \
  --access-key-id "${AWS_ACCESS_KEY_ID}" \
  --secret-access-key "${AWS_SECRET_ACCESS_KEY}" \
  sqs send-message \
  --queue-url "${QUEUE_URL}" \
  --message-body '{"event":"order.created","orderId":"12345"}' \
  --message-attribute "EventType=String:order.created" \
  --query 'MessageId' --output text)
echo "    Message ID: ${SEND_RESULT}"
echo ""

# ─── 3. Receive a message ────────────────────────────────────────────────────

echo ">>> Receiving a message..."
RECEIVE_RESULT=$(aws --endpoint-url "${ENDPOINT}" \
  --region "${REGION}" \
  --access-key-id "${AWS_ACCESS_KEY_ID}" \
  --secret-access-key "${AWS_SECRET_ACCESS_KEY}" \
  sqs receive-message \
  --queue-url "${QUEUE_URL}" \
  --max-number-of-messages 1 \
  --wait-time-seconds 5)
echo "    Received: ${RECEIVE_RESULT}"
echo ""

# ─── 4. Delete the message ───────────────────────────────────────────────────

RECEIPT_HANDLE=$(echo "${RECEIVE_RESULT}" | jq -r '.Messages[0].ReceiptHandle' 2>/dev/null || echo "")
if [ -n "${RECEIPT_HANDLE}" ]; then
  echo ">>> Deleting the message..."
  aws --endpoint-url "${ENDPOINT}" \
    --region "${REGION}" \
    --access-key-id "${AWS_ACCESS_KEY_ID}" \
    --secret-access-key "${AWS_SECRET_ACCESS_KEY}" \
    sqs delete-message \
    --queue-url "${QUEUE_URL}" \
    --receipt-handle "${RECEIPT_HANDLE}"
  echo "    Deleted."
else
  echo ">>> No receipt handle found, skipping delete."
fi
echo ""

# ─── 5. List queues ──────────────────────────────────────────────────────────

echo ">>> Listing queues..."
aws --endpoint-url "${ENDPOINT}" \
  --region "${REGION}" \
  --access-key-id "${AWS_ACCESS_KEY_ID}" \
  --secret-access-key "${AWS_SECRET_ACCESS_KEY}" \
  sqs list-queues
echo ""

# ─── 6. Get queue attributes ─────────────────────────────────────────────────

echo ">>> Getting queue attributes..."
aws --endpoint-url "${ENDPOINT}" \
  --region "${REGION}" \
  --access-key-id "${AWS_ACCESS_KEY_ID}" \
  --secret-access-key "${AWS_SECRET_ACCESS_KEY}" \
  sqs get-queue-attributes \
  --queue-url "${QUEUE_URL}" \
  --attribute-names All
echo ""

# ─── 7. Purge the queue ─────────────────────────────────────────────────────

echo ">>> Purging the queue..."
aws --endpoint-url "${ENDPOINT}" \
  --region "${REGION}" \
  --access-key-id "${AWS_ACCESS_KEY_ID}" \
  --secret-access-key "${AWS_SECRET_ACCESS_KEY}" \
  sqs purge-queue --queue-url "${QUEUE_URL}"
echo "    Purged."
echo ""

# ─── 8. Delete the queue ─────────────────────────────────────────────────────

echo ">>> Deleting the queue..."
aws --endpoint-url "${ENDPOINT}" \
  --region "${REGION}" \
  --access-key-id "${AWS_ACCESS_KEY_ID}" \
  --secret-access-key "${AWS_SECRET_ACCESS_KEY}" \
  sqs delete-queue --queue-url "${QUEUE_URL}"
echo "    Deleted."
echo ""

echo "============================================"
echo "  Done! All commands completed successfully."
echo "============================================"
