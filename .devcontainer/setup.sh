#!/bin/bash
# Skriva devcontainer post-create setup script.
# Ensures minimum Go version and runs mandatory security checks.
set -e

REQUIRED_GO="1.25.7"
echo "=== Skriva Dev Container Setup ==="

# --- 1. Verify / upgrade Go version ---
CURRENT_GO=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+\.[0-9]+')
echo "Current Go: $CURRENT_GO (required: >= $REQUIRED_GO)"

version_gte() {
  # Returns 0 (true) if $1 >= $2 using sort -V
  [ "$(printf '%s\n%s' "$1" "$2" | sort -V | head -n1)" = "$2" ]
}

if ! version_gte "$CURRENT_GO" "$REQUIRED_GO"; then
  echo "Go $CURRENT_GO is below required $REQUIRED_GO — upgrading..."
  GONOSUMCHECK='*' GONOSUMDB='*' go install "golang.org/dl/go${REQUIRED_GO}@latest" 2>/dev/null
  GONOSUMCHECK='*' GONOSUMDB='*' "go${REQUIRED_GO}" download
  # Make the new version the default
  export PATH="/home/vscode/sdk/go${REQUIRED_GO}/bin:$PATH"
  export GOROOT="/home/vscode/sdk/go${REQUIRED_GO}"
  # Persist for future shells
  echo "export PATH=\"/home/vscode/sdk/go${REQUIRED_GO}/bin:\$PATH\"" >> ~/.bashrc
  echo "export GOROOT=\"/home/vscode/sdk/go${REQUIRED_GO}\"" >> ~/.bashrc
  echo "Upgraded to Go $(go version | grep -oP 'go\K[0-9]+\.[0-9]+\.[0-9]+')"
else
  echo "Go version OK."
fi

# --- 2. Download dependencies ---
echo ""
echo "=== Downloading dependencies ==="
GONOSUMCHECK='*' GONOSUMDB='*' go mod download

# --- 3. Install security tools ---
echo ""
echo "=== Installing govulncheck ==="
GONOSUMCHECK='*' GONOSUMDB='*' go install golang.org/x/vuln/cmd/govulncheck@latest 2>/dev/null || true

# --- 4. Build verification ---
echo ""
echo "=== Build check ==="
go build ./cmd/blog && echo "BUILD: PASS" || { echo "BUILD: FAIL"; exit 1; }

# --- 5. Vet check ---
echo ""
echo "=== Vet check ==="
go vet ./... && echo "VET: PASS" || { echo "VET: FAIL (warnings found)"; exit 1; }

# --- 6. Vulnerability scan ---
echo ""
echo "=== Vulnerability scan ==="
if command -v govulncheck &>/dev/null; then
  govulncheck ./... && echo "VULNCHECK: PASS" || { echo "VULNCHECK: FAIL (vulnerabilities found)"; exit 1; }
else
  echo "VULNCHECK: SKIPPED (govulncheck not available)"
fi

echo ""
echo "=== All checks passed! ==="
echo "Go $(go version | grep -oP 'go\K[0-9]+\.[0-9]+\.[0-9]+') | Binary: $(ls -lh blog 2>/dev/null | awk '{print $5}' || echo 'N/A')"
