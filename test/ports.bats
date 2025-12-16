#!/usr/bin/env bats

load 'test_helper'

@test "gob manages process that listens on ports" {
    local port1=$(get_random_port)
    local port2=$(get_random_port)

    # Add job that listens on ports (main + child)
    "$JOB_CLI" add -- python3 "$BATS_TEST_DIRNAME/fixtures/port_listener.py" "$port1" "$port2"
    local job_id=$(get_job_field id)

    # Verify ports are open
    wait_for_port "$port1"
    wait_for_port "$port2"

    # Stop the job via gob
    "$JOB_CLI" stop "$job_id"
    local pid=$(get_job_field pid)
    wait_for_process_death "$pid"

    # Verify ports are closed (process tree killed)
    run nc -z localhost "$port1"
    assert_failure
    run nc -z localhost "$port2"
    assert_failure
}

@test "ports command shows message for stopped job" {
    # Add a job and stop it
    "$JOB_CLI" add sleep 300
    local job_id=$(get_job_field id)
    local pid=$(get_job_field pid)

    "$JOB_CLI" stop "$job_id"
    wait_for_process_death "$pid"

    # Get ports for stopped job - should succeed with message
    run "$JOB_CLI" ports "$job_id"
    assert_success
    assert_output --partial "is not running"
}

@test "ports command shows no ports for job without listeners" {
    # Add a job that doesn't listen on any port
    "$JOB_CLI" add sleep 300
    local job_id=$(get_job_field id)

    # Get ports
    run "$JOB_CLI" ports "$job_id"
    assert_success
    assert_output --partial "No listening ports"
}

@test "ports command shows port for job listening on single port" {
    local port=$(get_random_port)

    # Add job that listens on a port
    "$JOB_CLI" add -- python3 "$BATS_TEST_DIRNAME/fixtures/port_listener.py" "$port"
    local job_id=$(get_job_field id)

    # Wait for port to be available
    wait_for_port "$port"

    # Get ports
    run "$JOB_CLI" ports "$job_id"
    assert_success
    assert_output --partial "$port"
    assert_output --partial "tcp"
}

@test "ports command shows ports from child processes" {
    local port1=$(get_random_port)
    local port2=$(get_random_port)

    # Add job that spawns children listening on ports
    "$JOB_CLI" add -- python3 "$BATS_TEST_DIRNAME/fixtures/port_listener.py" "$port1" "$port2"
    local job_id=$(get_job_field id)

    # Wait for ports
    wait_for_port "$port1"
    wait_for_port "$port2"

    # Get ports
    run "$JOB_CLI" ports "$job_id"
    assert_success
    assert_output --partial "$port1"
    assert_output --partial "$port2"
}

@test "ports command without job_id lists all running jobs' ports" {
    local port1=$(get_random_port)
    local port2=$(get_random_port)

    # Add two jobs listening on different ports
    "$JOB_CLI" add -- python3 "$BATS_TEST_DIRNAME/fixtures/port_listener.py" "$port1"
    local job1_id=$(get_job_field id 0)

    "$JOB_CLI" add -- python3 "$BATS_TEST_DIRNAME/fixtures/port_listener.py" "$port2"
    local job2_id=$(get_job_field id 0)

    # Wait for ports
    wait_for_port "$port1"
    wait_for_port "$port2"

    # Get all ports
    run "$JOB_CLI" ports
    assert_success
    assert_output --partial "$port1"
    assert_output --partial "$port2"
}

@test "ports command with --all flag shows ports from all directories" {
    local port=$(get_random_port)

    # Create a subdirectory and add job there
    mkdir -p subdir
    cd subdir
    "$JOB_CLI" add -- python3 "$BATS_TEST_DIRNAME/fixtures/port_listener.py" "$port"
    local job_id=$(get_job_field id)
    cd ..

    # Wait for port
    wait_for_port "$port"

    # Without --all, should not see job from different directory
    run "$JOB_CLI" ports
    assert_success
    refute_output --partial "$port"

    # With --all, should see it
    run "$JOB_CLI" ports --all
    assert_success
    assert_output --partial "$port"
}

@test "ports command with --json outputs JSON format" {
    local port=$(get_random_port)

    "$JOB_CLI" add -- python3 "$BATS_TEST_DIRNAME/fixtures/port_listener.py" "$port"
    local job_id=$(get_job_field id)

    wait_for_port "$port"

    run "$JOB_CLI" ports --json "$job_id"
    assert_success

    # Verify JSON structure
    echo "$output" | jq -e ".job_id == \"$job_id\""
    echo "$output" | jq -e ".ports[0].port == $port"
    echo "$output" | jq -e ".ports[0].protocol == \"tcp\""
}

@test "ports command shows empty list when no running jobs" {
    run "$JOB_CLI" ports
    assert_success
    assert_output --partial "No listening ports"
}

@test "ports command with invalid job ID shows error" {
    run "$JOB_CLI" ports nonexistent
    assert_failure
    assert_output --partial "job not found"
}
