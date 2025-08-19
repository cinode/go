#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Fetching latest Go version...${NC}"

# Get the latest stable Go version
# Primary: official endpoint returns single line like 'go1.25.1'
latest_go_version=$(curl -fsSL https://go.dev/VERSION?m=text 2>/dev/null | head -n1 | sed 's/^go//')

if [ -z "$latest_go_version" ]; then
    echo -e "${RED}Failed to detect latest Go version. Aborting.${NC}"
    exit 1
fi

latest_go_minor_version=$(echo "$latest_go_version" | cut -d. -f1-2)

if [ -z "$latest_go_minor_version" ]; then
    echo -e "${RED}Failed to detect latest Go version. Aborting.${NC}"
    exit 1
fi

echo "Latest Go version available: $latest_go_version (minor: $latest_go_minor_version)"

# Resolve repo root (script is in scripts/ under repo root)
REPO_ROOT=$(cd "$(dirname "$0")/.."; pwd)
cd "$REPO_ROOT"

# Get the required Go version from go.mod
required_go_version=$(awk '/^go [0-9]/{print $2; exit}' go.mod)
echo "Required Go version in go.mod: $required_go_version"

# Helper for version comparison (checks if $1 >= $2 using natural sort)
version_ge() {
    first=$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -n1)
    [ "$first" = "$2" ]
}

if [ "$required_go_version" != "$latest_go_minor_version" ]; then
    echo -e "${YELLOW}New Go version available: $latest_go_minor_version${NC}"
    echo -e "${YELLOW}Current project requires: $required_go_version${NC}"
    echo -e "${YELLOW}Updating project files to use Go $latest_go_minor_version...${NC}"

    files_updated=()

    # Update go.mod 'go X.Y' directive
    if sed -i -E "s/^go [0-9]+\.[0-9]+/go ${latest_go_minor_version}/" go.mod; then
        files_updated+=("go.mod")
    fi

    # Update GitHub Actions workflows go-version
    if [ -d .github/workflows ]; then
        while IFS= read -r -d '' wf; do
            # Replace go-version: "X.Y" or X.Y[.Z]
            sed -i -E "s/(go-version:\s*[\"']?)[0-9]+\.[0-9]+(\.[0-9]+)?([\"']?)/\1${latest_go_minor_version}\3/" "$wf"
            files_updated+=("$wf")
        done < <(find .github/workflows -type f \( -name "*.yml" -o -name "*.yaml" \) -print0 2>/dev/null)
    fi

    # Update Dockerfiles (ARG GO_VERSION= and any 'golang:X.Y[.Z]' literals)
    if [ -d build ]; then
        while IFS= read -r -d '' df; do
            sed -i -E "s/^(ARG GO_VERSION=)[0-9]+\.[0-9]+(\.[0-9]+)?/\1${latest_go_minor_version}/" "$df"
            sed -i -E "s/(golang:)[0-9]+\.[0-9]+(\.[0-9]+)?/\1${latest_go_minor_version}/" "$df"
            files_updated+=("$df")
        done < <(find build -type f -name "Dockerfile*" -print0 2>/dev/null)
    fi

    echo -e "${GREEN}Updated files:"${NC}
    for f in "${files_updated[@]}"; do
        echo " - $f"
    done
fi

echo -e "${GREEN}Go version check completed${NC}"

echo -e "\n${YELLOW}Updating Go dependencies...${NC}"

# Only run dependency updates if local Go toolchain is compatible
local_go_version=$(go version 2>/dev/null | awk '{print $3}' | sed 's/^go//' || true)
local_go_minor_version=$(echo "$local_go_version" | cut -d. -f1-2)

if [ -z "$local_go_version" ]; then
    echo -e "${RED}Go toolchain not found in PATH. Skipping dependency updates.${NC}"
    exit 1
fi

if ! version_ge "$local_go_minor_version" "$latest_go_minor_version"; then
    echo -e "${YELLOW}Local Go version ($local_go_version) is older than target ($latest_go_minor_version). Skipping dependency updates.${NC}"
    echo -e "${YELLOW}Please upgrade your local Go toolchain and re-run this script to update dependencies.${NC}"
    exit 1
fi

# Update all direct dependencies
echo "Running go get -u ./..."
go get -u ./...

# Update all indirect dependencies
echo "Running go get -u -t ./..."
go get -u -t ./...

# Tidy up the dependencies
echo "Running go mod tidy..."
go mod tidy

# Verify the dependencies
echo "Running go mod verify..."
go mod verify

echo -e "${GREEN}Dependencies updated successfully!${NC}"

