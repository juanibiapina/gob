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
  local job_id=$(get_job_field id)

  # Wait for output to be written (logs in XDG_RUNTIME_DIR with daemon)
  wait_for_log_content "$XDG_RUNTIME_DIR/gob/${job_id}-1.stdout.log" "Hello stdout"

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
  local job_id=$(get_job_field id)

  # Wait for error output to be written (logs in XDG_RUNTIME_DIR with daemon)
  wait_for_log_content "$XDG_RUNTIME_DIR/gob/${job_id}-1.stderr.log" "No such file or directory"

  # Check stderr has error message
  run "$JOB_CLI" stderr "$job_id"
  assert_success
  assert_output --partial "No such file or directory"
}

@test "stdout and stderr are separate streams" {
  # Start job that writes to stdout
  run "$JOB_CLI" add echo "To stdout"
  assert_success

  local job_id=$(get_job_field id)

  # Wait for output to be written (logs in XDG_RUNTIME_DIR with daemon)
  wait_for_log_content "$XDG_RUNTIME_DIR/gob/${job_id}-1.stdout.log" "To stdout"

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
  local job_id=$(get_job_field id)

  # Verify log files exist (logs in XDG_RUNTIME_DIR with daemon)
  assert [ -f "$XDG_RUNTIME_DIR/gob/${job_id}-1.stdout.log" ]
  assert [ -f "$XDG_RUNTIME_DIR/gob/${job_id}-1.stderr.log" ]
}

@test "log files accumulate output over time" {
  # Start a job that writes multiple lines using printf
  run "$JOB_CLI" add printf "Line 1\nLine 2\nLine 3\n"
  assert_success

  # Extract job ID
  local job_id=$(get_job_field id)

  # Wait for output to be written (logs in XDG_RUNTIME_DIR with daemon)
  wait_for_log_content "$XDG_RUNTIME_DIR/gob/${job_id}-1.stdout.log" "Line 3"

  # Check stdout contains all lines
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  assert_output --partial "Line 1"
  assert_output --partial "Line 2"
  assert_output --partial "Line 3"
}

@test "restarted job clears previous log files" {
  # Start a job that writes unique timestamp output
  run "$JOB_CLI" add -- sh -c 'echo "run-$(date +%s%N)"'
  assert_success

  # Extract job ID
  local job_id=$(get_job_field id)

  # Wait for output to be written and process to stop (logs in XDG_RUNTIME_DIR with daemon)
  wait_for_log_content "$XDG_RUNTIME_DIR/gob/${job_id}-1.stdout.log" "run-"
  local pid=$(get_job_field pid)
  wait_for_process_death "$pid" || sleep 0.2

  # Record the first output
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  local first_output="$output"

  # Small delay to ensure different timestamp
  sleep 0.01

  # Restart the job (logs should be cleared)
  run "$JOB_CLI" restart "$job_id"
  assert_success

  # Get new PID and wait for process to finish
  local new_pid=$(get_job_field pid)
  wait_for_process_death "$new_pid" || sleep 0.2

  # Check that log file only contains new output (first output is gone)
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  refute_output --partial "$first_output"
  assert_output --partial "run-"
}

@test "stdout command handles empty output" {
  # Start a job that produces no stdout
  run "$JOB_CLI" add sleep 1
  assert_success

  # Extract job ID
  local job_id=$(get_job_field id)

  # Check stdout (should be empty but succeed)
  run "$JOB_CLI" stdout "$job_id"
  assert_success
  assert_output ""
}
