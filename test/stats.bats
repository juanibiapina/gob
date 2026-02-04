#!/usr/bin/env bats

load 'test_helper'

@test "stats command requires job ID argument" {
  run "$JOB_CLI" stats
  assert_failure
  assert_output --partial "accepts 1 arg(s)"
}

@test "stats command with invalid job ID shows error" {
  run "$JOB_CLI" stats 9999999999
  assert_failure
  assert_output --partial "job not found: 9999999999"
}

@test "stats command shows statistics for a job with completed runs" {
  # Start a job that completes quickly
  "$JOB_CLI" add true

  # Get job ID
  local job_id=$(get_job_field id)

  # Wait for job to complete
  wait_for_job_to_stop "$job_id"

  # Check stats
  run "$JOB_CLI" stats "$job_id"
  assert_success
  assert_output --partial "Job: $job_id (true)"
  assert_output --partial "Total runs: 1"
  assert_output --partial "Success rate: 100%"
}

@test "stats command shows no completed runs for running job" {
  # Start a job that runs for a while
  "$JOB_CLI" add sleep 300

  # Get job ID
  local job_id=$(get_job_field id)

  # Check stats - no completed runs yet
  run "$JOB_CLI" stats "$job_id"
  assert_success
  assert_output --partial "Job: $job_id (sleep 300)"
  assert_output --partial "No completed runs yet"
}

@test "stats command calculates success rate correctly" {
  # Start a job that succeeds
  "$JOB_CLI" add true

  # Get job ID
  local job_id=$(get_job_field id)

  # Wait for job to complete
  wait_for_job_to_stop "$job_id"

  # Restart to run again
  "$JOB_CLI" start "$job_id"
  wait_for_job_to_stop "$job_id"

  # Start a failing run by changing the command won't work, so we test with what we have
  # Two successful runs = 100% success rate

  # Check stats
  run "$JOB_CLI" stats "$job_id"
  assert_success
  assert_output --partial "Total runs: 2"
  assert_output --partial "Success rate: 100% (2/2)"
}

@test "stats command with --json outputs valid JSON" {
  # Start a job that completes quickly
  "$JOB_CLI" add true

  # Get job ID
  local job_id=$(get_job_field id)

  # Wait for job to complete
  wait_for_job_to_stop "$job_id"

  # Check stats with JSON output
  run "$JOB_CLI" stats --json "$job_id"
  assert_success

  # Verify it's valid JSON with expected fields
  local stats_job_id=$(echo "$output" | jq -r '.id')
  assert_equal "$stats_job_id" "$job_id"

  local run_count=$(echo "$output" | jq -r '.run_count')
  assert_equal "$run_count" "1"

  local success_rate=$(echo "$output" | jq -r '.success_rate')
  assert_equal "$success_rate" "100"
}

@test "stats command shows duration statistics for successful run" {
  # Start a job that completes successfully
  "$JOB_CLI" add true

  # Get job ID
  local job_id=$(get_job_field id)

  # Wait for job to complete
  wait_for_job_to_stop "$job_id"

  # Check stats
  run "$JOB_CLI" stats "$job_id"
  assert_success
  assert_output --partial "Avg success duration:"
  assert_output --partial "Fastest:"
  assert_output --partial "Slowest:"
  # Should not show failure duration since there are no failures
  refute_output --partial "Avg failure duration:"
}

@test "stats command shows failed runs in success rate" {
  # Start a job that fails
  "$JOB_CLI" add sh -c "exit 1"

  # Get job ID
  local job_id=$(get_job_field id)

  # Wait for job to complete
  wait_for_job_to_stop "$job_id"

  # Check stats
  run "$JOB_CLI" stats "$job_id"
  assert_success
  assert_output --partial "Total runs: 1"
  assert_output --partial "Success rate: 0% (0/1)"
  assert_output --partial "Avg failure duration:"
  # Should not show success duration since there are no successes
  refute_output --partial "Avg success duration:"
}
