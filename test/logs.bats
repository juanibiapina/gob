#!/usr/bin/env bats

load 'test_helper'

@test "logs command with no jobs in current directory shows error" {
  run "$JOB_CLI" logs
  assert_failure
  assert_output --partial "no jobs found in current directory"
}

@test "logs command fails for non-existent job" {
  run "$JOB_CLI" logs 999999999
  assert_failure
  assert_output --partial "job not found"
}
