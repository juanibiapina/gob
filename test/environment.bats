#!/usr/bin/env bats

load 'test_helper'

@test "add command passes client environment to job" {
  # Set a unique env var in the client environment
  export GOB_TEST_VAR="client_value_123"

  # Run a command that prints the env var
  run "$JOB_CLI" add -- sh -c 'echo "GOB_TEST_VAR=$GOB_TEST_VAR"'
  assert_success

  # Get job ID and wait for completion
  local job_id=$(get_job_field id)
  "$JOB_CLI" await "$job_id" || true

  # Verify the env var was passed
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  assert_output --partial "GOB_TEST_VAR=client_value_123"
}

@test "job does not inherit daemon environment" {
  # First, start the daemon with a unique env var
  export GOB_DAEMON_MARKER="daemon_env_value"
  run "$JOB_CLI" ping
  assert_success

  # Now unset the env var in the client
  unset GOB_DAEMON_MARKER

  # Run a command that tries to print the daemon's env var
  run "$JOB_CLI" add -- sh -c 'echo "GOB_DAEMON_MARKER=${GOB_DAEMON_MARKER:-NOTSET}"'
  assert_success

  # Get job ID and wait for completion
  local job_id=$(get_job_field id)
  "$JOB_CLI" await "$job_id" || true

  # The daemon's env var should NOT be visible to the job
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  assert_output --partial "GOB_DAEMON_MARKER=NOTSET"
}

@test "start command uses current client environment" {
  # Set env var when creating the job
  export GOB_TEST_START_VAR="original_value"

  # Add a quick job
  run "$JOB_CLI" add -- sh -c 'echo "VAR=$GOB_TEST_START_VAR"'
  assert_success
  local job_id=$(get_job_field id)

  # Wait for job to complete
  "$JOB_CLI" await "$job_id" || true

  # Verify first run has original value
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  assert_output --partial "VAR=original_value"

  # Change the env var
  export GOB_TEST_START_VAR="changed_value"

  # Start the job again - should use the NEW environment
  run "$JOB_CLI" start "$job_id"
  assert_success

  # Wait for completion
  "$JOB_CLI" await "$job_id" || true

  # Should have the changed value
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  assert_output --partial "VAR=changed_value"
}

@test "restart command uses current client environment" {
  # Set env var when creating the job
  export GOB_TEST_RESTART_VAR="restart_original"

  # Add a job that prints env and exits
  run "$JOB_CLI" add -- sh -c 'echo "VAR=$GOB_TEST_RESTART_VAR"'
  assert_success
  local job_id=$(get_job_field id)

  # Wait for job to complete
  "$JOB_CLI" await "$job_id" || true

  # Verify first run has original value
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  assert_output --partial "VAR=restart_original"

  # Change the env var in client
  export GOB_TEST_RESTART_VAR="restart_changed"

  # Restart the job - should use the NEW environment
  run "$JOB_CLI" restart "$job_id"
  assert_success

  # Wait for completion
  "$JOB_CLI" await "$job_id" || true

  # Should have the changed value
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  assert_output --partial "VAR=restart_changed"
}

@test "job environment includes PATH" {
  # PATH is essential for running commands
  run "$JOB_CLI" add -- sh -c 'echo "PATH=$PATH"'
  assert_success

  local job_id=$(get_job_field id)
  "$JOB_CLI" await "$job_id" || true

  run "$JOB_CLI" stdout "$job_id"
  assert_success
  # PATH should be non-empty
  refute_output "PATH="
  assert_output --partial "PATH=/"
}

@test "job environment includes HOME" {
  # HOME is often needed by programs
  run "$JOB_CLI" add -- sh -c 'echo "HOME=$HOME"'
  assert_success

  local job_id=$(get_job_field id)
  "$JOB_CLI" await "$job_id" || true

  run "$JOB_CLI" stdout "$job_id"
  assert_success
  # HOME should be set
  assert_output --partial "HOME=/"
}

@test "multiple jobs can have different environments" {
  # First job with one value
  export GOB_MULTI_TEST="value_one"
  run "$JOB_CLI" add -- sh -c 'sleep 0.1 && echo "GOB_MULTI_TEST=$GOB_MULTI_TEST"'
  assert_success
  local job1_id=$(get_job_field id)

  # Second job with different value
  export GOB_MULTI_TEST="value_two"
  run "$JOB_CLI" add -- sh -c 'sleep 0.2 && echo "GOB_MULTI_TEST=$GOB_MULTI_TEST"'
  assert_success
  local job2_id=$(get_job_field id)

  # Wait for both jobs
  "$JOB_CLI" await "$job1_id" || true
  "$JOB_CLI" await "$job2_id" || true

  # First job should have first value
  run "$JOB_CLI" stdout "$job1_id"
  assert_success
  assert_output --partial "GOB_MULTI_TEST=value_one"

  # Second job should have second value
  run "$JOB_CLI" stdout "$job2_id"
  assert_success
  assert_output --partial "GOB_MULTI_TEST=value_two"
}
