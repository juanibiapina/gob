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

@test "add command returns already_running for same command" {
  # Add a job
  run "$JOB_CLI" add sleep 300
  assert_success

  # Get the job ID
  local job_id=$(get_job_field id)

  # Try to add the same command again - should succeed with "already running" message
  run "$JOB_CLI" add sleep 300
  assert_success
  assert_output --partial "already running"
  assert_output --partial "$job_id"

  # Different command should also succeed (creates new job)
  run "$JOB_CLI" add sleep 301
  assert_success
}

@test "add command reuses existing job after stopped" {
  # Add a job
  run "$JOB_CLI" add sleep 300
  assert_success
  local job1_id=$(get_job_field id)

  # Stop the job
  "$JOB_CLI" stop "$job1_id"
  local pid=$(get_job_field pid)
  wait_for_process_death "$pid"

  # Add same command again - should reuse the job
  run "$JOB_CLI" add sleep 300
  assert_success
  local job2_id=$(get_job_field id)

  # Should be the same job ID (job was reused)
  assert_equal "$job1_id" "$job2_id"
}

@test "add command shows stats for job with previous runs" {
  # Add a quick job (use sleep 0.01 instead of true to avoid timing issues in CI)
  run "$JOB_CLI" add sleep 0.01
  assert_success
  local job_id=$(get_job_field id)

  # First add should NOT show stats (no previous completed runs)
  refute_output --partial "Previous runs:"

  # Wait for job to complete
  "$JOB_CLI" await "$job_id"

  # Add same command again - should show stats from previous run
  run "$JOB_CLI" add sleep 0.01
  assert_success

  # Should show stats but NOT expected duration (need 3+ runs for that)
  assert_output --partial "Previous runs: 1"
  assert_output --partial "100% success rate"
  refute_output --partial "Expected duration"
}

@test "add command shows expected duration after 3+ successful runs" {
  # Run a quick job 3 times to build up stats
  run "$JOB_CLI" add sleep 0.01
  assert_success
  local job_id=$(get_job_field id)
  "$JOB_CLI" await "$job_id"

  run "$JOB_CLI" add sleep 0.01
  assert_success
  "$JOB_CLI" await "$job_id"

  run "$JOB_CLI" add sleep 0.01
  assert_success
  "$JOB_CLI" await "$job_id"

  # Fourth add should show expected duration for success
  run "$JOB_CLI" add sleep 0.01
  assert_success
  assert_output --partial "Previous runs: 3"
  assert_output --partial "Expected duration if success:"
}

@test "add command with --description stores description" {
  run "$JOB_CLI" add --description "Test job description" sleep 300
  assert_success

  # Verify description appears in list
  run "$JOB_CLI" list
  assert_success
  assert_output --partial "Test job description"
}

@test "add command with -d short flag stores description" {
  run "$JOB_CLI" add -d "Short flag description" sleep 300
  assert_success

  # Verify description appears in list
  run "$JOB_CLI" list
  assert_success
  assert_output --partial "Short flag description"
}

@test "add command with --description= syntax stores description" {
  run "$JOB_CLI" add --description="Equals syntax" sleep 300
  assert_success

  # Verify description appears in list
  run "$JOB_CLI" list
  assert_success
  assert_output --partial "Equals syntax"
}

@test "add command with -d= syntax stores description" {
  run "$JOB_CLI" add -d="Short equals syntax" sleep 300
  assert_success

  # Verify description appears in list
  run "$JOB_CLI" list
  assert_success
  assert_output --partial "Short equals syntax"
}

@test "add command description appears in JSON output" {
  run "$JOB_CLI" add --description "JSON description" sleep 300
  assert_success

  run "$JOB_CLI" list --json
  assert_success

  # Verify description field is present
  local description=$(echo "$output" | jq -r '.[0].description')
  assert_equal "$description" "JSON description"
}

@test "add command without description has empty description field" {
  run "$JOB_CLI" add sleep 300
  assert_success

  run "$JOB_CLI" list --json
  assert_success

  # Verify description field is null or empty
  local description=$(echo "$output" | jq '.[0].description')
  assert_equal "$description" "null"
}

@test "add command updates description on subsequent run" {
  # Add job with initial description
  run "$JOB_CLI" add --description "First description" sleep 0.01
  assert_success
  local job_id=$(get_job_field id)

  # Wait for job to complete
  "$JOB_CLI" await "$job_id"

  # Add same command with new description
  run "$JOB_CLI" add --description "Updated description" sleep 0.01
  assert_success

  # Verify description was updated
  run "$JOB_CLI" list --json
  assert_success
  local description=$(echo "$output" | jq -r '.[0].description')
  assert_equal "$description" "Updated description"
}

@test "add command preserves description when not provided on subsequent run" {
  # Add job with description
  run "$JOB_CLI" add --description "Keep this description" sleep 0.01
  assert_success
  local job_id=$(get_job_field id)

  # Wait for job to complete
  "$JOB_CLI" await "$job_id"

  # Add same command without description
  run "$JOB_CLI" add sleep 0.01
  assert_success

  # Verify description was preserved
  run "$JOB_CLI" list --json
  assert_success
  local description=$(echo "$output" | jq -r '.[0].description')
  assert_equal "$description" "Keep this description"
}
