#!/usr/bin/env bats

load 'test_helper'

@test "start command requires job ID argument" {
  run "$JOB_CLI" start
  assert_failure
  assert_output --partial "accepts 1 arg(s)"
}

@test "start command starts a stopped job" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  local job_id=$(get_job_field id)
  local original_pid=$(get_job_field pid)

  # Stop the job
  "$JOB_CLI" stop "$job_id"
  wait_for_process_death "$original_pid"

  # Verify process is stopped
  run kill -0 "$original_pid"
  assert_failure

  # Start the stopped job
  run "$JOB_CLI" start "$job_id"
  assert_success
  assert_output --regexp "Started job $job_id with PID [0-9]+ running: sleep 300"

  # Get new PID
  local new_pid=$(get_job_field pid)

  # Verify new process is running
  assert kill -0 "$new_pid"
}

@test "start command fails if job is already running" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID
  local job_id=$(get_job_field id)

  # Try to start the already running job
  run "$JOB_CLI" start "$job_id"
  assert_failure
  assert_output --partial "already running"
  assert_output --partial "use 'gob restart'"
}

@test "start command with invalid job ID shows error" {
  run "$JOB_CLI" start 9999999999
  assert_failure
  assert_output --partial "job not found: 9999999999"
}

@test "start command updates PID" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID and original PID
  local job_id=$(get_job_field id)
  local original_pid=$(get_job_field pid)

  # Stop the job
  "$JOB_CLI" stop "$job_id"
  wait_for_process_death "$original_pid"

  # Start the job
  "$JOB_CLI" start "$job_id"

  # Get new PID
  local new_pid=$(get_job_field pid)

  # Verify PID was updated
  assert [ "$new_pid" != "$original_pid" ]

  # Verify new PID is valid
  assert [ "$new_pid" -gt 0 ]
  assert kill -0 "$new_pid"
}

@test "started job shows as running in list" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  local job_id=$(get_job_field id)
  local pid=$(get_job_field pid)

  # Stop the job
  "$JOB_CLI" stop "$job_id"
  wait_for_process_death "$pid"

  # Start the job
  "$JOB_CLI" start "$job_id"

  # Verify it shows as running
  run "$JOB_CLI" list
  assert_success
  assert_output --regexp "${job_id}: \[[0-9]+\] running: sleep 300"
}

@test "start command clears previous log files" {
  # Start a job that writes unique timestamp output
  run "$JOB_CLI" add -- sh -c 'echo "run-$(date +%s%N)"'
  assert_success

  # Extract job ID
  local job_id=$(get_job_field id)

  # Wait for output to be written and process to stop (logs in XDG_RUNTIME_DIR with daemon)
  wait_for_log_content "$XDG_RUNTIME_DIR/gob/${job_id}-1.stdout.log" "run-"
  local pid=$(get_job_field pid)
  wait_for_process_death "$pid" || sleep 0.2

  # Record the first output
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  local first_output="$output"

  # Small delay to ensure different timestamp
  sleep 0.01

  # Start the job (logs should be cleared)
  run "$JOB_CLI" start "$job_id"
  assert_success

  # Get new PID and wait for process to finish
  local new_pid=$(get_job_field pid)
  wait_for_process_death "$new_pid" || sleep 0.2

  # Check that log file only contains new output (first output is gone)
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  refute_output --partial "$first_output"
  assert_output --partial "run-"
}
