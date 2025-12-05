#!/usr/bin/env bats

load 'test_helper'

@test "run command requires at least one argument" {
  run "$JOB_CLI" run
  assert_failure
  assert_output --partial "requires at least 1 arg(s)"
}

@test "run command creates new job when no matching job exists" {
  run "$JOB_CLI" run echo "hello"
  assert_success
  assert_output --partial "Added job"
  assert_output --partial "running: echo hello"
  assert_output --partial "completed"

  # Verify job was created
  local count=$("$JOB_CLI" list --json | jq 'length')
  assert_equal "$count" "1"
}

@test "run command reuses existing stopped job with same command" {
  # Create a job
  "$JOB_CLI" add echo "reuse-test"
  
  # Get job ID
  local job_id=$(get_job_field id)
  
  # Wait for it to complete
  sleep 0.5
  
  # Run same command again - should reuse
  run "$JOB_CLI" run echo "reuse-test"
  assert_success
  assert_output --partial "Restarted job $job_id"
  assert_output --partial "completed"
  
  # Verify still only one job
  local count=$("$JOB_CLI" list --json | jq 'length')
  assert_equal "$count" "1"
}

@test "run command errors when job with same command is already running" {
  # Start a long-running job
  "$JOB_CLI" add sleep 300
  
  # Get job ID
  local job_id=$(get_job_field id)
  
  # Try to run same command - should error
  run "$JOB_CLI" run sleep 300
  assert_failure
  assert_output --partial "already running"
  assert_output --partial "$job_id"
}

@test "run command follows output until job completes" {
  run "$JOB_CLI" run echo "output-test"
  assert_success
  
  # Should contain the output
  assert_output --partial "output-test"
  
  # Should indicate completion
  assert_output --partial "completed"
}

@test "run command handles invalid command" {
  run "$JOB_CLI" run nonexistent_command_xyz
  assert_failure
  assert_output --partial "failed to add job"
}

@test "run command distinguishes jobs by arguments" {
  # Run with arg1
  "$JOB_CLI" run echo "arg1"
  sleep 0.3
  
  # Run with arg2 - should create new job, not reuse
  run "$JOB_CLI" run echo "arg2"
  assert_success
  assert_output --partial "Added job"
  
  # Should have two jobs
  local count=$("$JOB_CLI" list --json | jq 'length')
  assert_equal "$count" "2"
}

@test "run command matches exact command and args" {
  # Create job with specific args
  "$JOB_CLI" run echo "a" "b"
  sleep 0.3
  
  local job_count_before=$("$JOB_CLI" list --json | jq 'length')
  
  # Run with different args - should create new
  "$JOB_CLI" run echo "a" "b" "c"
  sleep 0.3
  
  local job_count_after=$("$JOB_CLI" list --json | jq 'length')
  
  # Should have created a new job
  assert [ "$job_count_after" -gt "$job_count_before" ]
}

@test "run command passes flags to subcommand with -- separator" {
  # Run ls with -a flag using -- separator
  run "$JOB_CLI" run -- ls -a
  assert_success
  assert_output --partial "running: ls -a"
}

@test "run command passes flags to subcommand without -- separator" {
  # Run command with flags without needing --
  run "$JOB_CLI" run ls -la
  assert_success
  assert_output --partial "running: ls -la"
}

@test "run command handles complex flags without -- separator" {
  # Simulate pnpm --filter style command
  run "$JOB_CLI" run echo --filter web typecheck
  assert_success
  assert_output --partial "running: echo --filter web typecheck"
  assert_output --partial "--filter web typecheck"
}

@test "run command handles quoted single string command" {
  # Command passed as single quoted string
  run "$JOB_CLI" run "echo hello world"
  assert_success
  assert_output --partial "running: echo hello world"
  assert_output --partial "hello world"
}
