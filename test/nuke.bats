#!/usr/bin/env bats

load 'test_helper'

@test "nuke command removes all metadata files" {
  # Start multiple jobs (nanosecond timestamps ensure unique job IDs)
  run "$JOB_CLI" start sleep 300
  assert_success
  run "$JOB_CLI" start sleep 300
  assert_success

  # Verify metadata files exist
  metadata_count=$(ls .local/share/gob/*.json 2>/dev/null | wc -l)
  assert [ "$metadata_count" -eq 2 ]

  # Run nuke
  run "$JOB_CLI" nuke
  assert_success
  assert_output --partial "Cleaned up 2 total job(s)"

  # Verify no metadata files remain
  metadata_count=$(ls .local/share/gob/*.json 2>/dev/null | wc -l)
  assert [ "$metadata_count" -eq 0 ]
}

@test "nuke command removes all log files" {
  # Start a job that writes to stdout
  run "$JOB_CLI" start echo "test output"
  assert_success

  # Get job ID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Wait for output to be written
  wait_for_log_content ".local/share/gob/${job_id}.stdout.log" "test output"

  # Verify log files exist
  assert [ -f ".local/share/gob/${job_id}.stdout.log" ]
  assert [ -f ".local/share/gob/${job_id}.stderr.log" ]

  # Run nuke
  run "$JOB_CLI" nuke
  assert_success
  assert_output --partial "Deleted 2 log file(s)"

  # Verify log files are removed
  assert [ ! -f ".local/share/gob/${job_id}.stdout.log" ]
  assert [ ! -f ".local/share/gob/${job_id}.stderr.log" ]
}

@test "nuke command stops running jobs" {
  # Start a long-running job
  run "$JOB_CLI" start sleep 300
  assert_success

  # Get job ID and PID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  pid=$(jq -r '.pid' "$metadata_file")

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
  run "$JOB_CLI" start sleep 300
  assert_success

  # Get job ID
  metadata_file=$(ls .local/share/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)

  # Manually remove log files to simulate missing logs
  rm -f ".local/share/gob/${job_id}.stdout.log"
  rm -f ".local/share/gob/${job_id}.stderr.log"

  # Run nuke (should not fail even if log files are missing)
  run "$JOB_CLI" nuke
  assert_success
  assert_output --partial "Cleaned up 1 total job(s)"
}
