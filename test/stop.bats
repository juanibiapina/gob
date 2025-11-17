#!/usr/bin/env bats

load 'test_helper'

@test "stop command requires job ID argument" {
  run "$JOB_CLI" stop
  assert_failure
  assert_output --partial "accepts 1 arg(s)"
}

@test "stop command stops a running job" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Verify process is running
  assert kill -0 "$pid"

  # Stop the job
  run "$JOB_CLI" stop "$job_id"
  assert_success
  assert_output "Stopped job $job_id (PID $pid)"

  # Wait for process to terminate
  sleep 0.5

  # Verify process is no longer running
  run kill -0 "$pid"
  assert_failure
}

@test "stop command handles already stopped job" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Stop the job manually
  kill "$pid"
  sleep 0.5

  # Try to stop again - should succeed (idempotent)
  run "$JOB_CLI" stop "$job_id"
  assert_success
  assert_output "Stopped job $job_id (PID $pid)"
}

@test "stop command with invalid job ID shows error" {
  run "$JOB_CLI" stop 9999999999
  assert_failure
  assert_output --partial "job not found: 9999999999"
}

@test "stop command with --force flag" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Verify process is running
  assert kill -0 "$pid"

  # Force stop the job
  run "$JOB_CLI" stop --force "$job_id"
  assert_success
  assert_output "Force stopped job $job_id (PID $pid)"

  # Wait for process to terminate
  sleep 0.5

  # Verify process is no longer running
  run kill -0 "$pid"
  assert_failure
}

@test "stop command with -f flag (short form)" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Verify process is running
  assert kill -0 "$pid"

  # Force stop the job using -f
  run "$JOB_CLI" stop -f "$job_id"
  assert_success
  assert_output "Force stopped job $job_id (PID $pid)"

  # Wait for process to terminate
  sleep 0.5

  # Verify process is no longer running
  run kill -0 "$pid"
  assert_failure
}

@test "stopped job shows as stopped in list" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Stop the job
  "$JOB_CLI" stop "$job_id"
  sleep 0.5

  # List jobs
  run "$JOB_CLI" list
  assert_success
  assert_output --regexp "${job_id}: \[[0-9]+\] stopped: sleep 300"
}

@test "stop command stops specific job among multiple jobs" {
  # Start first job
  "$JOB_CLI" add sleep 300
  sleep 1

  # Start second job
  "$JOB_CLI" add sleep 400

  # Get metadata files sorted by time (newest first)
  metadata_files=($(ls -t .local/share/job/*.json))

  # Get first job (older one)
  job_id1=$(basename "${metadata_files[1]}" .json)
  pid1=$(jq -r '.pid' "${metadata_files[1]}")

  # Get second job (newer one)
  job_id2=$(basename "${metadata_files[0]}" .json)
  pid2=$(jq -r '.pid' "${metadata_files[0]}")

  # Stop only the first job
  "$JOB_CLI" stop "$job_id1"
  sleep 0.5

  # First job should be stopped
  run kill -0 "$pid1"
  assert_failure

  # Second job should still be running
  assert kill -0 "$pid2"
}

@test "force stop handles already stopped job" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Stop the job manually
  kill "$pid"
  sleep 0.5

  # Try to force stop again - should succeed (idempotent)
  run "$JOB_CLI" stop --force "$job_id"
  assert_success
  assert_output "Force stopped job $job_id (PID $pid)"
}
