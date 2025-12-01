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
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  original_pid=$(jq -r '.pid' "$metadata_file")

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
  new_pid=$(jq -r '.pid' "$metadata_file")

  # Verify new process is running
  assert kill -0 "$new_pid"
}

@test "start command fails if job is already running" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

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

@test "start command updates PID in metadata" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID and original PID
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  original_pid=$(jq -r '.pid' "$metadata_file")

  # Stop the job
  "$JOB_CLI" stop "$job_id"
  wait_for_process_death "$original_pid"

  # Start the job
  "$JOB_CLI" start "$job_id"

  # Get new PID from metadata
  new_pid=$(jq -r '.pid' "$metadata_file")

  # Verify PID was updated
  assert [ "$new_pid" != "$original_pid" ]

  # Verify new PID is valid
  assert [ "$new_pid" -gt 0 ]
  assert kill -0 "$new_pid"
}

@test "started job shows as running in list" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

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
  # Start a job that writes unique output
  run "$JOB_CLI" add -- sh -c "echo first-run-marker"
  assert_success

  # Extract job ID
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Wait for output to be written and process to stop
  wait_for_log_content "$XDG_DATA_HOME/gob/${job_id}.stdout.log" "first-run-marker"
  pid=$(jq -r '.pid' "$metadata_file")
  wait_for_process_death "$pid" || sleep 0.2

  # Modify the command in metadata to output different text
  jq '.command = ["sh", "-c", "echo second-run-marker"]' "$metadata_file" > "${metadata_file}.tmp"
  mv "${metadata_file}.tmp" "$metadata_file"

  # Start the job (logs should be cleared)
  run "$JOB_CLI" start "$job_id"
  assert_success

  # Get new PID and wait for process to finish
  new_pid=$(jq -r '.pid' "$metadata_file")
  wait_for_process_death "$new_pid" || sleep 0.2

  # Check that log file only contains second run output
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  assert_output "second-run-marker"
  refute_output --partial "first-run-marker"
}
