#!/usr/bin/env bats

load 'test_helper'

@test "await command requires job ID argument" {
  run "$JOB_CLI" await
  assert_failure
  assert_output --partial "accepts 1 arg(s)"
}

@test "await command with invalid job ID shows error" {
  run "$JOB_CLI" await nonexistent
  assert_failure
  assert_output --partial "job not found"
}

@test "await command on running job follows output until completion" {
  # Start a job that outputs something and completes quickly
  "$JOB_CLI" add -- sh -c "echo 'test output'; sleep 0.3; echo 'done'"
  local job_id=$(get_job_field id)
  
  # Await should follow and show the output
  run "$JOB_CLI" await "$job_id"
  assert_success
  assert_output --partial "test output"
  assert_output --partial "done"
  assert_output --partial "completed"
}

@test "await command on stopped job shows existing output" {
  # Create and wait for job to complete
  "$JOB_CLI" add echo "existing output"
  local job_id=$(get_job_field id)
  sleep 0.5
  
  # Job should be stopped now
  wait_for_job_state "$job_id" "stopped"
  
  # Await should show the existing output
  run "$JOB_CLI" await "$job_id"
  assert_success
  assert_output --partial "existing output"
  assert_output --partial "completed"
}

@test "await command shows summary with command" {
  "$JOB_CLI" add echo "summary test"
  local job_id=$(get_job_field id)
  sleep 0.5
  wait_for_job_state "$job_id" "stopped"
  
  run "$JOB_CLI" await "$job_id"
  assert_success
  assert_output --partial "Command:"
  assert_output --partial "echo summary test"
}

@test "await command shows summary with duration" {
  "$JOB_CLI" add sleep 0.2
  local job_id=$(get_job_field id)
  sleep 0.5
  wait_for_job_state "$job_id" "stopped"
  
  run "$JOB_CLI" await "$job_id"
  assert_success
  assert_output --partial "Duration:"
}

@test "await command shows summary with exit code" {
  "$JOB_CLI" add true
  local job_id=$(get_job_field id)
  sleep 0.3
  wait_for_job_state "$job_id" "stopped"
  
  run "$JOB_CLI" await "$job_id"
  assert_success
  assert_output --partial "Exit code: 0"
}

@test "await command exits with job exit code 0 for successful job" {
  "$JOB_CLI" add true
  local job_id=$(get_job_field id)
  sleep 0.3
  wait_for_job_state "$job_id" "stopped"
  
  run "$JOB_CLI" await "$job_id"
  assert_success
}

@test "await command exits with job exit code for failed job" {
  "$JOB_CLI" add -- sh -c "exit 42"
  local job_id=$(get_job_field id)
  sleep 0.3
  wait_for_job_state "$job_id" "stopped"
  
  run "$JOB_CLI" await "$job_id"
  assert_failure 42
  assert_output --partial "Exit code: 42"
}

@test "await command exits with exit code 1 for job that returns false" {
  "$JOB_CLI" add false
  local job_id=$(get_job_field id)
  sleep 0.3
  wait_for_job_state "$job_id" "stopped"
  
  run "$JOB_CLI" await "$job_id"
  assert_failure 1
  assert_output --partial "Exit code: 1"
}

@test "await command on running job waits for completion" {
  # Start a job that takes a bit of time
  "$JOB_CLI" add -- sh -c "sleep 0.5; echo 'finished'"
  local job_id=$(get_job_field id)
  
  # Await should wait and show completion
  run "$JOB_CLI" await "$job_id"
  assert_success
  assert_output --partial "finished"
  assert_output --partial "completed"
}

@test "await command shows stderr output for stopped job" {
  "$JOB_CLI" add -- sh -c "echo 'stderr message' >&2"
  local job_id=$(get_job_field id)
  sleep 0.3
  wait_for_job_state "$job_id" "stopped"
  
  run "$JOB_CLI" await "$job_id"
  assert_success
  assert_output --partial "stderr message"
}

@test "await command indicates job is stopped in header" {
  "$JOB_CLI" add echo "test"
  local job_id=$(get_job_field id)
  sleep 0.3
  wait_for_job_state "$job_id" "stopped"
  
  run "$JOB_CLI" await "$job_id"
  assert_success
  assert_output --partial "(stopped)"
}

@test "await command indicates awaiting for running job" {
  "$JOB_CLI" add sleep 0.5
  local job_id=$(get_job_field id)
  
  run "$JOB_CLI" await "$job_id"
  assert_success
  assert_output --partial "Awaiting job"
}
