#!/usr/bin/env bats

load 'test_helper'

@test "remove command requires job ID argument" {
  run "$JOB_CLI" remove
  assert_failure
  assert_output --partial "accepts 1 arg(s)"
}

@test "remove command fails if job is running" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID
  local job_id=$(get_job_field id)

  # Try to remove running job - should fail
  run "$JOB_CLI" remove "$job_id"
  assert_failure
  assert_output --partial "cannot remove running job: $job_id (use 'stop' first)"

  # Verify job still exists in list
  run "$JOB_CLI" list --json
  assert_success
  local count=$(echo "$output" | jq 'length')
  assert_equal "$count" "1"
}

@test "remove command removes stopped job" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  local job_id=$(get_job_field id)
  local pid=$(get_job_field pid)

  # Stop the job
  "$JOB_CLI" stop "$job_id"
  wait_for_process_death "$pid"

  # Verify process is stopped
  run kill -0 "$pid"
  assert_failure

  # Remove the job
  run "$JOB_CLI" remove "$job_id"
  assert_success
  assert_output "Removed job $job_id (PID $pid)"

  # Verify job is gone from list
  run "$JOB_CLI" list --json
  assert_success
  assert_output "[]"
}

@test "remove command with invalid job ID shows error" {
  run "$JOB_CLI" remove 9999999999
  assert_failure
  assert_output --partial "job not found: 9999999999"
}

@test "remove command is not idempotent" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  local job_id=$(get_job_field id)
  local pid=$(get_job_field pid)

  # Stop the job
  "$JOB_CLI" stop "$job_id"
  wait_for_process_death "$pid"

  # Remove the job
  "$JOB_CLI" remove "$job_id"

  # Try to remove again - should fail (not idempotent)
  run "$JOB_CLI" remove "$job_id"
  assert_failure
  assert_output --partial "job not found: $job_id"
}

@test "remove command removes specific job among multiple jobs" {
  # Start first job
  "$JOB_CLI" add sleep 300

  # Start second job
  "$JOB_CLI" add sleep 400

  # Get first job (older one - index 1)
  local job_id1=$(get_job_field id 1)
  local pid1=$(get_job_field pid 1)

  # Get second job (newer one - index 0)
  local job_id2=$(get_job_field id 0)

  # Stop the first job
  "$JOB_CLI" stop "$job_id1"
  wait_for_process_death "$pid1"

  # Remove only the first job
  "$JOB_CLI" remove "$job_id1"

  # List should show only the second job
  run "$JOB_CLI" list
  assert_success
  assert_output --regexp "${job_id2}: \[[0-9]+\] running: sleep 400"
  refute_output --partial "$job_id1"

  # Verify only one job left
  local count=$("$JOB_CLI" list --json | jq 'length')
  assert_equal "$count" "1"
}

@test "remove command removes already stopped job" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  local job_id=$(get_job_field id)
  local pid=$(get_job_field pid)

  # Stop the process
  "$JOB_CLI" stop "$job_id"
  wait_for_process_death "$pid"

  # Verify process is stopped
  run kill -0 "$pid"
  assert_failure

  # Remove should work
  run "$JOB_CLI" remove "$job_id"
  assert_success
  assert_output "Removed job $job_id (PID $pid)"

  # Verify job is gone from list
  run "$JOB_CLI" list --json
  assert_success
  assert_output "[]"
}
