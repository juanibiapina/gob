#!/usr/bin/env bats

load 'test_helper'

@test "displays help text when run without arguments" {
  run "$JOB_CLI"
  assert_output --partial 'A CLI application to start and manage background jobs.'
  assert_output --partial 'You can use this tool to add jobs in the background, monitor their status,'
  assert_output --partial 'and manage their lifecycle.'
  assert_output --partial 'Available Commands:'
  assert_output --partial 'add'
}
