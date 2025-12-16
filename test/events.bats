#!/usr/bin/env bats

load 'test_helper'

@test "events command outputs JSON for job_added event" {
  # Start events command in background
  "$JOB_CLI" events --all > "$BATS_TEST_TMPDIR/events_output.txt" 2>&1 &
  events_pid=$!

  # Give it time to subscribe
  sleep 0.3

  # Add a job
  run "$JOB_CLI" add sleep 300
  assert_success
  local job_id=$(get_job_field id)

  # Wait for event to be captured
  sleep 0.3

  # Kill the events process
  kill $events_pid 2>/dev/null || true
  wait $events_pid 2>/dev/null || true

  # Check the captured output
  run cat "$BATS_TEST_TMPDIR/events_output.txt"
  assert_output --partial '"type":"job_added"'
  assert_output --partial "\"job_id\":\"$job_id\""
}

@test "events command outputs JSON for job_stopped event when process exits naturally" {
  # Start events command in background
  "$JOB_CLI" events --all > "$BATS_TEST_TMPDIR/events_output.txt" 2>&1 &
  events_pid=$!

  # Give it time to subscribe
  sleep 0.3

  # Add a job that exits quickly
  run "$JOB_CLI" add echo "quick exit"
  assert_success
  local job_id=$(get_job_field id)

  # Wait for process to exit and event to be captured
  sleep 0.5

  # Kill the events process
  kill $events_pid 2>/dev/null || true
  wait $events_pid 2>/dev/null || true

  # Check the captured output
  run cat "$BATS_TEST_TMPDIR/events_output.txt"
  assert_output --partial '"type":"job_stopped"'
  assert_output --partial "\"job_id\":\"$job_id\""
}

@test "events command outputs JSON for explicit stop" {
  # Add a job first
  run "$JOB_CLI" add sleep 300
  assert_success
  local job_id=$(get_job_field id)

  # Start events command in background
  "$JOB_CLI" events --all > "$BATS_TEST_TMPDIR/events_output.txt" 2>&1 &
  events_pid=$!

  # Give it time to subscribe
  sleep 0.3

  # Stop the job
  run "$JOB_CLI" stop "$job_id"
  assert_success

  # Wait for event to be captured
  sleep 0.3

  # Kill the events process
  kill $events_pid 2>/dev/null || true
  wait $events_pid 2>/dev/null || true

  # Check the captured output
  run cat "$BATS_TEST_TMPDIR/events_output.txt"
  assert_output --partial '"type":"job_stopped"'
  assert_output --partial "\"job_id\":\"$job_id\""
}

@test "events command outputs JSON for job_removed event" {
  # Add a job first
  run "$JOB_CLI" add sleep 300
  assert_success
  local job_id=$(get_job_field id)

  # Stop it
  run "$JOB_CLI" stop "$job_id"
  assert_success

  # Start events command in background
  "$JOB_CLI" events --all > "$BATS_TEST_TMPDIR/events_output.txt" 2>&1 &
  events_pid=$!

  # Give it time to subscribe
  sleep 0.3

  # Remove the job
  run "$JOB_CLI" remove "$job_id"
  assert_success

  # Wait for event to be captured
  sleep 0.3

  # Kill the events process
  kill $events_pid 2>/dev/null || true
  wait $events_pid 2>/dev/null || true

  # Check the captured output
  run cat "$BATS_TEST_TMPDIR/events_output.txt"
  assert_output --partial '"type":"job_removed"'
  assert_output --partial "\"job_id\":\"$job_id\""
}

@test "events command outputs JSON for job_started event" {
  # Add and stop a job first
  run "$JOB_CLI" add sleep 300
  assert_success
  local job_id=$(get_job_field id)

  run "$JOB_CLI" stop "$job_id"
  assert_success

  # Start events command in background
  "$JOB_CLI" events --all > "$BATS_TEST_TMPDIR/events_output.txt" 2>&1 &
  events_pid=$!

  # Give it time to subscribe
  sleep 0.3

  # Start the job
  run "$JOB_CLI" start "$job_id"
  assert_success

  # Wait for event to be captured
  sleep 0.3

  # Kill the events process
  kill $events_pid 2>/dev/null || true
  wait $events_pid 2>/dev/null || true

  # Check the captured output
  run cat "$BATS_TEST_TMPDIR/events_output.txt"
  assert_output --partial '"type":"job_started"'
  assert_output --partial "\"job_id\":\"$job_id\""
}

