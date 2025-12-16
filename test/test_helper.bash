# Load bats testing libraries
load 'test_helper/bats-support/load'
load 'test_helper/bats-assert/load'
load 'wait_helpers'

# Get job info as JSON. Usage: get_job [index]
# index defaults to 0 (first/newest job)
get_job() {
  local index=${1:-0}
  "$JOB_CLI" list --json | jq ".[$index]"
}

# Get specific field from job. Usage: get_job_field <field> [index]
get_job_field() {
  local field=$1
  local index=${2:-0}
  "$JOB_CLI" list --json | jq -r ".[$index].$field"
}

# Helper to get runtime directory for daemon files
get_runtime_dir() {
  echo "$XDG_RUNTIME_DIR/gob"
}

# Get a random available port
get_random_port() {
  python3 -c "import socket; s=socket.socket(); s.bind(('',0)); print(s.getsockname()[1]); s.close()"
}

# Helper to kill daemon if running.
# Used in teardown and in daemon-specific tests that need to test signal handling.
kill_daemon() {
  local runtime_dir=$(get_runtime_dir)
  if [ -f "$runtime_dir/daemon.pid" ]; then
    local pid=$(cat "$runtime_dir/daemon.pid")
    kill "$pid" 2>/dev/null || true
    # Wait for daemon to exit
    for i in {1..20}; do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.1
    done
  fi
  rm -f "$runtime_dir/daemon.pid" "$runtime_dir/daemon.sock"
}

setup() {
  # Use BATS provided temporary directory (unique per test)
  cd "$BATS_TEST_TMPDIR"

  # Override XDG runtime directory to use temporary directory for tests
  export XDG_RUNTIME_DIR="$BATS_TEST_TMPDIR/.xdg-runtime"

  # Override XDG state directory for persistent data (database, logs)
  export XDG_STATE_HOME="$BATS_TEST_TMPDIR/.xdg-state"

  # Disable telemetry during tests
  export GOB_TELEMETRY_DISABLED=1

  JOB_CLI="$BATS_TEST_DIRNAME/../dist/gob"
}

teardown() {
  # Cleanup: shutdown stops jobs and shuts down daemon, kill_daemon handles any orphaned daemon
  "$JOB_CLI" shutdown >/dev/null 2>&1 || true
  kill_daemon
}
