#!/bin/sh
#
# Install the pre-commit hook for terraform-mirror
#
# Usage:
#   ./scripts/install-hooks.sh
#
# Or on Windows (PowerShell):
#   .\scripts\install-hooks.ps1
#

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
HOOKS_DIR="$REPO_ROOT/.git/hooks"

echo "Installing git hooks for terraform-mirror..."

# Ensure .git/hooks directory exists
if [ ! -d "$HOOKS_DIR" ]; then
    echo "Error: .git/hooks directory not found. Are you in a git repository?"
    exit 1
fi

# Copy pre-commit hook
cp "$SCRIPT_DIR/pre-commit" "$HOOKS_DIR/pre-commit"
chmod +x "$HOOKS_DIR/pre-commit"

echo ""
echo "✓ Pre-commit hook installed successfully!"
echo ""
echo "The hook will run automatically before each commit."
echo "To skip the hook (not recommended), use: git commit --no-verify"
echo ""
echo "Prerequisites:"
echo "  - Go (required)"
echo "  - golangci-lint (recommended): go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