@test "events command filters by workdir by default" {
  # Create subdirectory and add job from there
  mkdir -p "$BATS_TEST_TMPDIR/subdir"

  # Start events command from main directory (no --all flag)
  "$JOB_CLI" events > "$BATS_TEST_TMPDIR/events_output.txt" 2>&1 &
  events_pid=$!

  # Give it time to subscribe
  sleep 0.3

  # Add a job from subdirectory (different workdir)
  (cd "$BATS_TEST_TMPDIR/subdir" && "$JOB_CLI" add sleep 300)
  local job_id=$("$JOB_CLI" list --all --json | jq -r '.[0].id')

  # Wait a moment
  sleep 0.3

  # Kill the events process
  kill $events_pid 2>/dev/null || true
  wait $events_pid 2>/dev/null || true

  # Events should NOT include the job from subdir (due to workdir filtering)
  run cat "$BATS_TEST_TMPDIR/events_output.txt"
  refute_output --partial '"type":"job_added"'
}

@test "events command with --all flag shows events from all directories" {
  # Create subdirectory
  mkdir -p "$BATS_TEST_TMPDIR/subdir"

  # Start events command with --all flag
  "$JOB_CLI" events --all > "$BATS_TEST_TMPDIR/events_output.txt" 2>&1 &
  events_pid=$!

  # Give it time to subscribe
  sleep 0.3

  # Add a job from subdirectory
  (cd "$BATS_TEST_TMPDIR/subdir" && "$JOB_CLI" add sleep 300)
  local job_id=$("$JOB_CLI" list --all --json | jq -r '.[0].id')

  # Wait for event to be captured
  sleep 0.3

  # Kill the events process
  kill $events_pid 2>/dev/null || true
  wait $events_pid 2>/dev/null || true

  # Events SHOULD include the job from subdir with --all flag
  run cat "$BATS_TEST_TMPDIR/events_output.txt"
  assert_output --partial '"type":"job_added"'
  assert_output --partial "\"job_id\":\"$job_id\""
}

@test "events command receives multiple events in sequence" {
  # Start events command in background
  "$JOB_CLI" events --all > "$BATS_TEST_TMPDIR/events_output.txt" 2>&1 &
  events_pid=$!

  # Give it time to subscribe
  sleep 0.3

  # Add a job
  run "$JOB_CLI" add sleep 300
  assert_success
  local job_id=$(get_job_field id)

  # Stop it
  run "$JOB_CLI" stop "$job_id"
  assert_success

  # Remove it
  run "$JOB_CLI" remove "$job_id"
  assert_success

  # Wait for events to be captured
  sleep 0.3

  # Kill the events process
  kill $events_pid 2>/dev/null || true
  wait $events_pid 2>/dev/null || true

  # Check all events are present
  run cat "$BATS_TEST_TMPDIR/events_output.txt"
  assert_output --partial '"type":"job_added"'
  assert_output --partial '"type":"job_stopped"'
  assert_output --partial '"type":"job_removed"'
}

@test "events command outputs JSON for ports_updated event" {
  local port=$(get_random_port)

  # Start events command in background
  "$JOB_CLI" events --all > "$BATS_TEST_TMPDIR/events_output.txt" 2>&1 &
  events_pid=$!

  # Give it time to subscribe
  sleep 0.3

  # Add a job that listens on a port
  run "$JOB_CLI" add -- python3 "$BATS_TEST_DIRNAME/fixtures/port_listener.py" "$port"
  assert_success
  local job_id=$(get_job_field id)

  # Wait for port to be available
  wait_for_port "$port"

  # Wait for port polling (first poll at 2s)
  sleep 3

  # Kill the events process
  kill $events_pid 2>/dev/null || true
  wait $events_pid 2>/dev/null || true

  # Check the captured output
  run cat "$BATS_TEST_TMPDIR/events_output.txt"
  assert_output --partial '"type":"ports_updated"'
  assert_output --partial "\"job_id\":\"$job_id\""
  assert_output --partial "\"port\":$port"
}
