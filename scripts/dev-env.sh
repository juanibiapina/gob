# Development environment for testing gob locally
# Usage: source scripts/dev-env.sh <session-name>
#
# Example:
#   source scripts/dev-env.sh test1
#   gob list          # uses local build with isolated daemon
#   gob               # TUI with isolated daemon
#
# In another terminal with the same session:
#   source scripts/dev-env.sh test1
#   gob list          # connects to same daemon

if [ -z "$1" ]; then
    echo "Usage: source scripts/dev-env.sh <session-name>"
    echo "Example: source scripts/dev-env.sh test1"
    return 1 2>/dev/null || exit 1
fi

_GOB_SESSION="$1"
_GOB_DEV_DIR="/tmp/gob-dev-${_GOB_SESSION}"

# Create directories
mkdir -p "${_GOB_DEV_DIR}/runtime"
mkdir -p "${_GOB_DEV_DIR}/state"

# Set XDG paths
export XDG_RUNTIME_DIR="${_GOB_DEV_DIR}/runtime"
export XDG_STATE_HOME="${_GOB_DEV_DIR}/state"

# Build the local version
echo "Building gob..."
(cd "$(dirname "${BASH_SOURCE[0]:-$0}")/.." && go build -o ./gob-dev .) || {
    echo "Build failed!"
    return 1 2>/dev/null || exit 1
}

# Create alias
_GOB_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")/.." && pwd)"
alias gob="${_GOB_ROOT}/gob-dev"

echo ""
echo "Development environment ready!"
echo "  Session:  ${_GOB_SESSION}"
echo "  Runtime:  ${XDG_RUNTIME_DIR}"
echo "  State:    ${XDG_STATE_HOME}"
echo ""
echo "Commands:"
echo "  gob list    # test CLI"
echo "  gob         # test TUI"
echo "  gob-dev-cleanup   # shutdown daemon and clean up"
echo ""

# Cleanup function
gob-dev-cleanup() {
    echo "Shutting down dev daemon..."
    "${_GOB_ROOT}/gob-dev" shutdown 2>/dev/null || true
    echo "Removing ${_GOB_DEV_DIR}..."
    rm -rf "${_GOB_DEV_DIR}"
    unalias gob 2>/dev/null || true
    unset -f gob-dev-cleanup
    unset XDG_RUNTIME_DIR XDG_STATE_HOME _GOB_SESSION _GOB_DEV_DIR _GOB_ROOT
    echo "Cleaned up!"
}
