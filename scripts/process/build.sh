#!/bin/bash
set -euo pipefail

# ============================================================
# build.sh — Full Build & Unit Test Runner
#
# Builds the entire project and runs unit tests.
# Integration tests (under tests/) are excluded;
# use integration_test.sh for those.
#
# Usage:
#   ./scripts/process/build.sh [OPTIONS]
#
# Options:
#   --backend-only   Run only the Go backend build & tests
#   --help           Show this help message
#
# Exit Codes:
#   0 = All builds and tests passed
#   1 = Build or test failure
# ============================================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color
BOLD='\033[1m'

# --- Helpers ---
info()    { echo -e "${BLUE}[INFO]${NC} $*"; }
success() { echo -e "${GREEN}[PASS]${NC} $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
fail()    { echo -e "${RED}[FAIL]${NC} $*"; }
step()    { echo -e "${CYAN}${BOLD}===> $*${NC}"; }

show_help() {
    cat << 'EOF'
Usage: ./scripts/process/build.sh [OPTIONS]

Builds the entire project and runs unit tests.
Integration tests (under tests/) are excluded.

Options:
  --help           Show this help message

Exit Codes:
  0 = All builds and tests passed
  1 = Build or test failure

Examples:
  # Full build
  ./scripts/process/build.sh

EOF
}

# --- Argument Parsing ---

while [[ $# -gt 0 ]]; do
    case "$1" in
        --help|-h)
            show_help
            exit 0
            ;;
        *)
            fail "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# --- Track overall result ---
FAILED=false

# ============================================================
# Go Build & Unit Test
# ============================================================
build_go() {
    step "Go: Build & Unit Test"

    cd "$PROJECT_ROOT"

    # Ensure bin/ directory exists
    mkdir -p "$PROJECT_ROOT/bin"

    build_go_module() {
        local module_dir="$1"
        local module_name="$2"
        local bin_group="$3"

        step "Feature: $module_name"
        cd "$module_dir"

        info "Running Go unit tests for $module_name (excluding tests/ directory)..."
        UNIT_PKGS=$(go list ./... | grep -v '/tests/' | grep -v '/tests$' || true)

        if [[ -z "$UNIT_PKGS" ]]; then
            warn "No Go unit test packages found for $module_name."
        elif echo "$UNIT_PKGS" | xargs go test -v -count=1; then
            success "Unit tests passed for $module_name."
        else
            fail "Unit tests failed for $module_name."
            FAILED=true
            return 1
        fi

        info "Building $module_name..."
        PKG_COUNT=$(go list ./... | wc -l | tr -d ' ')
        if [[ "$PKG_COUNT" -le 1 ]]; then
            if go build -o "$PROJECT_ROOT/bin/$bin_group" ./...; then
                success "Build succeeded for $module_name → bin/$bin_group"
            else
                fail "Build failed for $module_name."
                FAILED=true
                return 1
            fi
        else
            if go build ./...; then
                success "Package build succeeded for $module_name."
            else
                fail "Build failed for $module_name."
                FAILED=true
                return 1
            fi

            MAIN_PKGS=$(go list -f '{{if eq .Name "main"}}{{.ImportPath}}{{end}}' ./... | awk 'NF')
            if [[ -n "$MAIN_PKGS" ]]; then
                mkdir -p "$PROJECT_ROOT/bin/$bin_group"
                while IFS= read -r main_pkg; do
                    [[ -n "$main_pkg" ]] || continue
                    bin_name="$(basename "$main_pkg")"
                    if ! go build -o "$PROJECT_ROOT/bin/$bin_group/$bin_name" "$main_pkg"; then
                        fail "Binary build failed for $main_pkg."
                        FAILED=true
                        return 1
                    fi
                done <<< "$MAIN_PKGS"
                success "Main binaries built for $module_name → bin/$bin_group/"
            fi
        fi
        cd "$PROJECT_ROOT"
    }

    # Enumerate features/{name}/ directories containing go.mod
    local found_any=false
    local root_module=""

    if [[ -f "$PROJECT_ROOT/go.mod" ]]; then
        root_module=$(awk '/^module /{print $2; exit}' "$PROJECT_ROOT/go.mod")
        found_any=true
        build_go_module "$PROJECT_ROOT" "entext-root" "entext" || return 1
    fi

    for feature_dir in features/*/; do
        # Skip if glob didn't match (no features/ directories)
        [[ -d "$feature_dir" ]] || continue

        # Only process directories that contain go.mod (Go projects)
        if [[ ! -f "$feature_dir/go.mod" ]]; then
            info "Skipping $feature_dir — no go.mod found."
            continue
        fi

        found_any=true
        local feature_name
        feature_name=$(basename "$feature_dir")
        feature_module=$(awk '/^module /{print $2; exit}' "$feature_dir/go.mod")

        if [[ -n "$root_module" && "$feature_module" == "$root_module" ]]; then
            info "Skipping $feature_name — duplicate module path with root ($root_module)."
            continue
        fi

        build_go_module "$PROJECT_ROOT/$feature_dir" "$feature_name" "$feature_name" || return 1
    done

    if [[ "$found_any" == "false" ]]; then
        warn "No Go projects found (root go.mod or features/*/go.mod)."
        return 0
    fi
}

# ============================================================
# Main
# ============================================================
main() {
    echo ""
    echo -e "${BOLD}╔══════════════════════════════════════════╗${NC}"
    echo -e "${BOLD}║     Build & Unit Test Pipeline           ║${NC}"
    echo -e "${BOLD}╚══════════════════════════════════════════╝${NC}"
    echo ""

    local start_time=$SECONDS

    build_go

    local elapsed=$(( SECONDS - start_time ))
    echo ""
    echo -e "${BOLD}─────────────────────────────────────────────${NC}"

    if [[ "$FAILED" == "true" ]]; then
        fail "Build pipeline FAILED (${elapsed}s)"
        echo -e "${RED}Fix the errors above before running integration tests.${NC}"
        exit 1
    else
        success "Build pipeline PASSED (${elapsed}s)"
        echo -e "${GREEN}Ready for integration tests: ./scripts/process/integration_test.sh${NC}"
        exit 0
    fi
}

main
