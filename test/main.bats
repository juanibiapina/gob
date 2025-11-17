#!/usr/bin/env bats

load 'test_helper'

@test "displays overview when run without arguments" {
  run "$JOB_CLI"
  assert_output --partial 'gob - Background Job Manager'
  assert_output --partial 'BASIC WORKFLOW'
  assert_output --partial 'Add a job (starts in background):'
  assert_output --partial 'AVAILABLE COMMANDS'
  assert_output --partial 'add'
}
