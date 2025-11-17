# Load bats testing libraries
load 'test_helper/bats-support/load'
load 'test_helper/bats-assert/load'

setup() {
  # Use BATS provided temporary directory
  cd "$BATS_TEST_TMPDIR"

  JOB_CLI="$BATS_TEST_DIRNAME/../dist/gob"
}

teardown() {
  # Use nuke command to clean up all jobs and metadata
  "$JOB_CLI" nuke >/dev/null 2>&1 || true
}
