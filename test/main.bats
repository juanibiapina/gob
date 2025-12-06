#!/usr/bin/env bats

load 'test_helper'

@test "displays overview when run without arguments" {
  run "$JOB_CLI"
  assert_output --partial 'gob - Background Job Manager'
  assert_output --partial 'RUNNING COMMANDS'
  assert_output --partial 'gob add -- <command>'
  assert_output --partial 'gob list'
}
