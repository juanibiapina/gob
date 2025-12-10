#!/usr/bin/env bats

load 'test_helper'

@test "nuke command removes all jobs" {
  # Start multiple jobs
  run "$JOB_CLI" add sleep 300
  assert_success
  run "$JOB_CLI" add sleep 400
  assert_success

  # Verify jobs exist
  local job_count=$("$JOB_CLI" list --json | jq 'length')
  assert_equal "$job_count" "2"

  # Run nuke
  run "$JOB_CLI" nuke
  assert_success
  assert_output --partial "Cleaned up 2 total job(s)"

  # Verify no jobs remain
  run "$JOB_CLI" list --json
  assert_success
  assert_output "[]"
}

@test "nuke command removes all log files" {
  # Start a job that writes to stdout
  run "$JOB_CLI" add echo "test output"
  assert_success

  # Get job ID
  local job_id=$(get_job_field id)

  # Wait for output to be written (logs in XDG_RUNTIME_DIR with daemon)
  # Run ID is job_id-1 for the first run
  wait_for_log_content "$XDG_RUNTIME_DIR/gob/${job_id}-1.stdout.log" "test output"

  # Verify log files exist
  assert [ -f "$XDG_RUNTIME_DIR/gob/${job_id}-1.stdout.log" ]
  assert [ -f "$XDG_RUNTIME_DIR/gob/${job_id}-1.stderr.log" ]

  # Run nuke
  run "$JOB_CLI" nuke
  assert_success
  assert_output --partial "Deleted 2 log file(s)"

  # Verify log files are removed
  assert [ ! -f "$XDG_RUNTIME_DIR/gob/${job_id}-1.stdout.log" ]
  assert [ ! -f "$XDG_RUNTIME_DIR/gob/${job_id}-1.stderr.log" ]
}

@test "nuke command stops running jobs" {
  # Start a long-running job
  run "$JOB_CLI" add sleep 300
  assert_success

  # Get job ID and PID
  local job_id=$(get_job_field id)
  local pid=$(get_job_field pid)

  # Verify process is running
  run ps -p "$pid"
  assert_success

  # Run nuke
  run "$JOB_CLI" nuke
  assert_success
  assert_output --partial "Stopped 1 running job(s)"

  # Verify process is no longer running
  wait_for_process_death "$pid"
  run ps -p "$pid"
  assert_failure
}

@test "nuke command handles jobs with no log files gracefully" {
  # Start a job
  run "$JOB_CLI" add sleep 300
  assert_success

  # Get job ID
  local job_id=$(get_job_field id)

  # Manually remove log files to simulate missing logs (logs in XDG_RUNTIME_DIR with daemon)
  # Run ID is job_id-1 for the first run
  rm -f "$XDG_RUNTIME_DIR/gob/${job_id}-1.stdout.log"
  rm -f "$XDG_RUNTIME_DIR/gob/${job_id}-1.stderr.log"

  # Run nuke (should not fail even if log files are missing)
  run "$JOB_CLI" nuke
  assert_success
  assert_output --partial "Cleaned up 1 total job(s)"
}

@test "nuke command shuts down daemon" {
  # Start a job to ensure daemon is running
  run "$JOB_CLI" add sleep 300
  assert_success

  # Get daemon PID
  local daemon_pid=$(cat "$XDG_RUNTIME_DIR/gob/daemon.pid")
  assert [ -n "$daemon_pid" ]

  # Verify daemon is running
  run ps -p "$daemon_pid"
  assert_success

  # Run nuke
  run "$JOB_CLI" nuke
  assert_success
  assert_output --partial "Daemon shut down"

  # Verify daemon is no longer running
  sleep 0.5
  run ps -p "$daemon_pid"
  assert_failure
}
