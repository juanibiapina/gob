#!/usr/bin/env bats

setup() {
  load 'test_helper/bats-support/load'
  load 'test_helper/bats-assert/load'
}

@test "displays help text when run without arguments" {
  run go run .
  assert_output 'A CLI application to start and manage background jobs.

You can use this tool to run jobs in the background, monitor their status,
and manage their lifecycle.'
}
