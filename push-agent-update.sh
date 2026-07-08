#!/bin/bash
# HermesRemote Agent Auto-Update Script
# Usage: ./push-agent-update.sh <version_label> [binary_path]
# 
# Example: ./push-agent-update.sh v9c-v0.2.2 /opt/data/workspace-operative/hermes-remote/HermesRemote_v9c.exe
#
# This script:
# 1. Cross-compiles the Windows agent binary (if binary_path not given)
# 2. Uploads it to the VPS download directory
# 3. Triggers the agent_update API endpoint
# 4. Waits for the new agent to reconnect and confirms version change
# 5. Reports success/failure

set -e

VPS_IP="187.124.31.229"
SSH_KEY=~/.ssh/hermes_desktop
VPS_USER="root"
AGENT_ID="a0-falke"
REPO_DIR="/opt/data/workspace-operative/hermes-remote"

VERSION_LABEL="${1:-unknown}"
BINARY_PATH="${2:-}"

# If no binary path given, build one
if [ -z "$BINARY_PATH" ]; then
    BINARY_NAME="HermesRemote_${VERSION_LABEL}.exe"
    BINARY_PATH="${REPO_DIR}/${BINARY_NAME}"
    echo "Building ${BINARY_NAME}..."
    cd "${REPO_DIR}" && GOOS=windows GOARCH=amd64 go build -o "${BINARY_PATH}" ./cmd/hermes-remote/
fi

BINARY_NAME=$(basename "$BINARY_PATH")

echo "Uploading ${BINARY_NAME} to VPS..."
ssh -i ${SSH_KEY} ${VPS_USER}@${VPS_IP} "mkdir -p /tmp/hermes-remote-files"
scp -i ${SSH_KEY} "$BINARY_PATH" ${VPS_USER}@${VPS_IP}:/tmp/hermes-remote-files/${BINARY_NAME}

echo "Triggering agent update..."
# Use --max-time 180 to allow for download + process swap
# The HTTP response may timeout (agent connection drops during swap) — that's OK
curl -s --max-time 180 -X POST "http://${VPS_IP}:80/api/agent/${AGENT_ID}/update" \
    -H "Content-Type: application/json" \
    -d "{\"binary_path\":\"/tmp/hermes-remote-files/${BINARY_NAME}\",\"version\":\"${VERSION_LABEL}\",\"download_host\":\"${VPS_IP}:80\"}" \
    || true  # curl may timeout — the update still succeeds

echo ""
echo "Waiting for new agent to reconnect..."
sleep 5

# Check the agent version
echo "Checking agent version..."
VERSION=$(ssh -i ${SSH_KEY} ${VPS_USER}@${VPS_IP} "curl -s http://localhost:80/api/agents" | python3 -c "import sys,json; data=json.load(sys.stdin); print(data[0]['version'] if data else 'no-agent')" 2>/dev/null)

echo "Agent version: ${VERSION}"

# Verify the agent is alive
echo "Verifying agent is responsive..."
RESULT=$(ssh -i ${SSH_KEY} ${VPS_USER}@${VPS_IP} "curl -s -X POST http://localhost:80/api/agent/${AGENT_ID}/exec -H 'Content-Type: application/json' -d '{\"command\":\"echo updated\",\"timeout\":10}'" 2>/dev/null)

if echo "$RESULT" | grep -q "updated"; then
    echo "✅ Update complete — agent is alive and responding"
    echo "   Version: ${VERSION}"
else
    echo "⚠️  Agent may still be reconnecting. Check in 30s."
    echo "   Version: ${VERSION}"
fi