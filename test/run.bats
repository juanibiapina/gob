#!/usr/bin/env bats

load 'test_helper'

@test "run command requires at least one argument" {
  run "$JOB_CLI" run
  assert_failure
  assert_output --partial "requires at least 1 arg(s)"
}

@test "run command requires argument after -- separator" {
  run "$JOB_CLI" run --
  assert_failure
  assert_output --partial "requires at least 1 arg(s)"
}

@test "run command adds and waits for job completion" {
  run "$JOB_CLI" run echo "hello world"
  assert_success
  assert_output --partial "Running job"
  assert_output --partial "hello world"
  assert_output --partial "completed"
}

@test "run command shows job output" {
  run "$JOB_CLI" run -- sh -c "echo 'test output'; echo 'more output'"
  assert_success
  assert_output --partial "test output"
  assert_output --partial "more output"
}

@test "run command shows summary with command" {
  run "$JOB_CLI" run echo "summary test"
  assert_success
  assert_output --partial "Command:"
  assert_output --partial "echo summary test"
}

@test "run command shows summary with duration" {
  run "$JOB_CLI" run sleep 0.1
  assert_success
  assert_output --partial "Duration:"
}

@test "run command shows summary with exit code" {
  run "$JOB_CLI" run true
  assert_success
  assert_output --partial "Exit code: 0"
}

@test "run command exits with job exit code 0 for successful job" {
  run "$JOB_CLI" run true
  assert_success
}

@test "run command exits with job exit code for failed job" {
  run "$JOB_CLI" run -- sh -c "exit 42"
  assert_failure 42
  assert_output --partial "Exit code: 42"
}

@test "run command exits with exit code 1 for job that returns false" {
  run "$JOB_CLI" run false
  assert_failure 1
  assert_output --partial "Exit code: 1"
}

@test "run command handles invalid command" {
  run "$JOB_CLI" run nonexistent_command_xyz
  assert_failure
  assert_output --partial "failed to add job"
}

@test "run command passes flags to subcommand without separator" {
  run "$JOB_CLI" run ls -a
  assert_success
  assert_output --partial "Running job"
}

@test "run command supports optional -- separator" {
  run "$JOB_CLI" run -- ls -a
  assert_success
  assert_output --partial "Running job"
}

@test "run command handles quoted command string" {
  run "$JOB_CLI" run "echo hello world"
  assert_success
  assert_output --partial "Running job"
  assert_output --partial "hello world"
}

@test "run command shows stderr output" {
  run "$JOB_CLI" run -- sh -c "echo 'stderr message' >&2"
  assert_success
  assert_output --partial "stderr message"
}

@test "run command fails if same command is already running" {
  # Add a job with add command first
  "$JOB_CLI" add sleep 300
  local job_id=$(get_job_field id)

  # Try to run the same command - should fail
  run "$JOB_CLI" run sleep 300
  assert_failure
  assert_output --partial "is already running"
}

@test "run command shows stats for job with previous runs" {
  # Run a quick job (true exits immediately with success)
  run "$JOB_CLI" run true
  assert_success

  # First run should NOT show stats (no previous completed runs)
  refute_output --partial "Previous runs:"

  # Run same command again - should show stats from previous run
  run "$JOB_CLI" run true
  assert_success

  # Should show stats
  assert_output --partial "Previous runs: 1"
  assert_output --partial "100% success rate"
  assert_output --partial "Expected duration:"
}

@test "run command streams output in real-time" {
  # Start a job that outputs something and completes
  run "$JOB_CLI" run -- sh -c "echo 'first'; sleep 0.2; echo 'second'"
  assert_success
  assert_output --partial "first"
  assert_output --partial "second"
  assert_output --partial "completed"
}
