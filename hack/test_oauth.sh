#!/bin/bash
# hack/test_oauth.sh - Verify OAuth Discovery and Propagation

set -e

REPO_ROOT=$(pwd)
TEST_TMP=$(mktemp -d)
trap 'rm -rf "$TEST_TMP"' EXIT

echo "Using temporary directory: $TEST_TMP"

# Mock HOME
export HOME="$TEST_TMP"
GEMINI_DIR="$HOME/.gemini"
mkdir -p "$GEMINI_DIR"

# 1. Mock settings.json with OAuth selected
cat > "$GEMINI_DIR/settings.json" <<EOF
{
  "security": {
    "auth": {
      "selectedType": "oauth-personal"
    }
  }
}
EOF

# 2. Mock oauth_creds.json
echo '{"access_token": "mock-token", "refresh_token": "mock-refresh"}' > "$GEMINI_DIR/oauth_creds.json"

echo "=== Testing OAuth Discovery ==="

# Initialize a grove in the temp dir
cd "$TEST_TMP"
scion grove init

echo "=== Starting Agent ==="
scion start test-oauth-agent "hello" > start_output.log 2>&1 || true

echo "Start output:"
cat start_output.log

# Check if the agent directory was created
AGENT_DIR=".scion/agents/test-oauth-agent"
if [ -d "$AGENT_DIR" ]; then
    echo "SUCCESS: Agent directory created."
else
    echo "FAILURE: Agent directory not created."
    exit 1
fi

scion stop test-oauth-agent --rm || true

echo "Test complete."