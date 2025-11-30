#!/usr/bin/env bats

load 'test_helper'

@test "run command requires at least one argument" {
  run "$JOB_CLI" run
  assert_failure
  assert_output --partial "requires at least 1 arg(s)"
}

@test "run command creates new job when no matching job exists" {
  run "$JOB_CLI" run echo "hello"
  assert_success
  assert_output --partial "Added job"
  assert_output --partial "running: echo hello"
  assert_output --partial "completed"

  # Verify job was created
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  assert [ -f "$metadata_file" ]
}

@test "run command reuses existing stopped job with same command" {
  # Create a job
  "$JOB_CLI" add echo "reuse-test"
  
  # Get job ID
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  
  # Wait for it to complete
  sleep 0.5
  
  # Run same command again - should reuse
  run "$JOB_CLI" run echo "reuse-test"
  assert_success
  assert_output --partial "Restarted job $job_id"
  assert_output --partial "completed"
  
  # Verify still only one job
  file_count=$(ls $XDG_DATA_HOME/gob/*.json | wc -l)
  assert [ "$file_count" -eq 1 ]
}

@test "run command errors when job with same command is already running" {
  # Start a long-running job
  "$JOB_CLI" add sleep 300
  
  # Get job ID
  metadata_file=$(ls $XDG_DATA_HOME/gob/*.json | head -n 1)
  job_id=$(basename "$metadata_file" .json)
  
  # Try to run same command - should error
  run "$JOB_CLI" run sleep 300
  assert_failure
  assert_output --partial "already running"
  assert_output --partial "$job_id"
}

@test "run command follows output until job completes" {
  run "$JOB_CLI" run echo "output-test"
  assert_success
  
  # Should contain the output
  assert_output --partial "output-test"
  
  # Should indicate completion
  assert_output --partial "completed"
}

@test "run command handles invalid command" {
  run "$JOB_CLI" run nonexistent_command_xyz
  assert_failure
  assert_output --partial "failed to add job"
}

@test "run command distinguishes jobs by arguments" {
  # Run with arg1
  "$JOB_CLI" run echo "arg1"
  sleep 0.3
  
  # Run with arg2 - should create new job, not reuse
  run "$JOB_CLI" run echo "arg2"
  assert_success
  assert_output --partial "Added job"
  
  # Should have two jobs
  file_count=$(ls $XDG_DATA_HOME/gob/*.json | wc -l)
  assert [ "$file_count" -eq 2 ]
}

@test "run command matches exact command and args" {
  # Create job with specific args
  "$JOB_CLI" run echo "a" "b"
  sleep 0.3
  
  job_count_before=$(ls $XDG_DATA_HOME/gob/*.json | wc -l)
  
  # Run with different args - should create new
  "$JOB_CLI" run echo "a" "b" "c"
  sleep 0.3
  
  job_count_after=$(ls $XDG_DATA_HOME/gob/*.json | wc -l)
  
  # Should have created a new job
  assert [ "$job_count_after" -gt "$job_count_before" ]
}

@test "run command passes flags to subcommand with -- separator" {
  # Run ls with -a flag using -- separator
  run "$JOB_CLI" run -- ls -a
  assert_success
  assert_output --partial "running: ls -a"
}
