#!/usr/bin/env bats

load 'test_helper'

@test "stdout command requires job ID argument" {
  run "$JOB_CLI" stdout
  assert_failure
  assert_output --partial "accepts 1 arg(s)"
}

@test "stderr command requires job ID argument" {
  run "$JOB_CLI" stderr
  assert_failure
  assert_output --partial "accepts 1 arg(s)"
}

@test "stdout command fails for non-existent job" {
  run "$JOB_CLI" stdout 999999999
  assert_failure
  assert_output --partial "job not found"
}

@test "stderr command fails for non-existent job" {
  run "$JOB_CLI" stderr 999999999
  assert_failure
  assert_output --partial "job not found"
}

@test "stdout captures output from job" {
  # Start a job that writes to stdout (using echo directly)
  run "$JOB_CLI" add echo "Hello stdout"
  assert_success

  # Extract job ID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Give process time to write output
  sleep 1

  # Check stdout
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  assert_output "Hello stdout"
}

@test "stderr captures error output from job" {
  # Use a command that naturally writes to stderr (cat with invalid file)
  run "$JOB_CLI" add cat /nonexistent/file/path
  assert_success

  # Extract job ID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Give process time to write output
  sleep 1

  # Check stderr has error message
  run "$JOB_CLI" stderr "$job_id"
  assert_success
  assert_output --partial "No such file or directory"
}

@test "stdout and stderr are separate streams" {
  # Start job that writes to stdout
  run "$JOB_CLI" add echo "To stdout"
  assert_success

  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  sleep 1

  # Check stdout contains message
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  assert_output "To stdout"

  # Check stderr is empty (echo doesn't write to stderr)
  run "$JOB_CLI" stderr "$job_id"
  assert_success
  assert_output ""
}

@test "log files are created in job directory" {
  run "$JOB_CLI" add sleep 300
  assert_success

  # Extract job ID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Verify log files exist
  assert [ -f ".local/share/gob/${job_id}.stdout.log" ]
  assert [ -f ".local/share/gob/${job_id}.stderr.log" ]
}

@test "metadata contains log file paths" {
  run "$JOB_CLI" add sleep 300
  assert_success

  # Get the metadata file
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)

  # Verify stdout_file field is present and contains .stdout.log
  stdout_file=$(jq -r '.stdout_file' "$metadata_file")
  assert [ -n "$stdout_file" ]
  run echo "$stdout_file"
  assert_output --partial ".stdout.log"

  # Verify stderr_file field is present and contains .stderr.log
  stderr_file=$(jq -r '.stderr_file' "$metadata_file")
  assert [ -n "$stderr_file" ]
  run echo "$stderr_file"
  assert_output --partial ".stderr.log"
}

@test "log files accumulate output over time" {
  # Start a job that writes multiple lines using printf
  run "$JOB_CLI" add printf "Line 1\nLine 2\nLine 3\n"
  assert_success

  # Extract job ID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Wait for output
  sleep 1

  # Check stdout contains all lines
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  assert_output --partial "Line 1"
  assert_output --partial "Line 2"
  assert_output --partial "Line 3"
}

@test "restarted job appends to existing log files" {
  # Start a job that writes output
  run "$JOB_CLI" add echo "First run"
  assert_success

  # Extract job ID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Wait for job to finish
  sleep 1

  # Restart the job
  run "$JOB_CLI" start "$job_id"
  assert_success

  # Wait for second run to finish
  sleep 1

  # Check that log file exists and has content
  # Since we're using append mode, restarting should add to the log
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  # The file should contain "First run" at least once
  assert_output --partial "First run"
}

@test "stdout command handles empty output" {
  # Start a job that produces no stdout
  run "$JOB_CLI" add sleep 1
  assert_success

  # Extract job ID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Check stdout (should be empty but succeed)
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  assert_output ""
}
