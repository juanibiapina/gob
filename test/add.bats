#!/usr/bin/env bats

load 'test_helper'

@test "add command requires at least one argument" {
  run "$JOB_CLI" add
  assert_failure
  assert_output --partial "requires at least 1 arg(s)"
}

@test "add command requires argument after -- separator" {
  run "$JOB_CLI" add --
  assert_failure
  assert_output --partial "requires at least 1 arg(s)"
}

@test "add command adds a background process" {
  run "$JOB_CLI" add sleep 300
  assert_success
  assert_output --regexp "Added job [A-Za-z0-9]+ running: sleep 300"

  # Extract PID from list --json
  TEST_PID=$("$JOB_CLI" list --json | jq -r '.[0].pid')

  # Verify process is running
  assert kill -0 "$TEST_PID"
}

@test "background process continues after CLI exits" {
  # Run the command
  output=$("$JOB_CLI" add sleep 300)

  # Extract PID from list --json
  TEST_PID=$("$JOB_CLI" list --json | jq -r '.[0].pid')

  # Wait a moment to ensure CLI has exited
  sleep 1

  # Verify process is still running
  assert kill -0 "$TEST_PID"
}

@test "job is tracked after add" {
  run "$JOB_CLI" add sleep 300
  assert_success

  # Job should appear in list
  run "$JOB_CLI" list --json
  assert_success

  local count=$(echo "$output" | jq 'length')
  assert_equal "$count" "1"
}

@test "job has correct command and PID" {
  run "$JOB_CLI" add sleep 300
  assert_success

  # Get job from list --json
  local job=$( "$JOB_CLI" list --json | jq '.[0]')

  # Verify PID is present and valid
  local pid=$(echo "$job" | jq -r '.pid')
  assert [ -n "$pid" ]
  assert [ "$pid" -gt 0 ]

  # Verify command is correct
  local command_length=$(echo "$job" | jq '.command | length')
  assert [ "$command_length" -eq 2 ]
  assert [ "$(echo "$job" | jq -r '.command[0]')" = "sleep" ]
  assert [ "$(echo "$job" | jq -r '.command[1]')" = "300" ]

  # Verify id is present (3-character random base62)
  local id=$(echo "$job" | jq -r '.id')
  assert [ -n "$id" ]
  assert [ ${#id} -eq 3 ]
}

@test "add command handles invalid command" {
  run "$JOB_CLI" add nonexistent_command_xyz
  assert_failure
  assert_output --partial "failed to add job"
}

@test "multiple jobs are tracked separately" {
  # Add first job
  run "$JOB_CLI" add sleep 300
  assert_success

  # Add second job
  run "$JOB_CLI" add sleep 301
  assert_success

  # Verify two jobs exist
  local job_count=$("$JOB_CLI" list --json | jq 'length')
  assert_equal "$job_count" "2"

  # Clean up second process
  local job_id2=$("$JOB_CLI" list --json | jq -r '.[0].id')
  "$JOB_CLI" stop "$job_id2"
}

@test "add command passes flags to subcommand without separator" {
  # Flags are passed directly to the subcommand (no -- needed)
  run "$JOB_CLI" add ls -a
  assert_success
  assert_output --partial "running: ls -a"
}

@test "add command supports optional -- separator" {
  # The -- separator is optional but still works
  run "$JOB_CLI" add -- ls -a
  assert_success
  assert_output --partial "running: ls -a"
}

@test "add command handles quoted command string" {
  # Quoted string with spaces is split into command + args
  run "$JOB_CLI" add "echo hello world"
  assert_success
  assert_output --partial "running: echo hello world"

  # Verify the command was split correctly
  local job=$("$JOB_CLI" list --json | jq '.[0]')
  local command_length=$(echo "$job" | jq '.command | length')
  assert [ "$command_length" -eq 3 ]
  assert [ "$(echo "$job" | jq -r '.command[0]')" = "echo" ]
  assert [ "$(echo "$job" | jq -r '.command[1]')" = "hello" ]
  assert [ "$(echo "$job" | jq -r '.command[2]')" = "world" ]
}
