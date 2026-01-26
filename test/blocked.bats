#!/usr/bin/env bats

load 'test_helper'

# Helper to create a gobfile with blocked job
create_gobfile() {
  mkdir -p .config
  cat > .config/gobfile.toml <<EOF
[[job]]
command = "sleep 300"
description = "This job is blocked for testing"
blocked = true

[[job]]
command = "echo hello"
description = "This job is not blocked"
blocked = false
EOF
}

@test "blocked job cannot be started via gob add" {
  # Create gobfile with blocked job
  create_gobfile

  # Try to add the blocked command - should fail (CLI checks gobfile directly)
  run "$JOB_CLI" add sleep 300
  assert_failure
  assert_output --partial "job is blocked"
  assert_output --partial "This job is blocked for testing"
}

@test "blocked job cannot be started via gob run" {
  # Create gobfile with blocked job
  create_gobfile

  # Try to run the blocked command - should fail (CLI checks gobfile directly)
  run "$JOB_CLI" run sleep 300
  assert_failure
  assert_output --partial "job is blocked"
  assert_output --partial "This job is blocked for testing"
}

@test "non-blocked job can be started normally" {
  # Create gobfile with blocked and non-blocked jobs
  create_gobfile

  # The non-blocked job should be able to start
  run "$JOB_CLI" add echo hello
  assert_success
  assert_output --partial "Added job"
}

@test "blocked job shows description in error message" {
  # Create gobfile with blocked job
  create_gobfile

  # Try to add the blocked command
  run "$JOB_CLI" add sleep 300
  assert_failure

  # Error message should include the description
  assert_output --partial "This job is blocked for testing"
}

@test "blocked job without description shows simple message" {
  # Create gobfile with blocked job without description
  mkdir -p .config
  cat > .config/gobfile.toml <<EOF
[[job]]
command = "sleep 300"
blocked = true
EOF

  # Try to add the blocked command
  run "$JOB_CLI" add sleep 300
  assert_failure
  assert_output --partial "job is blocked"
}

@test "unblocking a job allows it to be started" {
  # Create gobfile with blocked job
  create_gobfile

  # Verify it's blocked
  run "$JOB_CLI" add sleep 300
  assert_failure
  assert_output --partial "job is blocked"

  # Update gobfile to unblock the job
  cat > .config/gobfile.toml <<EOF
[[job]]
command = "sleep 300"
description = "This job is now unblocked"
blocked = false
EOF

  # CLI checks gobfile directly, so unblocking should work immediately
  run "$JOB_CLI" add sleep 300
  assert_success
  assert_output --partial "Added job"
}

@test "command not in gobfile can be started" {
  # Create gobfile with a blocked job
  create_gobfile

  # A command not in gobfile should work fine
  run "$JOB_CLI" add sleep 999
  assert_success
  assert_output --partial "Added job"
}

@test "blocked check matches exact command" {
  # Create gobfile with blocked job for "sleep 300"
  create_gobfile

  # Different arguments should not be blocked
  run "$JOB_CLI" add sleep 301
  assert_success
  assert_output --partial "Added job"
}

@test "blocked job with autostart true does not autostart" {
  # Create gobfile with blocked + autostart job
  mkdir -p .config
  cat > .config/gobfile.toml <<EOF
[[job]]
command = "sleep 300"
description = "Blocked autostart job"
autostart = true
blocked = true
EOF

  # Start TUI briefly to trigger gobfile loading (use list as proxy)
  # The job should be created but NOT started
  run "$JOB_CLI" list
  assert_success

  # Verify job is blocked via CLI
  run "$JOB_CLI" add sleep 300
  assert_failure
  assert_output --partial "job is blocked"

  # No sleep process should be running
  run pgrep -f "sleep 300"
  assert_failure
}
