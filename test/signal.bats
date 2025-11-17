#!/usr/bin/env bats

load 'test_helper'

@test "signal command requires job ID and signal arguments" {
  run "$JOB_CLI" signal
  assert_failure
  assert_output --partial "accepts 2 arg(s)"
}

@test "signal command requires signal argument" {
  run "$JOB_CLI" signal 123456
  assert_failure
  assert_output --partial "accepts 2 arg(s)"
}

@test "signal command sends TERM signal by name" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Verify process is running
  assert kill -0 "$pid"

  # Send TERM signal
  run "$JOB_CLI" signal "$job_id" TERM
  assert_success
  assert_output "Sent signal TERM to job $job_id (PID $pid)"

  # Wait for process to terminate
  sleep 0.5

  # Verify process stopped
  run kill -0 "$pid"
  assert_failure
}

@test "signal command sends signal by name with SIG prefix" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Send SIGTERM signal
  run "$JOB_CLI" signal "$job_id" SIGTERM
  assert_success
  assert_output "Sent signal SIGTERM to job $job_id (PID $pid)"

  # Wait for process to terminate
  sleep 0.5

  # Verify process stopped
  run kill -0 "$pid"
  assert_failure
}

@test "signal command sends signal by number" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Send signal 15 (SIGTERM)
  run "$JOB_CLI" signal "$job_id" 15
  assert_success
  assert_output "Sent signal 15 to job $job_id (PID $pid)"

  # Wait for process to terminate
  sleep 0.5

  # Verify process stopped
  run kill -0 "$pid"
  assert_failure
}

@test "signal command sends INT signal" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Send INT signal
  run "$JOB_CLI" signal "$job_id" INT
  assert_success
  assert_output "Sent signal INT to job $job_id (PID $pid)"

  # Wait for process to terminate
  sleep 0.5

  # Verify process stopped
  run kill -0 "$pid"
  assert_failure
}

@test "signal command sends KILL signal" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Send KILL signal
  run "$JOB_CLI" signal "$job_id" KILL
  assert_success
  assert_output "Sent signal KILL to job $job_id (PID $pid)"

  # Wait for process to terminate
  sleep 0.5

  # Verify process stopped
  run kill -0 "$pid"
  assert_failure
}

@test "signal command is idempotent on stopped job" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Stop the job
  "$JOB_CLI" stop "$job_id"
  sleep 0.5

  # Send signal to stopped job - should succeed
  run "$JOB_CLI" signal "$job_id" TERM
  assert_success
  assert_output "Sent signal TERM to job $job_id (PID $pid)"
}

@test "signal command with invalid job ID shows error" {
  run "$JOB_CLI" signal 9999999999 TERM
  assert_failure
  assert_output --partial "job not found: 9999999999"
}

@test "signal command with invalid signal shows error" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Try to send invalid signal
  run "$JOB_CLI" signal "$job_id" INVALID
  assert_failure
  assert_output --partial "invalid signal: INVALID"
}

@test "signal command accepts lowercase signal names" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Send signal with lowercase name
  run "$JOB_CLI" signal "$job_id" term
  assert_success
  assert_output "Sent signal term to job $job_id (PID $pid)"

  # Wait for process to terminate
  sleep 0.5

  # Verify process stopped
  run kill -0 "$pid"
  assert_failure
}

@test "signal command sends HUP signal" {
  # Add a job
  "$JOB_CLI" add sleep 300

  # Get job ID and PID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

  # Send HUP signal
  run "$JOB_CLI" signal "$job_id" HUP
  assert_success
  assert_output "Sent signal HUP to job $job_id (PID $pid)"

  # HUP terminates processes that don't handle it
  sleep 0.5

  # Verify process stopped (sleep doesn't handle SIGHUP)
  run kill -0 "$pid"
  assert_failure
}
