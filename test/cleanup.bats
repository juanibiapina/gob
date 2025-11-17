#!/usr/bin/env bats

load 'test_helper'

@test "cleanup command with no jobs" {
  run "$JOB_CLI" cleanup
  assert_success
  assert_output "Cleaned up 0 stopped job(s)"
}

@test "cleanup command removes only stopped jobs" {
  # Start a job
  "$JOB_CLI" run sleep 300

  # Get job ID and PID
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Stop the job manually
  kill "$pid"
  sleep 0.5

  # Verify process is stopped
  run kill -0 "$pid"
  assert_failure

  # Verify metadata file exists
  assert [ -f ".local/share/job/$job_id.json" ]

  # Run cleanup
  run "$JOB_CLI" cleanup
  assert_success
  assert_output "Cleaned up 1 stopped job(s)"

  # Verify metadata file was removed
  assert [ ! -f ".local/share/job/$job_id.json" ]
}

@test "cleanup command preserves running jobs" {
  # Start a job
  "$JOB_CLI" run sleep 300

  # Get job ID
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Verify process is running
  assert kill -0 "$pid"

  # Run cleanup
  run "$JOB_CLI" cleanup
  assert_success
  assert_output "Cleaned up 0 stopped job(s)"

  # Verify metadata file still exists
  assert [ -f ".local/share/job/$job_id.json" ]

  # Verify process is still running
  assert kill -0 "$pid"
}

@test "cleanup command with mixed running and stopped jobs" {
  # Start first job
  "$JOB_CLI" run sleep 300
  sleep 1

  # Start second job
  "$JOB_CLI" run sleep 400
  sleep 1

  # Start third job
  "$JOB_CLI" run sleep 500

  # Get metadata files sorted by time (newest first)
  metadata_files=($(ls -t .local/share/job/*.json))

  # Get job IDs and PIDs
  job_id1=$(basename "${metadata_files[2]}" .json)
  pid1=$(jq -r '.pid' "${metadata_files[2]}")

  job_id2=$(basename "${metadata_files[1]}" .json)
  pid2=$(jq -r '.pid' "${metadata_files[1]}")

  job_id3=$(basename "${metadata_files[0]}" .json)
  pid3=$(jq -r '.pid' "${metadata_files[0]}")

  # Stop first and third jobs
  kill "$pid1"
  kill "$pid3"
  sleep 0.5

  # Verify first and third are stopped, second is running
  run kill -0 "$pid1"
  assert_failure

  assert kill -0 "$pid2"

  run kill -0 "$pid3"
  assert_failure

  # Run cleanup
  run "$JOB_CLI" cleanup
  assert_success
  assert_output "Cleaned up 2 stopped job(s)"

  # Verify stopped jobs were removed
  assert [ ! -f ".local/share/job/$job_id1.json" ]
  assert [ ! -f ".local/share/job/$job_id3.json" ]

  # Verify running job still exists
  assert [ -f ".local/share/job/$job_id2.json" ]
  assert kill -0 "$pid2"
}

@test "cleanup command cleans up multiple stopped jobs" {
  # Start three jobs
  "$JOB_CLI" run sleep 300
  sleep 1
  "$JOB_CLI" run sleep 400
  sleep 1
  "$JOB_CLI" run sleep 500

  # Get metadata files
  metadata_files=($(ls -t .local/share/job/*.json))

  # Stop all jobs
  for metadata_file in "${metadata_files[@]}"; do
    pid=$(jq -r '.pid' "$metadata_file")
    kill "$pid"
  done
  sleep 0.5

  # Run cleanup
  run "$JOB_CLI" cleanup
  assert_success
  assert_output "Cleaned up 3 stopped job(s)"

  # Verify all metadata files were removed
  run ls .local/share/job/*.json
  assert_failure
}

@test "cleanup command is safe to run multiple times" {
  # Start a job
  "$JOB_CLI" run sleep 300

  # Get job ID and PID
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  pid=$(jq -r '.pid' "$metadata_file")

  # Stop the job
  kill "$pid"
  sleep 0.5

  # Run cleanup first time
  run "$JOB_CLI" cleanup
  assert_success
  assert_output "Cleaned up 1 stopped job(s)"

  # Run cleanup again
  run "$JOB_CLI" cleanup
  assert_success
  assert_output "Cleaned up 0 stopped job(s)"
}

@test "cleanup after using stop command" {
  # Start a job
  "$JOB_CLI" run sleep 300

  # Get job ID
  metadata_file=$(ls .local/share/job/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Stop using stop command
  "$JOB_CLI" stop "$job_id"
  sleep 0.5

  # Verify metadata file still exists (stop doesn't remove it)
  assert [ -f ".local/share/job/$job_id.json" ]

  # Run cleanup
  run "$JOB_CLI" cleanup
  assert_success
  assert_output "Cleaned up 1 stopped job(s)"

  # Verify metadata file was removed
  assert [ ! -f ".local/share/job/$job_id.json" ]
}
