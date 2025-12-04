# Load bats testing libraries
load 'test_helper/bats-support/load'
load 'test_helper/bats-assert/load'
load 'wait_helpers'

# Helper to get runtime directory for daemon files
get_runtime_dir() {
  echo "$XDG_RUNTIME_DIR/gob"
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

  # Override XDG directories to use temporary directory for tests
  export XDG_DATA_HOME="$BATS_TEST_TMPDIR/.xdg-data"
  export XDG_RUNTIME_DIR="$BATS_TEST_TMPDIR/.xdg-runtime"

  JOB_CLI="$BATS_TEST_DIRNAME/../dist/gob"
}

teardown() {
  # Cleanup strategy (Phase 1):
  # 1. nuke: stops jobs and cleans up metadata/logs (currently file-based, doesn't need daemon)
  # 2. kill_daemon: stops the daemon process
  #
  # In Phase 2+, nuke will be daemon-based and will handle daemon shutdown itself,
  # so kill_daemon can be removed from here.
  "$JOB_CLI" nuke >/dev/null 2>&1 || true
  kill_daemon
}
