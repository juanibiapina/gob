#!/usr/bin/env bats

setup() {
  load 'test_helper/bats-support/load'
  load 'test_helper/bats-assert/load'
}

@test "displays help text when run without arguments" {
  run go run .
  assert_output --partial 'A CLI application to start and manage background jobs.'
  assert_output --partial 'You can use this tool to run jobs in the background, monitor their status,'
  assert_output --partial 'and manage their lifecycle.'
  assert_output --partial 'Available Commands:'
  assert_output --partial 'run'
}
