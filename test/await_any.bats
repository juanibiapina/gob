#!/usr/bin/env bats

load 'test_helper'

@test "await-any command with no jobs shows message" {
  run "$JOB_CLI" await-any
  assert_success
  assert_output "No running jobs to await"
}

@test "await-any command with only stopped jobs shows no running jobs" {
  "$JOB_CLI" add true
  local job_id=$(get_job_field id)
  wait_for_job_state "$job_id" "stopped"

  run "$JOB_CLI" await-any
  assert_success
  assert_output "No running jobs to await"
}

@test "await-any command waits for first job to complete" {
  # Start two jobs with different durations
  "$JOB_CLI" add -- sleep 2
  local slow_job=$(get_job_field id 0)

  "$JOB_CLI" add -- sh -c "sleep 0.3; echo 'fast done'"
  local fast_job=$(get_job_field id 0)

  # await-any should return when the fast job completes
  run "$JOB_CLI" await-any
  assert_success
  assert_output --partial "Job $fast_job completed"
  assert_output --partial "Remaining job"
  assert_output --partial "$slow_job"
}

@test "await-any command shows initial job list" {
  "$JOB_CLI" add -- sleep 1
  local job1=$(get_job_field id 0)

  "$JOB_CLI" add -- sleep 1.1
  local job2=$(get_job_field id 0)

  # Start await-any in background and capture initial output
  run timeout 3 "$JOB_CLI" await-any
  assert_success
  assert_output --partial "Awaiting 2 jobs..."
  assert_output --partial "$job1"
  assert_output --partial "$job2"
}

@test "await-any command shows completed job summary" {
  "$JOB_CLI" add -- sh -c "sleep 0.2; echo 'test output'"
  local job_id=$(get_job_field id)

  run "$JOB_CLI" await-any
  assert_success
  assert_output --partial "Job $job_id completed"
  assert_output --partial "Command:"
  assert_output --partial "Duration:"
  assert_output --partial "Exit code: 0"
}

@test "await-any command exits with completed job's exit code" {
  "$JOB_CLI" add -- sh -c "sleep 0.2; exit 42"

  run "$JOB_CLI" await-any
  assert_failure 42
  assert_output --partial "Exit code: 42"
}

@test "await-any command exits with exit code 0 for successful job" {
  "$JOB_CLI" add -- sh -c "sleep 0.2; true"

  run "$JOB_CLI" await-any
  assert_success
  assert_output --partial "Exit code: 0"
}

@test "await-any command shows no remaining jobs when last job completes" {
  "$JOB_CLI" add -- sh -c "sleep 0.2; echo 'done'"
  local job_id=$(get_job_field id)

  run "$JOB_CLI" await-any
  assert_success
  assert_output --partial "Job $job_id completed"
  refute_output --partial "Remaining job"
}

@test "await-any command filters by current directory" {
  # Create a job in a subdirectory
  mkdir -p subdir
  pushd subdir > /dev/null
  "$JOB_CLI" add -- sleep 5
  local subdir_job=$(get_job_field id 0)
  popd > /dev/null

  # Create a job in current directory
  "$JOB_CLI" add -- sh -c "sleep 0.2; echo 'current dir'"
  local current_job=$(get_job_field id 0)

  # await-any should only see the current directory job
  run "$JOB_CLI" await-any
  assert_success
  assert_output --partial "Awaiting 1 job..."
  assert_output --partial "$current_job"
  refute_output --partial "$subdir_job"
}

@test "await-any command with --timeout exits with 124 on timeout" {
  "$JOB_CLI" add -- sleep 10

  run "$JOB_CLI" await-any --timeout 1
  assert_failure 124
  assert_output --partial "Timeout reached"
}

@test "await-any command with -t flag (short form)" {
  "$JOB_CLI" add -- sleep 10

  run "$JOB_CLI" await-any -t 1
  assert_failure 124
  assert_output --partial "Timeout reached"
}

@test "await-any command handles single job" {
  "$JOB_CLI" add -- sh -c "sleep 0.2; echo 'single'"
  local job_id=$(get_job_field id)

  run "$JOB_CLI" await-any
  assert_success
  assert_output --partial "Awaiting 1 job..."
  assert_output --partial "$job_id"
  assert_output --partial "Job $job_id completed"
}

@test "await-any command plural grammar for multiple jobs" {
  "$JOB_CLI" add -- sleep 1
  "$JOB_CLI" add -- sleep 1.1

  run timeout 3 "$JOB_CLI" await-any
  assert_success
  assert_output --partial "Awaiting 2 jobs..."
}

@test "await-any command singular grammar for one job" {
  "$JOB_CLI" add -- sh -c "sleep 0.2; echo 'one'"

  run "$JOB_CLI" await-any
  assert_success
  assert_output --partial "Awaiting 1 job..."
}

@test "await-any command remaining jobs plural grammar" {
  "$JOB_CLI" add -- sleep 3
  "$JOB_CLI" add -- sleep 3.1
  "$JOB_CLI" add -- sh -c "sleep 0.2; echo 'fast'"

  run "$JOB_CLI" await-any
  assert_success
  assert_output --partial "Remaining jobs (2):"
}

@test "await-any command remaining job singular grammar" {
  "$JOB_CLI" add -- sleep 3
  "$JOB_CLI" add -- sh -c "sleep 0.2; echo 'fast'"

  run "$JOB_CLI" await-any
  assert_success
  assert_output --partial "Remaining job (1):"
}

@test "await-any command does not accept arguments" {
  run "$JOB_CLI" await-any somearg
  assert_failure
  assert_output --partial "unknown command"
}
