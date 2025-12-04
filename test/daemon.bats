#!/usr/bin/env bats

load test_helper

@test "gob ping works on first run" {
  run "$JOB_CLI" ping
  assert_success
  assert_output "pong"
}

@test "daemon persists across multiple commands" {
  # First ping starts daemon
  run "$JOB_CLI" ping
  assert_success

  # Get daemon PID
  local runtime_dir=$(get_runtime_dir)
  assert [ -f "$runtime_dir/daemon.pid" ]
  local pid1=$(cat "$runtime_dir/daemon.pid")

  # Second ping reuses daemon
  run "$JOB_CLI" ping
  assert_success

  # PID should be the same
  local pid2=$(cat "$runtime_dir/daemon.pid")
  assert_equal "$pid1" "$pid2"
}

@test "daemon auto-starts if not running" {
  # Ensure no daemon running
  kill_daemon

  local runtime_dir=$(get_runtime_dir)
  assert [ ! -f "$runtime_dir/daemon.pid" ]
  assert [ ! -S "$runtime_dir/daemon.sock" ]

  # Ping should auto-start daemon
  run "$JOB_CLI" ping
  assert_success
  assert_output "pong"

  # Daemon should be running
  assert [ -f "$runtime_dir/daemon.pid" ]
  assert [ -S "$runtime_dir/daemon.sock" ]
}

@test "multiple concurrent pings work" {
  # Run 5 pings in parallel
  for i in {1..5}; do
    "$JOB_CLI" ping &
  done

  # Wait for all to complete
  wait

  # If any failed, wait would have returned non-zero
}

@test "socket has correct permissions" {
  "$JOB_CLI" ping

  local runtime_dir=$(get_runtime_dir)
  
  # Get socket permissions
  if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS uses -f flag
    local perms=$(stat -f "%Lp" "$runtime_dir/daemon.sock")
  else
    # Linux uses -c flag
    local perms=$(stat -c "%a" "$runtime_dir/daemon.sock")
  fi

  # Should be 600 (user read/write only)
  assert_equal "$perms" "600"
}

@test "PID file contains correct daemon PID" {
  "$JOB_CLI" ping

  local runtime_dir=$(get_runtime_dir)
  local pid=$(cat "$runtime_dir/daemon.pid")

  # Process should exist
  run kill -0 "$pid"
  assert_success

  # Process should be gob daemon
  run ps -p "$pid" -o comm=
  assert_success
  assert_output --partial "gob"
}

@test "daemon shuts down gracefully on SIGTERM" {
  "$JOB_CLI" ping # Start daemon

  local runtime_dir=$(get_runtime_dir)
  local pid=$(cat "$runtime_dir/daemon.pid")

  # Send SIGTERM
  kill -TERM "$pid"

  # Wait for shutdown (max 2s)
  for i in {1..20}; do
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.1
  done

  # Daemon should be stopped
  run kill -0 "$pid" 2>/dev/null
  assert_failure

  # Socket and PID file should be removed
  assert [ ! -f "$runtime_dir/daemon.pid" ]
  assert [ ! -S "$runtime_dir/daemon.sock" ]
}

@test "daemon shuts down gracefully on SIGINT" {
  "$JOB_CLI" ping # Start daemon

  local runtime_dir=$(get_runtime_dir)
  local pid=$(cat "$runtime_dir/daemon.pid")

  # Send SIGINT
  kill -INT "$pid"

  # Wait for shutdown (max 2s)
  for i in {1..20}; do
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.1
  done

  # Daemon should be stopped
  run kill -0 "$pid" 2>/dev/null
  assert_failure

  # Socket and PID file should be removed
  assert [ ! -f "$runtime_dir/daemon.pid" ]
  assert [ ! -S "$runtime_dir/daemon.sock" ]
}

@test "daemon cleans up stale socket on start" {
  local runtime_dir=$(get_runtime_dir)
  
  # Create runtime dir and stale socket
  mkdir -p "$runtime_dir"
  touch "$runtime_dir/daemon.sock"

  # This should clean up the stale socket and start fresh
  run "$JOB_CLI" ping
  assert_success

  # Daemon should be running
  assert [ -f "$runtime_dir/daemon.pid" ]
  local pid=$(cat "$runtime_dir/daemon.pid")
  run kill -0 "$pid"
  assert_success
}

@test "daemon cleans up stale PID file on start" {
  local runtime_dir=$(get_runtime_dir)
  
  # Create runtime dir and stale PID file
  mkdir -p "$runtime_dir"
  echo "99999" > "$runtime_dir/daemon.pid"

  # This should clean up the stale PID file and start fresh
  run "$JOB_CLI" ping
  assert_success

  # Daemon should be running with correct PID
  assert [ -f "$runtime_dir/daemon.pid" ]
  local pid=$(cat "$runtime_dir/daemon.pid")
  assert [ "$pid" != "99999" ]
  run kill -0 "$pid"
  assert_success
}

@test "daemon survives after client disconnects" {
  # Run ping and get PID
  run "$JOB_CLI" ping
  assert_success
  
  local runtime_dir=$(get_runtime_dir)
  local pid1=$(cat "$runtime_dir/daemon.pid")

  # Wait a bit to ensure client fully disconnected
  sleep 0.2

  # Daemon should still be running
  run kill -0 "$pid1"
  assert_success

  # Another ping should work
  run "$JOB_CLI" ping
  assert_success
  
  local pid2=$(cat "$runtime_dir/daemon.pid")

  # Same daemon
  assert_equal "$pid1" "$pid2"
}
