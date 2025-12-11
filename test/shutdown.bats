#!/usr/bin/env bats

load 'test_helper'

@test "shutdown command stops running jobs" {
  # Start a long-running job
  run "$JOB_CLI" add sleep 300
  assert_success

  # Get job ID and PID
  local job_id=$(get_job_field id)
  local pid=$(get_job_field pid)

  # Verify process is running
  run ps -p "$pid"
  assert_success

  # Run shutdown
  run "$JOB_CLI" shutdown
  assert_success
  assert_output --partial "Stopped 1 running job(s)"

  # Verify process is no longer running
  wait_for_process_death "$pid"
  run ps -p "$pid"
  assert_failure
}

@test "shutdown command preserves job history" {
  # Start a job
  run "$JOB_CLI" add echo "test output"
  assert_success

  # Get job ID
  local job_id=$(get_job_field id)

  # Wait for it to complete
  run "$JOB_CLI" await "$job_id"
  assert_success

  # Run shutdown
  run "$JOB_CLI" shutdown
  assert_success

  # Start daemon again and verify job still exists
  run "$JOB_CLI" list --json
  assert_success
  
  # Job should still be in the list
  local job_count=$(echo "$output" | jq 'length')
  assert_equal "$job_count" "1"
}

@test "shutdown command shuts down daemon" {
  # Start a job to ensure daemon is running
  run "$JOB_CLI" add sleep 300
  assert_success

  # Get daemon PID
  local daemon_pid=$(cat "$XDG_RUNTIME_DIR/gob/daemon.pid")
  assert [ -n "$daemon_pid" ]

  # Verify daemon is running
  run ps -p "$daemon_pid"
  assert_success

  # Run shutdown
  run "$JOB_CLI" shutdown
  assert_success
  assert_output --partial "Daemon shut down"

  # Verify daemon is no longer running
  sleep 0.5
  run ps -p "$daemon_pid"
  assert_failure
}

@test "shutdown command with no running jobs" {
  # Start a job that completes immediately
  run "$JOB_CLI" add echo "done"
  assert_success

  # Wait for it to complete
  local job_id=$(get_job_field id)
  run "$JOB_CLI" await "$job_id"
  assert_success

  # Run shutdown
  run "$JOB_CLI" shutdown
  assert_success
  assert_output --partial "Stopped 0 running job(s)"
  assert_output --partial "Daemon shut down"
}
