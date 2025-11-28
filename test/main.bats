#!/usr/bin/env bats

load 'test_helper'

@test "displays overview when run without arguments" {
  run "$JOB_CLI"
  assert_output --partial 'gob - Background Job Manager'
  assert_output --partial 'WORKFLOW'
  assert_output --partial 'gob start <command>'
  assert_output --partial 'gob list'
}
