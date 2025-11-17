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
  assert_output --regexp "[0-9]+: \[[0-9]+\] running: sleep 300"
}

@test "list command shows stopped job" {
  # Start a job
  "$JOB_CLI" add sleep 300

  # Get job ID and stop the process
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  "$JOB_CLI" stop "$job_id"
  sleep 0.5

  # List jobs
  run "$JOB_CLI" list
  assert_success
  assert_output --regexp "[0-9]+: \[[0-9]+\] stopped: sleep 300"
}

@test "list command shows multiple jobs" {
  # Start first job
  "$JOB_CLI" add sleep 300
  sleep 1

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
  sleep 1

  # Start second job
  "$JOB_CLI" add sleep 400

  # Kill the first job
  metadata_files=($(ls -t .local/share/gob/*.json))
  pid=$(jq -r '.pid' "${metadata_files[1]}")
  kill "$pid"
  sleep 0.5

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
  sleep 1

  # Start second job
  "$JOB_CLI" add sleep 200
  sleep 1

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

  # Verify format: <job_id>: [<pid>] <status>: <command>
  # Extract job ID from metadata file
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Check that output contains the expected format
  assert_output --regexp "^${job_id}: \[${pid}\] (running|stopped): echo test$"
}
