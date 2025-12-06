#!/usr/bin/env bats

load 'test_helper'

@test "await-all command with no jobs shows message" {
  run "$JOB_CLI" await-all
  assert_success
  assert_output "No running jobs to await"
}

@test "await-all command with only stopped jobs shows no running jobs" {
  "$JOB_CLI" add true
  local job_id=$(get_job_field id)
  wait_for_job_state "$job_id" "stopped"

  run "$JOB_CLI" await-all
  assert_success
  assert_output "No running jobs to await"
}

@test "await-all command waits for all jobs to complete" {
  "$JOB_CLI" add -- sh -c "sleep 0.2; echo 'first'"
  local job1=$(get_job_field id 0)

  "$JOB_CLI" add -- sh -c "sleep 0.3; echo 'second'"
  local job2=$(get_job_field id 0)

  run "$JOB_CLI" await-all
  assert_success
  assert_output --partial "$job1"
  assert_output --partial "$job2"
  assert_output --partial "All jobs completed"
}

@test "await-all command shows initial job list" {
  "$JOB_CLI" add -- sleep 0.3
  local job1=$(get_job_field id 0)

  "$JOB_CLI" add -- sleep 0.3
  local job2=$(get_job_field id 0)

  run "$JOB_CLI" await-all
  assert_success
  assert_output --partial "Awaiting 2 jobs..."
  assert_output --partial "$job1"
  assert_output --partial "$job2"
}

@test "await-all command shows completion status for each job" {
  "$JOB_CLI" add -- sh -c "sleep 0.2; true"
  local job1=$(get_job_field id 0)

  "$JOB_CLI" add -- sh -c "sleep 0.3; true"
  local job2=$(get_job_field id 0)

  run "$JOB_CLI" await-all
  assert_success
  assert_output --partial "✓ $job1"
  assert_output --partial "✓ $job2"
}

@test "await-all command shows remaining count" {
  "$JOB_CLI" add -- sh -c "sleep 0.5; true"
  "$JOB_CLI" add -- sh -c "sleep 0.2; true"

  run "$JOB_CLI" await-all
  assert_success
  assert_output --partial "[1 remaining]"
}

@test "await-all command shows final summary" {
  "$JOB_CLI" add -- sh -c "sleep 0.2; true"
  "$JOB_CLI" add -- sh -c "sleep 0.3; true"

  run "$JOB_CLI" await-all
  assert_success
  assert_output --partial "All jobs completed: 2 succeeded"
}

@test "await-all command exits with 0 when all jobs succeed" {
  "$JOB_CLI" add -- sh -c "sleep 0.2; true"
  "$JOB_CLI" add -- sh -c "sleep 0.3; true"

  run "$JOB_CLI" await-all
  assert_success
}

@test "await-all command exits with first failure code" {
  "$JOB_CLI" add -- sh -c "sleep 0.3; exit 0"
  "$JOB_CLI" add -- sh -c "sleep 0.2; exit 42"

  run "$JOB_CLI" await-all
  assert_failure 42
}

@test "await-all command shows failed job status" {
  "$JOB_CLI" add -- sh -c "sleep 0.2; exit 42"
  local job_id=$(get_job_field id)

  run "$JOB_CLI" await-all
  assert_failure 42
  assert_output --partial "✗ (42) $job_id"
}

@test "await-all command summary shows failed count" {
  "$JOB_CLI" add -- sh -c "sleep 0.2; exit 1"
  "$JOB_CLI" add -- sh -c "sleep 0.3; true"

  run "$JOB_CLI" await-all
  assert_failure 1
  assert_output --partial "1 succeeded"
  assert_output --partial "1 failed"
}

@test "await-all command filters by current directory" {
  # Create a job in a subdirectory
  mkdir -p subdir
  pushd subdir > /dev/null
  "$JOB_CLI" add -- sleep 5
  local subdir_job=$(get_job_field id 0)
  popd > /dev/null

  # Create a job in current directory
  "$JOB_CLI" add -- sh -c "sleep 0.2; echo 'current dir'"
  local current_job=$(get_job_field id 0)

  # await-all should only see the current directory job
  run "$JOB_CLI" await-all
  assert_success
  assert_output --partial "Awaiting 1 job..."
  assert_output --partial "$current_job"
  refute_output --partial "$subdir_job"
}

@test "await-all command with --timeout exits with 124 on timeout" {
  "$JOB_CLI" add -- sleep 10

  run "$JOB_CLI" await-all --timeout 1
  assert_failure 124
  assert_output --partial "Timeout reached"
}

@test "await-all command with -t flag (short form)" {
  "$JOB_CLI" add -- sleep 10

  run "$JOB_CLI" await-all -t 1
  assert_failure 124
  assert_output --partial "Timeout reached"
}

@test "await-all command handles single job" {
  "$JOB_CLI" add -- sh -c "sleep 0.2; echo 'single'"
  local job_id=$(get_job_field id)

  run "$JOB_CLI" await-all
  assert_success
  assert_output --partial "Awaiting 1 job..."
  assert_output --partial "All jobs completed: 1 succeeded"
}

@test "await-all command plural grammar for multiple jobs" {
  "$JOB_CLI" add -- sleep 0.3
  "$JOB_CLI" add -- sleep 0.3

  run "$JOB_CLI" await-all
  assert_success
  assert_output --partial "Awaiting 2 jobs..."
}

@test "await-all command singular grammar for one job" {
  "$JOB_CLI" add -- sh -c "sleep 0.2; echo 'one'"

  run "$JOB_CLI" await-all
  assert_success
  assert_output --partial "Awaiting 1 job..."
}

@test "await-all command does not accept arguments" {
  run "$JOB_CLI" await-all somearg
  assert_failure
  assert_output --partial "unknown command"
}
