#!/usr/bin/env bats

load 'test_helper'

@test "run command requires at least one argument" {
  run "$JOB_CLI" run
  assert_failure
  assert_output --partial "requires at least 1 arg(s)"
}

@test "run command requires argument after -- separator" {
  run "$JOB_CLI" run --
  assert_failure
  assert_output --partial "requires at least 1 arg(s)"
}

@test "run command adds and waits for job completion" {
  run "$JOB_CLI" run echo "hello world"
  assert_success
  assert_output --partial "Running job"
  assert_output --partial "completed"
  assert_output --partial "Exit code: 0"
}

@test "run command suppresses output on success" {
  # Use arithmetic so the output ("val_23", "val_34") differs from the command text
  run "$JOB_CLI" run -- sh -c 'echo val_$((20+3)); echo val_$((30+4))'
  assert_success
  refute_output --partial "val_23"
  refute_output --partial "val_34"
  assert_output --partial "gob stdout"
  assert_output --partial "gob logs"
}

@test "run command shows summary with command" {
  run "$JOB_CLI" run echo "summary test"
  assert_success
  assert_output --partial "Command:"
  assert_output --partial "echo summary test"
}

@test "run command shows summary with duration" {
  run "$JOB_CLI" run sleep 0.1
  assert_success
  assert_output --partial "Duration:"
}

@test "run command shows summary with exit code" {
  run "$JOB_CLI" run true
  assert_success
  assert_output --partial "Exit code: 0"
}

@test "run command exits with job exit code 0 for successful job" {
  run "$JOB_CLI" run true
  assert_success
}

@test "run command exits with job exit code for failed job" {
  run "$JOB_CLI" run -- sh -c "exit 42"
  assert_failure 42
  assert_output --partial "Exit code: 42"
}

@test "run command exits with exit code 1 for job that returns false" {
  run "$JOB_CLI" run false
  assert_failure 1
  assert_output --partial "Exit code: 1"
}

@test "run command handles invalid command" {
  run "$JOB_CLI" run nonexistent_command_xyz
  assert_failure
  assert_output --partial "failed to add job"
}

@test "run command passes flags to subcommand without separator" {
  run "$JOB_CLI" run ls -a
  assert_success
  assert_output --partial "Running job"
}

@test "run command supports optional -- separator" {
  run "$JOB_CLI" run -- ls -a
  assert_success
  assert_output --partial "Running job"
}

@test "run command handles quoted command string" {
  run "$JOB_CLI" run "echo hello world"
  assert_success
  assert_output --partial "Running job"
}

@test "run command suppresses stderr on success" {
  # Use arithmetic so the output ("err_45") differs from the command text
  run "$JOB_CLI" run -- sh -c 'echo err_$((40+5)) >&2'
  assert_success
  refute_output --partial "err_45"
}

@test "run command attaches to already running job" {
  # Add a job with add command first
  "$JOB_CLI" add sleep 300
  local job_id=$(get_job_field id)

  # Try to run the same command - should succeed and attach
  # Use timeout since it will wait for the job
  run timeout 2 "$JOB_CLI" run sleep 300 || true
  
  # Should indicate it attached to the running job
  assert_output --partial "already running"
  assert_output --partial "attaching"
  assert_output --partial "$job_id"
}

@test "run command shows stats for job with previous runs" {
  # Run a quick job (true exits immediately with success)
  run "$JOB_CLI" run true
  assert_success

  # First run should NOT show stats (no previous completed runs)
  refute_output --partial "Previous runs:"

  # Run same command again - should show stats from previous run
  run "$JOB_CLI" run true
  assert_success

  # Should show stats but NOT expected duration (need 3+ runs for that)
  assert_output --partial "Previous runs: 1"
  assert_output --partial "100% success rate"
  refute_output --partial "Expected duration"
}

@test "run command shows expected duration after 3+ successful runs" {
  # Run a quick job 3 times to build up stats
  run "$JOB_CLI" run true
  assert_success
  run "$JOB_CLI" run true
  assert_success
  run "$JOB_CLI" run true
  assert_success

  # Fourth run should show expected duration for success
  run "$JOB_CLI" run true
  assert_success
  assert_output --partial "Previous runs: 3"
  assert_output --partial "Expected duration if success:"
}

