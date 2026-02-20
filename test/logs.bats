#!/usr/bin/env bats

load 'test_helper'

# --- Dump mode (default, no -f) ---

@test "logs command dumps stdout for a specific job" {
  run "$JOB_CLI" add echo "Hello from stdout"
  assert_success

  local job_id=$(get_job_field id)

  wait_for_log_content "$XDG_STATE_HOME/gob/logs/${job_id}-1.stdout.log" "Hello from stdout"

  run "$JOB_CLI" logs "$job_id"
  assert_success
  assert_output "Hello from stdout"
}

@test "logs command dumps stderr for a specific job" {
  run "$JOB_CLI" add -- sh -c 'echo "Hello from stderr" >&2'
  assert_success

  local job_id=$(get_job_field id)

  wait_for_log_content "$XDG_STATE_HOME/gob/logs/${job_id}-1.stderr.log" "Hello from stderr"

  # Capture streams separately (bats 'run' merges them)
  "$JOB_CLI" logs "$job_id" >"$BATS_TEST_TMPDIR/captured_stdout.txt" 2>"$BATS_TEST_TMPDIR/captured_stderr.txt"

  # stdout should be empty
  run cat "$BATS_TEST_TMPDIR/captured_stdout.txt"
  assert_output ""

  # stderr should have the message
  run cat "$BATS_TEST_TMPDIR/captured_stderr.txt"
  assert_output "Hello from stderr"
}

@test "logs command preserves stream separation" {
  run "$JOB_CLI" add -- sh -c 'echo "out message"; echo "err message" >&2'
  assert_success

  local job_id=$(get_job_field id)

  wait_for_log_content "$XDG_STATE_HOME/gob/logs/${job_id}-1.stdout.log" "out message"
  wait_for_log_content "$XDG_STATE_HOME/gob/logs/${job_id}-1.stderr.log" "err message"

  "$JOB_CLI" logs "$job_id" >"$BATS_TEST_TMPDIR/captured_stdout.txt" 2>"$BATS_TEST_TMPDIR/captured_stderr.txt"

  run cat "$BATS_TEST_TMPDIR/captured_stdout.txt"
  assert_output "out message"

  run cat "$BATS_TEST_TMPDIR/captured_stderr.txt"
  assert_output "err message"
}

@test "logs command fails for non-existent job" {
  run "$JOB_CLI" logs 999999999
  assert_failure
  assert_output --partial "job not found"
}

@test "logs command handles empty output" {
  run "$JOB_CLI" add sleep 1
  assert_success

  local job_id=$(get_job_field id)

  run "$JOB_CLI" logs "$job_id"
  assert_success
  assert_output ""
}

@test "logs command dumps all jobs in current directory" {
  run "$JOB_CLI" add echo "first job output"
  assert_success
  local job_id1=$(get_job_field id)

  run "$JOB_CLI" add echo "second job output"
  assert_success
  local job_id2=$(get_job_field id)

  wait_for_log_content "$XDG_STATE_HOME/gob/logs/${job_id1}-1.stdout.log" "first job output"
  wait_for_log_content "$XDG_STATE_HOME/gob/logs/${job_id2}-1.stdout.log" "second job output"

  run "$JOB_CLI" logs
  assert_success
  assert_output --partial "first job output"
  assert_output --partial "second job output"
}

# --- Follow mode (-f) ---

@test "logs -f with no jobs waits for jobs to appear" {
  run timeout 1 "$JOB_CLI" logs -f
  assert_failure
  assert_output --partial "waiting for jobs..."
}

@test "logs -f picks up dynamically started jobs" {
  "$JOB_CLI" logs -f > "$BATS_TEST_TMPDIR/logs_output.txt" 2>&1 &
  logs_pid=$!

  sleep 0.3

  run "$JOB_CLI" add echo "Dynamic job output"
  assert_success

  local job_id=$(get_job_field id)

  wait_for_log_content "$XDG_STATE_HOME/gob/logs/${job_id}-1.stdout.log" "Dynamic job output"

  sleep 0.5

  kill $logs_pid 2>/dev/null || true
  wait $logs_pid 2>/dev/null || true

  run cat "$BATS_TEST_TMPDIR/logs_output.txt"
  assert_output --partial "[$job_id] Dynamic job output"
}
