#!/usr/bin/env bats

load 'test_helper'

@test "restart command requires job ID argument" {
  run "$JOB_CLI" restart
  assert_failure
  assert_output --partial "accepts 1 arg(s)"
}

@test "restart command restarts a running job" {
  # Add a job
  "$JOB_CLI" start sleep 300

  # Get job ID and original PID
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  original_pid=$(jq -r '.pid' "$metadata_file")

  # Verify original process is running
  assert kill -0 "$original_pid"

  # Restart the job
  run "$JOB_CLI" restart "$job_id"
  assert_success
  assert_output --regexp "Restarted job $job_id with new PID [0-9]+ running: sleep 300"

  # Get new PID
  new_pid=$(jq -r '.pid' "$metadata_file")

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
  "$JOB_CLI" start sleep 300

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

  # Restart the job
  run "$JOB_CLI" restart "$job_id"
  assert_success
  assert_output --regexp "Restarted job $job_id with new PID [0-9]+ running: sleep 300"

  # Get new PID
  new_pid=$(jq -r '.pid' "$metadata_file")

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
  "$JOB_CLI" start sleep 300

  # Get job ID
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Restart the job
  "$JOB_CLI" restart "$job_id"

  # Verify job ID is still the same
  new_job_id=$(basename "$metadata_file" .json)
  assert [ "$job_id" = "$new_job_id" ]

  # Verify metadata file still exists at same location
  assert [ -f "$metadata_file" ]
}

@test "restart command updates PID in metadata" {
  # Add a job
  "$JOB_CLI" start sleep 300

  # Get job ID and original PID
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  original_pid=$(jq -r '.pid' "$metadata_file")

  # Restart the job
  "$JOB_CLI" restart "$job_id"

  # Get new PID from metadata
  new_pid=$(jq -r '.pid' "$metadata_file")

  # Verify PID was updated
  assert [ "$new_pid" != "$original_pid" ]

  # Verify new PID is valid
  assert [ "$new_pid" -gt 0 ]
  assert kill -0 "$new_pid"
}

@test "restart command preserves command in metadata" {
  # Add a job with multiple arguments
  "$JOB_CLI" start sleep 300

  # Get job ID
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Restart the job
  "$JOB_CLI" restart "$job_id"

  # Verify command is preserved
  command_length=$(jq '.command | length' "$metadata_file")
  assert [ "$command_length" -eq 2 ]
  assert [ "$(jq -r '.command[0]' "$metadata_file")" = "sleep" ]
  assert [ "$(jq -r '.command[1]' "$metadata_file")" = "300" ]
}

@test "restarted job shows as running in list" {
  # Add a job
  "$JOB_CLI" start sleep 300

  # Get job ID
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

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
  "$JOB_CLI" start sleep 300

  # Get job ID
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  first_pid=$(jq -r '.pid' "$metadata_file")

  # First restart
  "$JOB_CLI" restart "$job_id"
  second_pid=$(jq -r '.pid' "$metadata_file")

  # Verify PID changed
  assert [ "$second_pid" != "$first_pid" ]
  assert kill -0 "$second_pid"

  # Second restart
  "$JOB_CLI" restart "$job_id"
  third_pid=$(jq -r '.pid' "$metadata_file")

  # Verify PID changed again
  assert [ "$third_pid" != "$second_pid" ]
  assert kill -0 "$third_pid"

  # Verify second PID was stopped
  run kill -0 "$second_pid"
  assert_failure
}
