#!/usr/bin/env bats

load 'test_helper'

@test "restart command requires job ID argument" {
  run "$JOB_CLI" restart
  assert_failure
  assert_output --partial "accepts 1 arg(s)"
}

@test "restart command restarts a running job" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID and original PID
  local job_id=$(get_job_field id)
  local original_pid=$(get_job_field pid)

  # Verify original process is running
  assert kill -0 "$original_pid"

  # Restart the job
  run "$JOB_CLI" restart "$job_id"
  assert_success
  assert_output --regexp "Restarted job $job_id with new PID [0-9]+ running: sleep 300"

  # Get new PID
  local new_pid=$(get_job_field pid)

  # Verify new process is running
  assert kill -0 "$new_pid"

  # Verify PIDs are different
  assert [ "$new_pid" != "$original_pid" ]

  # Verify original process was stopped
  run kill -0 "$original_pid"
  assert_failure
}

@test "restart command starts a stopped job" {
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

  # Restart the job
  run "$JOB_CLI" restart "$job_id"
  assert_success
  assert_output --regexp "Restarted job $job_id with new PID [0-9]+ running: sleep 300"

  # Get new PID
  local new_pid=$(get_job_field pid)

  # Verify new process is running
  assert kill -0 "$new_pid"
}

@test "restart command with invalid job ID shows error" {
  run "$JOB_CLI" restart 9999999999
  assert_failure
  assert_output --partial "job not found: 9999999999"
}

@test "restart command preserves job ID" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID
  local job_id=$(get_job_field id)

  # Restart the job
  "$JOB_CLI" restart "$job_id"

  # Verify job ID is still the same
  local new_job_id=$(get_job_field id)
  assert_equal "$job_id" "$new_job_id"
}

@test "restart command updates PID in metadata" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID and original PID
  local job_id=$(get_job_field id)
  local original_pid=$(get_job_field pid)

  # Restart the job
  "$JOB_CLI" restart "$job_id"

  # Get new PID
  local new_pid=$(get_job_field pid)

  # Verify PID was updated
  assert [ "$new_pid" != "$original_pid" ]

  # Verify new PID is valid
  assert [ "$new_pid" -gt 0 ]
  assert kill -0 "$new_pid"
}

@test "restart command preserves command in metadata" {
  # Add a job with multiple arguments
  "$JOB_CLI" add sleep 300

  # Get job ID
  local job_id=$(get_job_field id)

  # Restart the job
  "$JOB_CLI" restart "$job_id"

  # Verify command is preserved
  local job=$("$JOB_CLI" list --json | jq '.[0]')
  local command_length=$(echo "$job" | jq '.command | length')
  assert [ "$command_length" -eq 2 ]
  assert [ "$(echo "$job" | jq -r '.command[0]')" = "sleep" ]
  assert [ "$(echo "$job" | jq -r '.command[1]')" = "300" ]
}

@test "restarted job shows as running in list" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  local job_id=$(get_job_field id)
  local pid=$(get_job_field pid)

  # Stop the job
  "$JOB_CLI" stop "$job_id"
  wait_for_process_death "$pid"

  # Restart the job
  "$JOB_CLI" restart "$job_id"

  # Verify it shows as running
  run "$JOB_CLI" list
  assert_success
  assert_output --regexp "${job_id}: \[[0-9]+\] running: sleep 300"
}

@test "restart command works multiple times" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID
  local job_id=$(get_job_field id)
  local first_pid=$(get_job_field pid)

  # First restart
  "$JOB_CLI" restart "$job_id"
  local second_pid=$(get_job_field pid)

  # Verify PID changed
  assert [ "$second_pid" != "$first_pid" ]
  assert kill -0 "$second_pid"

  # Second restart
  "$JOB_CLI" restart "$job_id"
  local third_pid=$(get_job_field pid)

  # Verify PID changed again
  assert [ "$third_pid" != "$second_pid" ]
  assert kill -0 "$third_pid"

  # Verify second PID was stopped
  run kill -0 "$second_pid"
  assert_failure
}
