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
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Try to remove running job - should fail
  run "$JOB_CLI" remove "$job_id"
  assert_failure
  assert_output --partial "cannot remove running job: $job_id (use 'stop' first)"

  # Verify metadata file still exists
  [ -f "$metadata_file" ]
}

@test "remove command removes stopped job" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

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

  # Verify metadata file is gone
  [ ! -f "$metadata_file" ]
}

@test "remove command with invalid job ID shows error" {
  run "$JOB_CLI" remove 9999999999
  assert_failure
  assert_output --partial "job not found: 9999999999"
}

@test "remove command is not idempotent" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

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

  # Get metadata files sorted by time (newest first)
  metadata_files=($(ls -t $XDG_DATA_HOME/gob/*.json))

  # Get first job (older one)
  job_id1=$(basename "${metadata_files[1]}" .json)
  metadata_file1="${metadata_files[1]}"

  # Get second job (newer one)
  job_id2=$(basename "${metadata_files[0]}" .json)
  metadata_file2="${metadata_files[0]}"

  # Stop the first job
  pid1=$(jq -r '.pid' "$metadata_file1")
  "$JOB_CLI" stop "$job_id1"
  wait_for_process_death "$pid1"

  # Remove only the first job
  "$JOB_CLI" remove "$job_id1"

  # First job metadata should be gone
  [ ! -f "$metadata_file1" ]

  # Second job metadata should still exist
  [ -f "$metadata_file2" ]

  # List should show only the second job
  run "$JOB_CLI" list
  assert_success
  assert_output --regexp "${job_id2}: \[[0-9]+\] running: sleep 400"
  refute_output --partial "$job_id1"
}

@test "remove command removes already stopped job" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Stop the process
  "$JOB_CLI" stop "$job_id"
  wait_for_process_death "$pid"

  # Verify process is stopped
  run kill -0 "$pid"
  assert_failure

  # Remove should work even if we didn't use stop command
  run "$JOB_CLI" remove "$job_id"
  assert_success
  assert_output "Removed job $job_id (PID $pid)"

  # Verify metadata file is gone
  [ ! -f "$metadata_file" ]
}
