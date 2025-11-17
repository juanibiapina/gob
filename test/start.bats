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
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  original_pid=$(jq -r '.pid' "$metadata_file")

  # Stop the job
  "$JOB_CLI" stop "$job_id"
  sleep 0.5

  # Verify process is stopped
  run kill -0 "$original_pid"
  assert_failure

  # Start the job again
  run "$JOB_CLI" start "$job_id"
  assert_success
  assert_output --regexp "Started job $job_id with new PID [0-9]+ running: sleep 300"

  # Get new PID
  new_pid=$(jq -r '.pid' "$metadata_file")

  # Verify new process is running
  assert kill -0 "$new_pid"

  # Verify PIDs are different
  assert [ "$new_pid" != "$original_pid" ]
}

@test "start command fails if job is already running" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Try to start already running job
  run "$JOB_CLI" start "$job_id"
  assert_failure
  assert_output --partial "is already running"
}

@test "start command with invalid job ID shows error" {
  run "$JOB_CLI" start 9999999999
  assert_failure
  assert_output --partial "job not found: 9999999999"
}

@test "start command preserves job ID" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Stop and start the job
  "$JOB_CLI" stop "$job_id"
  sleep 0.5
  "$JOB_CLI" start "$job_id"

  # Verify job ID is still the same
  new_job_id=$(basename "$metadata_file" .json)
  assert [ "$job_id" = "$new_job_id" ]

  # Verify metadata file still exists at same location
  assert [ -f "$metadata_file" ]
}

@test "start command updates PID in metadata" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID and original PID
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  original_pid=$(jq -r '.pid' "$metadata_file")

  # Stop the job
  "$JOB_CLI" stop "$job_id"
  sleep 0.5

  # Start the job again
  "$JOB_CLI" start "$job_id"

  # Get new PID from metadata
  new_pid=$(jq -r '.pid' "$metadata_file")

  # Verify PID was updated
  assert [ "$new_pid" != "$original_pid" ]

  # Verify new PID is valid
  assert [ "$new_pid" -gt 0 ]
  assert kill -0 "$new_pid"
}

@test "start command preserves command in metadata" {
  # Add a job with multiple arguments
  "$JOB_CLI" add sleep 300

  # Get job ID
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Stop and start the job
  "$JOB_CLI" stop "$job_id"
  sleep 0.5
  "$JOB_CLI" start "$job_id"

  # Verify command is preserved
  command_length=$(jq '.command | length' "$metadata_file")
  assert [ "$command_length" -eq 2 ]
  assert [ "$(jq -r '.command[0]' "$metadata_file")" = "sleep" ]
  assert [ "$(jq -r '.command[1]' "$metadata_file")" = "300" ]
}

@test "started job shows as running in list" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Stop the job
  "$JOB_CLI" stop "$job_id"
  sleep 0.5

  # Verify it shows as stopped
  run "$JOB_CLI" list
  assert_success
  assert_output --regexp "${job_id}: \[[0-9]+\] stopped: sleep 300"

  # Start the job
  "$JOB_CLI" start "$job_id"

  # Verify it shows as running
  run "$JOB_CLI" list
  assert_success
  assert_output --regexp "${job_id}: \[[0-9]+\] running: sleep 300"
}
