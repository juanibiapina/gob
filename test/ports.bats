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
