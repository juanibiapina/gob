#!/usr/bin/env bats

load 'test_helper'

@test "list command with no jobs shows message" {
  run "$JOB_CLI" list
  assert_success
  assert_output "No jobs found"
}

@test "list command shows running job" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # List jobs
  run "$JOB_CLI" list
  assert_success
  assert_output --regexp "[A-Za-z0-9]+: \[[0-9]+\] running: sleep 300"
}

@test "list command shows stopped job" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID and stop the process
  local job=$( "$JOB_CLI" list --json | jq '.[0]')
  local job_id=$(echo "$job" | jq -r '.id')
  local pid=$(echo "$job" | jq -r '.pid')
  "$JOB_CLI" stop "$job_id"
  wait_for_process_death "$pid"

  # List jobs
  run "$JOB_CLI" list
  assert_success
  assert_output --regexp "[A-Za-z0-9]+: \[[0-9]+\] stopped: sleep 300"
}

@test "list command shows multiple jobs" {
  # Start first job
  "$JOB_CLI" add sleep 300

  # Start second job
  "$JOB_CLI" add sleep 400

  # List jobs
  run "$JOB_CLI" list
  assert_success

  # Should show both jobs
  assert_output --regexp "sleep 300"
  assert_output --regexp "sleep 400"
}

@test "list command shows mixed running and stopped jobs" {
  # Start first job
  "$JOB_CLI" add sleep 300

  # Start second job
  "$JOB_CLI" add sleep 400

  # Kill the first job (oldest, so second in list since newest first)
  local pid=$("$JOB_CLI" list --json | jq -r '.[1].pid')
  kill "$pid"
  wait_for_process_death "$pid"

  # List jobs
  run "$JOB_CLI" list
  assert_success

  # Should show one running and one stopped
  assert_output --regexp "running: sleep 400"
  assert_output --regexp "stopped: sleep 300"
}

@test "list command shows newest jobs first" {
  # Start first job
  "$JOB_CLI" add sleep 100

  # Start second job
  "$JOB_CLI" add sleep 200

  # Start third job
  "$JOB_CLI" add sleep 300

  # List jobs
  run "$JOB_CLI" list
  assert_success

  # Extract the order of jobs by command
  output_lines=("${lines[@]}")

  # First line should be sleep 300 (newest)
  assert echo "${output_lines[0]}" | grep "sleep 300"

  # Second line should be sleep 200
  assert echo "${output_lines[1]}" | grep "sleep 200"

  # Third line should be sleep 100 (oldest)
  assert echo "${output_lines[2]}" | grep "sleep 100"
}

@test "list command output format includes job ID, PID, status, and command" {
  # Start a job
  "$JOB_CLI" add echo "test"

  # List jobs
  run "$JOB_CLI" list
  assert_success

  # Verify format: <job_id>: [<pid>] <status> [(<exit_code>)]: <command>
  # Extract job ID and PID from list --json
  local job=$("$JOB_CLI" list --json | jq '.[0]')
  local job_id=$(echo "$job" | jq -r '.id')
  local pid=$(echo "$job" | jq -r '.pid')

  # Check that output contains the expected format (exit code optional)
  assert_output --regexp "^${job_id}: \[${pid}\] (running|stopped( \([0-9]+\))?): echo test$"
}

@test "list command with --all flag shows all jobs" {
  # This test verifies --all flag works (currently shows same as default since all jobs are in same workdir)
  "$JOB_CLI" add sleep 300
  "$JOB_CLI" add sleep 300

  run "$JOB_CLI" list --all
  assert_success
  # Should show 2 jobs
  assert_output --regexp "sleep 300.*sleep 300"
}

@test "list command with --workdir flag shows working directory" {
  "$JOB_CLI" add sleep 300

  run "$JOB_CLI" list --workdir
  assert_success
  # Should contain the working directory path in parentheses
  assert_output --regexp "\(.*\)"
}

@test "list command with --all implies --workdir" {
  "$JOB_CLI" add sleep 300

  run "$JOB_CLI" list --all
  assert_success
  # Should contain the working directory path in parentheses
  assert_output --regexp "\(.*\)"
}

@test "list command with --json outputs valid JSON" {
  "$JOB_CLI" add sleep 300

  run "$JOB_CLI" list --json
  assert_success

  # Should be valid JSON array
  echo "$output" | jq -e '.' > /dev/null

  # Should have one job
  local count=$(echo "$output" | jq 'length')
  assert_equal "$count" "1"

  # Should have expected fields
  local id=$(echo "$output" | jq -r '.[0].id')
  local pid=$(echo "$output" | jq -r '.[0].pid')
  local status=$(echo "$output" | jq -r '.[0].status')
  local cmd=$(echo "$output" | jq -r '.[0].command[0]')

  assert [ -n "$id" ]
  assert [ "$pid" -gt 0 ]
  assert_equal "$status" "running"
  assert_equal "$cmd" "sleep"
}

@test "list command with --json and no jobs outputs empty array" {
  run "$JOB_CLI" list --json
  assert_success
  assert_output "[]"
}

@test "list command shows exit code for completed job" {
  # Run a command that exits with code 42
  "$JOB_CLI" add -- sh -c "exit 42"
  sleep 0.5
  
  # List jobs
  run "$JOB_CLI" list
  assert_success
  assert_output --regexp "stopped \(42\): sh"
}

@test "list command shows exit code 0 for successful job" {
  # Run a successful command
  "$JOB_CLI" add true
  sleep 0.5
  
  # List jobs
  run "$JOB_CLI" list
  assert_success
  assert_output --regexp "stopped \(0\): true"
}

@test "list command with --json includes exit code" {
  # Run a command that exits with specific code
  "$JOB_CLI" add -- sh -c "exit 7"
  sleep 0.5
  
  run "$JOB_CLI" list --json
  assert_success
  
  # Should have exit_code field
  local exit_code=$(echo "$output" | jq -r '.[0].exit_code')
  assert_equal "$exit_code" "7"
}

@test "list command shows no exit code for stopped running job" {
  # Start a long-running job
  "$JOB_CLI" add sleep 300
  
  # Get job ID and stop it
  local job_id=$("$JOB_CLI" list --json | jq -r '.[0].id')
  "$JOB_CLI" stop "$job_id"
  sleep 0.3
  
  # List jobs - should just show "stopped" without exit code
  run "$JOB_CLI" list
  assert_success
  # Should NOT have exit code in parentheses for killed process
  assert_output --regexp "stopped: sleep 300"
}
