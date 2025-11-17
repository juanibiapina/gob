# Load bats testing libraries
load 'test_helper/bats-support/load'
load 'test_helper/bats-assert/load'

setup() {
  # Use BATS provided temporary directory
  cd "$BATS_TEST_TMPDIR"

  JOB_CLI="$BATS_TEST_DIRNAME/../dist/job"
}

teardown() {
  # Kill any background processes we started
  if [ -d ".local/share/job" ]; then
    for metadata_file in .local/share/job/*.json; do
      if [ -f "$metadata_file" ]; then
        pid=$(jq -r '.pid' "$metadata_file" 2>/dev/null)
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
          kill "$pid" 2>/dev/null || true
        fi
      fi
    done
  fi
}
