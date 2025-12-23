#!/bin/bash
# hack/test_auth.sh - Verify Auth Discovery

REPO_ROOT=$(pwd)
TEST_DIR="${REPO_ROOT}/../qa-scion"
SCION_GROVE="${TEST_DIR}/.scion"

# Load the test key
if [ ! -f "${REPO_ROOT}/TEST_GEMINI_KEY" ]; then
    echo "TEST_GEMINI_KEY file not found in repo root."
    exit 1
fi
TEST_KEY=$(cat "${REPO_ROOT}/TEST_GEMINI_KEY")

if [ ! -d "${TEST_DIR}" ]; then
    echo "Test directory not found. Run hack/setup.sh first."
    exit 1
fi

echo "=== Testing Case A: Environment Variable ==="
export GEMINI_API_KEY="${TEST_KEY}"
scion -g "$SCION_GROVE" start qa-auth-env "test auth"

# Verify using container list (assuming Apple container on macOS)
if scion -g "$SCION_GROVE" list | grep -q "qa-auth-env"; then
    echo "Agent qa-auth-env started."
    # Check if env var is in the container list output
    if container list -a --format json | grep -q "GEMINI_API_KEY=${TEST_KEY}"; then
        echo "SUCCESS: GEMINI_API_KEY propagated correctly."
    else
        echo "FAILURE: GEMINI_API_KEY not found in container environment."
        exit 1
    fi
else
    echo "FAILURE: Agent qa-auth-env failed to start."
    exit 1
fi

scion -g "$SCION_GROVE" stop qa-auth-env --rm

echo "=== Testing Case B: --no-auth flag ==="
unset GEMINI_API_KEY
scion -g "$SCION_GROVE" start qa-no-auth "test no auth" --no-auth
if container list -a --format json | grep "qa-no-auth" -A 50 | grep -q "GEMINI_API_KEY=${TEST_KEY}"; then
    echo "FAILURE: GEMINI_API_KEY found when --no-auth was used."
    exit 1
else
    echo "SUCCESS: --no-auth respected."
fi

scion -g "$SCION_GROVE" stop qa-no-auth --rm