@test "run command shows expected failure duration after 3+ failed runs" {
  # Run a failing job 3 times
  run "$JOB_CLI" run false
  assert_failure
  run "$JOB_CLI" run false
  assert_failure
  run "$JOB_CLI" run false
  assert_failure

  # Fourth run should show expected duration for failure
  run "$JOB_CLI" run false
  assert_failure
  assert_output --partial "Previous runs: 3"
  assert_output --partial "Expected duration if failure:"
}

@test "run command does not show output on success" {
  # Use arithmetic so the output ("out_56", "out_67") differs from the command text
  run "$JOB_CLI" run -- sh -c 'echo out_$((50+6)); sleep 0.2; echo out_$((60+7))'
  assert_success
  refute_output --partial "out_56"
  refute_output --partial "out_67"
  assert_output --partial "completed"
}

@test "run command shows output on failure" {
  run "$JOB_CLI" run -- sh -c "echo 'failure stdout'; echo 'failure stderr' >&2; exit 1"
  assert_failure 1
  assert_output --partial "failure stdout"
  assert_output --partial "failure stderr"
  assert_output --partial "Exit code: 1"
}

@test "run command shows helper commands on success" {
  run "$JOB_CLI" run true
  assert_success
  assert_output --partial "gob stdout"
  assert_output --partial "gob stderr"
  assert_output --partial "gob logs"
}

@test "run command does not show helper commands on failure" {
  run "$JOB_CLI" run false
  assert_failure
  refute_output --partial "gob stdout"
  refute_output --partial "gob logs"
}

@test "run command with --description stores description" {
  run "$JOB_CLI" run --description "Run test description" true
  assert_success

  # Verify description appears in list
  run "$JOB_CLI" list
  assert_success
  assert_output --partial "Run test description"
}

@test "run command with -d short flag stores description" {
  run "$JOB_CLI" run -d "Run short flag" true
  assert_success

  # Verify description appears in list
  run "$JOB_CLI" list
  assert_success
  assert_output --partial "Run short flag"
}

@test "run command with --description= syntax stores description" {
  run "$JOB_CLI" run --description="Run equals syntax" true
  assert_success

  # Verify description appears in list
  run "$JOB_CLI" list
  assert_success
  assert_output --partial "Run equals syntax"
}

@test "run command with -d= syntax stores description" {
  run "$JOB_CLI" run -d="Run short equals" true
  assert_success

  # Verify description appears in list
  run "$JOB_CLI" list
  assert_success
  assert_output --partial "Run short equals"
}

@test "run command description appears in JSON output" {
  run "$JOB_CLI" run --description "Run JSON description" true
  assert_success

  run "$JOB_CLI" list --json
  assert_success

  # Verify description field is present
  local description=$(echo "$output" | jq -r '.[0].description')
  assert_equal "$description" "Run JSON description"
}

@test "run command without description has empty description field" {
  run "$JOB_CLI" run true
  assert_success

  run "$JOB_CLI" list --json
  assert_success

  # Verify description field is null or empty
  local description=$(echo "$output" | jq '.[0].description')
  assert_equal "$description" "null"
}

@test "run command updates description on subsequent run" {
  # Run with initial description
  run "$JOB_CLI" run --description "First description" true
  assert_success

  # Run same command with new description
  run "$JOB_CLI" run --description "Updated description" true
  assert_success

  # Verify description was updated
  run "$JOB_CLI" list --json
  assert_success
  local description=$(echo "$output" | jq -r '.[0].description')
  assert_equal "$description" "Updated description"
}

@test "run command preserves description when not provided on subsequent run" {
  # Run with description
  run "$JOB_CLI" run --description "Keep this description" true
  assert_success

  # Run same command without description
  run "$JOB_CLI" run true
  assert_success

  # Verify description was preserved
  run "$JOB_CLI" list --json
  assert_success
  local description=$(echo "$output" | jq -r '.[0].description')
  assert_equal "$description" "Keep this description"
}
