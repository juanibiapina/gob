# Load bats testing libraries
load 'test_helper/bats-support/load'
load 'test_helper/bats-assert/load'
load 'wait_helpers'

setup() {
  # Use BATS provided temporary directory
  cd "$BATS_TEST_TMPDIR"

  # Override XDG_DATA_HOME to use temporary directory for tests
  export XDG_DATA_HOME="$BATS_TEST_TMPDIR/.xdg-data"

  JOB_CLI="$BATS_TEST_DIRNAME/../dist/gob"
}

teardown() {
  # Use nuke command to clean up all jobs and metadata
  "$JOB_CLI" nuke >/dev/null 2>&1 || true
}
