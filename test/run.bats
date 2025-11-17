#!/usr/bin/env bats

load 'test_helper/bats-support/load'
load 'test_helper/bats-assert/load'

setup() {
  # Use BATS provided temporary directory
  cd "$BATS_TEST_TMPDIR"

  JOB_CLI="$BATS_TEST_DIRNAME/../dist/job"
}

teardown() {
  # Kill any background processes we started
  if [ -n "$TEST_PID" ] && kill -0 "$TEST_PID" 2>/dev/null; then
    kill "$TEST_PID" 2>/dev/null || true
  fi
}

@test "run command requires at least one argument" {
  run "$JOB_CLI" run
  assert_failure
  assert_output --partial "requires at least 1 arg(s)"
}

@test "run command starts a background process" {
  run "$JOB_CLI" run sleep 300
  assert_success
  assert_output --regexp "Started job [0-9]+ running: sleep 300"

  # Extract PID from metadata file
  metadata_file=$(ls .local/share/job/*.json 2>/dev/null | head -n 1)
  TEST_PID=$(jq -r '.pid' "$metadata_file")

  # Verify process is running
  assert kill -0 "$TEST_PID"
}

@test "background process continues after CLI exits" {
  # Run the command
  output=$("$JOB_CLI" run sleep 300)

  # Extract PID from metadata file
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  TEST_PID=$(jq -r '.pid' "$metadata_file")

  # Wait a moment to ensure CLI has exited
  sleep 1

  # Verify process is still running
  assert kill -0 "$TEST_PID"
}

@test "metadata file is created in .local/share/job" {
  run "$JOB_CLI" run sleep 300
  assert_success

  # Check that metadata directory exists
  assert [ -d ".local/share/job" ]

  # Check that a JSON file was created
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  assert [ -f "$metadata_file" ]

  # Clean up
  TEST_PID=$(jq -r '.pid' "$metadata_file")
}

@test "metadata contains correct command and PID" {
  run "$JOB_CLI" run sleep 300
  assert_success

  # Get the metadata file
  metadata_file=$(ls .local/share/job/*.json | head -n 1)

  # Verify PID is present and valid
  TEST_PID=$(jq -r '.pid' "$metadata_file")
  assert [ -n "$TEST_PID" ]
  assert [ "$TEST_PID" -gt 0 ]

  # Verify command is correct
  command_length=$(jq '.command | length' "$metadata_file")
  assert [ "$command_length" -eq 2 ]
  assert [ "$(jq -r '.command[0]' "$metadata_file")" = "sleep" ]
  assert [ "$(jq -r '.command[1]' "$metadata_file")" = "300" ]

  # Verify started_at timestamp is present
  started_at=$(jq -r '.started_at' "$metadata_file")
  assert [ -n "$started_at" ]
  assert [ "$started_at" -gt 0 ]

  # Verify work_dir is present
  work_dir=$(jq -r '.work_dir' "$metadata_file")
  assert [ "$work_dir" = "$BATS_TEST_TMPDIR" ]
}

@test "run command with multiple arguments" {
  run "$JOB_CLI" run sleep 300
  assert_success
  assert_output --regexp "Started job [0-9]+ running: sleep 300"

  # Extract PID and verify process is running
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  TEST_PID=$(jq -r '.pid' "$metadata_file")
  assert kill -0 "$TEST_PID"
}

@test "run command handles invalid command" {
  run "$JOB_CLI" run nonexistent_command_xyz
  assert_failure
  assert_output --partial "failed to start job"
}

@test "multiple jobs create separate metadata files" {
  # Start first job
  run "$JOB_CLI" run sleep 300
  assert_success
  metadata_file1=$(ls .local/share/job/*.json | head -n 1)
  TEST_PID=$(jq -r '.pid' "$metadata_file1")

  # Wait a moment to ensure different timestamps
  sleep 1

  # Start second job
  run "$JOB_CLI" run sleep 300
  assert_success

  # Verify two separate files exist
  file_count=$(ls .local/share/job/*.json | wc -l)
  assert [ "$file_count" -eq 2 ]

  # Clean up second process
  metadata_file2=$(ls .local/share/job/*.json | tail -n 1)
  PID2=$(jq -r '.pid' "$metadata_file2")
  kill "$PID2" 2>/dev/null || true
}
