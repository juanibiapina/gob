#!/usr/bin/env bats

load 'test_helper'

@test "logs command with no jobs waits for jobs to appear" {
  run timeout 1 "$JOB_CLI" logs
  assert_failure
  assert_output --partial "waiting for jobs..."
}

@test "logs command picks up dynamically started jobs" {
  # Start logs in background (it will wait for jobs)
  "$JOB_CLI" logs > "$BATS_TEST_TMPDIR/logs_output.txt" 2>&1 &
  logs_pid=$!

  # Give it time to start waiting
  sleep 0.3

  # Start a job that writes output
  run "$JOB_CLI" start echo "Dynamic job output"
  assert_success

  # Extract job ID
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Wait for output to be written
  wait_for_log_content "$XDG_DATA_HOME/gob/${job_id}.stdout.log" "Dynamic job output"

  # Give logs command time to pick it up
  sleep 0.5

  # Kill the logs process
  kill $logs_pid 2>/dev/null || true
  wait $logs_pid 2>/dev/null || true

  # Check the captured output
  run cat "$BATS_TEST_TMPDIR/logs_output.txt"
  assert_output --partial "[$job_id] Dynamic job output"
}
