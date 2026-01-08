#!/bin/bash
# hack/cleanup.sh - Cleanup agents and specific template folder

REPO_ROOT=$(pwd)
TEST_DIR="${REPO_ROOT}/../qa-scion"

echo "=== Cleaning up agents ==="

# Stop all agents started by scion
# Use the scion on path
if command -v scion &> /dev/null; then
    # We need to be in a grove context or use -g
    AGENTS=$(scion -g "${TEST_DIR}/.scion" list | tail -n +2 | awk '{print $1}')
    for agent in $AGENTS; do
        if [ -n "$agent" ]; then
            scion -g "${TEST_DIR}/.scion" rm "$agent"
        fi
    done
fi

echo "=== Cleaning up specific scion directories ==="
if [ -d "${TEST_DIR}/.scion" ]; then
    # Only remove agents, default templates, and settings
    rm -rf "${TEST_DIR}/.scion/agents"
    rm -rf "${TEST_DIR}/.scion/templates/claude"
    rm -rf "${TEST_DIR}/.scion/templates/gemini"
    rm -f "${TEST_DIR}/.scion/settings.json"
    echo "Removed .scion/agents, templates, and settings.json"
fi

echo "=== Cleanup Complete ==="



"$(dirname "$0")"/setup.sh