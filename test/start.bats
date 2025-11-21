#!/usr/bin/env bats

load 'test_helper'

@test "start command requires at least one argument" {
  run "$JOB_CLI" start
  assert_failure
  assert_output --partial "requires at least 1 arg(s)"
}

@test "start command starts a background process" {
  run "$JOB_CLI" start sleep 300
  assert_success
  assert_output --regexp "Started job [0-9]+ running: sleep 300"

  # Extract PID from metadata file
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json 2>/dev/null | head -n 1)
  TEST_PID=$(jq -r '.pid' "$metadata_file")

  # Verify process is running
  assert kill -0 "$TEST_PID"
}

@test "background process continues after CLI exits" {
  # Run the command
  output=$("$JOB_CLI" start sleep 300)

  # Extract PID from metadata file
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  TEST_PID=$(jq -r '.pid' "$metadata_file")

  # Wait a moment to ensure CLI has exited
  sleep 1

  # Verify process is still running
  assert kill -0 "$TEST_PID"
}

@test "metadata file is created in XDG data directory" {
  run "$JOB_CLI" start sleep 300
  assert_success

  # Check that metadata directory exists
  assert [ -d "$XDG_DATA_HOME/gob" ]

  # Check that a JSON file was created
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  assert [ -f "$metadata_file" ]

  # Clean up
  TEST_PID=$(jq -r '.pid' "$metadata_file")
}

@test "metadata contains correct command and PID" {
  run "$JOB_CLI" start sleep 300
  assert_success

  # Get the metadata file
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)

  # Verify PID is present and valid
  TEST_PID=$(jq -r '.pid' "$metadata_file")
  assert [ -n "$TEST_PID" ]
  assert [ "$TEST_PID" -gt 0 ]

  # Verify command is correct
  command_length=$(jq '.command | length' "$metadata_file")
  assert [ "$command_length" -eq 2 ]
  assert [ "$(jq -r '.command[0]' "$metadata_file")" = "sleep" ]
  assert [ "$(jq -r '.command[1]' "$metadata_file")" = "300" ]

  # Verify id (timestamp) is present
  id=$(jq -r '.id' "$metadata_file")
  assert [ -n "$id" ]
  assert [ "$id" -gt 0 ]
}

@test "start command with multiple arguments" {
  run "$JOB_CLI" start sleep 300
  assert_success
  assert_output --regexp "Started job [0-9]+ running: sleep 300"

  # Extract PID and verify process is running
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  TEST_PID=$(jq -r '.pid' "$metadata_file")
  assert kill -0 "$TEST_PID"
}

@test "start command handles invalid command" {
  run "$JOB_CLI" start nonexistent_command_xyz
  assert_failure
  assert_output --partial "failed to start job"
}

@test "multiple jobs create separate metadata files" {
  # Start first job
  run "$JOB_CLI" start sleep 300
  assert_success
  metadata_file1=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  TEST_PID=$(jq -r '.pid' "$metadata_file1")

  # Start second job (nanosecond timestamps ensure uniqueness)
  run "$JOB_CLI" start sleep 300
  assert_success

  # Verify two separate files exist
  file_count=$(ls $XDG_DATA_HOME/gob/*.json | wc -l)
  assert [ "$file_count" -eq 2 ]

  # Clean up second process
  metadata_file2=$(ls $XDG_DATA_HOME/gob/*.json | tail -n 1)
  job_id2=$(basename "$metadata_file2" .json)
  "$JOB_CLI" stop "$job_id2"
}
