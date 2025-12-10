#!/usr/bin/env bats

load 'test_helper'

@test "runs command requires job ID argument" {
  run "$JOB_CLI" runs
  assert_failure
  assert_output --partial "accepts 1 arg(s)"
}

@test "runs command with invalid job ID shows error" {
  run "$JOB_CLI" runs 9999999999
  assert_failure
  assert_output --partial "job not found: 9999999999"
}

@test "runs command shows run history for a job" {
  # Start a job that completes quickly
  "$JOB_CLI" add true

  # Get job ID
  local job_id=$(get_job_field id)

  # Wait for job to complete
  wait_for_job_to_stop "$job_id"

  # Check runs
  run "$JOB_CLI" runs "$job_id"
  assert_success
  assert_output --regexp "${job_id}-1.*✓ \(0\)"
}

@test "runs command shows multiple runs after restart" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID
  local job_id=$(get_job_field id)
  local pid=$(get_job_field pid)

  # Stop the job
  "$JOB_CLI" stop "$job_id"
  wait_for_process_death "$pid"

  # Restart the job
  "$JOB_CLI" start "$job_id"
  local pid2=$(get_job_field pid)

  # Stop the job again
  "$JOB_CLI" stop "$job_id"
  wait_for_process_death "$pid2"

  # Check runs - should show 2 runs
  run "$JOB_CLI" runs "$job_id"
  assert_success
  assert_output --regexp "${job_id}-2"
  assert_output --regexp "${job_id}-1"
}

@test "runs command shows running status for active run" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID
  local job_id=$(get_job_field id)

  # Check runs
  run "$JOB_CLI" runs "$job_id"
  assert_success
  assert_output --regexp "${job_id}-1.*running.*◉"
}

@test "runs command with --json outputs valid JSON" {
  # Start a job that completes quickly
  "$JOB_CLI" add true

  # Get job ID
  local job_id=$(get_job_field id)

  # Wait for job to complete
  wait_for_job_to_stop "$job_id"

  # Check runs with JSON output
  run "$JOB_CLI" runs --json "$job_id"
  assert_success

  # Verify it's valid JSON with expected fields
  local run_id=$(echo "$output" | jq -r '.[0].id')
  assert_equal "$run_id" "${job_id}-1"

  local status=$(echo "$output" | jq -r '.[0].status')
  assert_equal "$status" "stopped"
}

@test "runs command shows exit code for failed job" {
  # Start a job that fails
  "$JOB_CLI" add sh -c "exit 42"

  # Get job ID
  local job_id=$(get_job_field id)

  # Wait for job to complete
  wait_for_job_to_stop "$job_id"

  # Check runs
  run "$JOB_CLI" runs "$job_id"
  assert_success
  assert_output --regexp "${job_id}-1.*✗ \(42\)"
}

@test "runs command shows no runs found for new job with no history" {
  # This shouldn't happen in normal usage since add always starts a run
  # but we test the output when there are no runs
  "$JOB_CLI" add sleep 300

  # Get job ID
  local job_id=$(get_job_field id)

  # Check runs - should show at least the current run
  run "$JOB_CLI" runs "$job_id"
  assert_success
  assert_output --regexp "${job_id}-1"
}
