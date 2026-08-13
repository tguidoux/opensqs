#!/usr/bin/env bash
#
# OpenSQS — Run with Bazel (from source)
#
# Purpose:
#   Commands for building and running OpenSQS from source using Bazel.
#   No Docker or Kubernetes required — just Bazelisk.
#
# Prerequisites:
#   - Bazelisk (brew install bazelisk)
#   - AWS CLI (for testing)
#
# See: examples/commands/README.md

set -euo pipefail

echo "============================================"
echo "  OpenSQS — Bazel Build & Run"
echo "============================================"
echo ""

# ─── 1. Clone and initialize ─────────────────────────────────────────────────

echo ">>> Cloning and initializing workspace..."
echo "    git clone https://github.com/tguidoux/opensqs.git"
echo "    cd opensqs"
echo "    bazel run //:clean"
echo ""

# ─── 2. Run the server ───────────────────────────────────────────────────────

echo ">>> Starting OpenSQS server..."
echo "    bazel run //apps/go/server:opensqs-server"
echo ""
echo "    Server:   http://localhost:9324"
echo "    Web UI:   http://localhost:9325"
echo "    Metrics:  http://localhost:9326/metrics"
echo ""

# ─── 3. Run the Go playground examples ──────────────────────────────────────

echo ">>> Available Go examples:"
echo ""
echo "    # Basic: create, send, receive, delete"
echo "    bazel run //apps/go/playground/sqs_example:sqs_example"
echo ""
echo "    # Phase 2: attributes, batch ops, tagging"
echo "    bazel run //apps/go/playground/sqs_phase2_example:sqs_phase2_example"
echo ""
echo "    # FIFO: message groups, deduplication, ordering"
echo "    bazel run //apps/go/playground/sqs_fifo_example:sqs_fifo_example"
echo ""
echo "    # DLQ: redrive policy, maxReceiveCount, message redrive"
echo "    bazel run //apps/go/playground/sqs_dlq_example:sqs_dlq_example"
echo ""

# ─── 4. Build and load Docker image ──────────────────────────────────────────

echo ">>> Building and loading Docker image..."
echo "    bazel run //apps/go/server:opensqs_server_image_platform_transition_load_docker"
echo "    docker run -p 9324:9324 -p 9325:9325 opensqs_server_image"
echo ""

# ─── 5. Run tests ────────────────────────────────────────────────────────────

echo ">>> Running tests..."
echo "    bazel test //...                                    # All tests"
echo "    bazel test //pkgs/v1/queue/tests:go_default_test    # Queue tests"
echo "    bazel test --test_output=all //pkgs/v1/queue/tests:go_default_test  # Verbose"
echo ""

echo "============================================"
echo "  See examples/commands/README.md for more"
echo "============================================"
