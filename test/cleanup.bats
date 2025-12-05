#!/usr/bin/env bats

load 'test_helper'

@test "cleanup command with no jobs" {
  run "$JOB_CLI" cleanup
  assert_success
  assert_output "Cleaned up 0 stopped job(s)"
}

@test "cleanup command removes only stopped jobs" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  local job_id=$(get_job_field id)
  local pid=$(get_job_field pid)

  # Stop the job manually
  "$JOB_CLI" stop "$job_id"
  wait_for_process_death "$pid"

  # Verify process is stopped
  run kill -0 "$pid"
  assert_failure

  # Verify job exists in list
  local count=$("$JOB_CLI" list --json | jq 'length')
  assert_equal "$count" "1"

  # Run cleanup
  run "$JOB_CLI" cleanup
  assert_success
  assert_output "Cleaned up 1 stopped job(s)"

  # Verify job was removed from list
  run "$JOB_CLI" list --json
  assert_success
  assert_output "[]"
}

@test "cleanup command preserves running jobs" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  local job_id=$(get_job_field id)
  local pid=$(get_job_field pid)

  # Verify process is running
  assert kill -0 "$pid"

  # Run cleanup
  run "$JOB_CLI" cleanup
  assert_success
  assert_output "Cleaned up 0 stopped job(s)"

  # Verify job still exists in list
  local count=$("$JOB_CLI" list --json | jq 'length')
  assert_equal "$count" "1"

  # Verify process is still running
  assert kill -0 "$pid"
}

@test "cleanup command with mixed running and stopped jobs" {
  # Start three jobs
  "$JOB_CLI" add sleep 300
  "$JOB_CLI" add sleep 400
  "$JOB_CLI" add sleep 500

  # Get job IDs and PIDs (newest first: 500, 400, 300)
  local job_id1=$(get_job_field id 2)  # sleep 300 (oldest)
  local pid1=$(get_job_field pid 2)
  local job_id2=$(get_job_field id 1)  # sleep 400 (middle)
  local pid2=$(get_job_field pid 1)
  local job_id3=$(get_job_field id 0)  # sleep 500 (newest)
  local pid3=$(get_job_field pid 0)

  # Stop first and third jobs
  "$JOB_CLI" stop "$job_id1"
  "$JOB_CLI" stop "$job_id3"
  wait_for_process_death "$pid1"
  wait_for_process_death "$pid3"

  # Verify first and third are stopped, second is running
  run kill -0 "$pid1"
  assert_failure
  assert kill -0 "$pid2"
  run kill -0 "$pid3"
  assert_failure

  # Run cleanup
  run "$JOB_CLI" cleanup
  assert_success
  assert_output "Cleaned up 2 stopped job(s)"

  # Verify only running job remains
  local count=$("$JOB_CLI" list --json | jq 'length')
  assert_equal "$count" "1"

  local remaining_id=$(get_job_field id)
  assert_equal "$remaining_id" "$job_id2"
  assert kill -0 "$pid2"
}

@test "cleanup command cleans up multiple stopped jobs" {
  # Start three jobs
  "$JOB_CLI" add sleep 300
  "$JOB_CLI" add sleep 400
  "$JOB_CLI" add sleep 500

  # Stop all jobs
  local jobs=$("$JOB_CLI" list --json)
  for i in 0 1 2; do
    local job_id=$(echo "$jobs" | jq -r ".[$i].id")
    local pid=$(echo "$jobs" | jq -r ".[$i].pid")
    "$JOB_CLI" stop "$job_id"
    wait_for_process_death "$pid"
  done

  # Run cleanup
  run "$JOB_CLI" cleanup
  assert_success
  assert_output "Cleaned up 3 stopped job(s)"

  # Verify all jobs were removed
  run "$JOB_CLI" list --json
  assert_success
  assert_output "[]"
}

@test "cleanup command is safe to run multiple times" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  local job_id=$(get_job_field id)
  local pid=$(get_job_field pid)

  # Stop the job
  "$JOB_CLI" stop "$job_id"
  wait_for_process_death "$pid"

  # Run cleanup first time
  run "$JOB_CLI" cleanup
  assert_success
  assert_output "Cleaned up 1 stopped job(s)"

  # Run cleanup again
  run "$JOB_CLI" cleanup
  assert_success
  assert_output "Cleaned up 0 stopped job(s)"
}

@test "cleanup after using stop command" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  local job_id=$(get_job_field id)
  local pid=$(get_job_field pid)

  # Stop using stop command
  "$JOB_CLI" stop "$job_id"
  wait_for_process_death "$pid"

  # Verify job still exists in list (stop doesn't remove it)
  local count=$("$JOB_CLI" list --json | jq 'length')
  assert_equal "$count" "1"

  # Run cleanup
  run "$JOB_CLI" cleanup
  assert_success
  assert_output "Cleaned up 1 stopped job(s)"

  # Verify job was removed from list
  run "$JOB_CLI" list --json
  assert_success
  assert_output "[]"
}
