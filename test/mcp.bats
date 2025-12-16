#!/usr/bin/env bats

load 'test_helper/bats-support/load'
load 'test_helper/bats-assert/load'
load 'test_helper.bash'

@test "mcp command shows help" {
    run "$JOB_CLI" mcp --help
    assert_success
    assert_output --partial "Start an MCP (Model Context Protocol) server"
}

@test "mcp server responds to initialize request" {
    # Send initialize request and capture response
    response=$(echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}' \
        | timeout 5 "$JOB_CLI" mcp 2>/dev/null | head -1)
    
    # Verify it's a valid JSON-RPC response
    echo "$response" | jq -e '.result.serverInfo.name == "gob"'
}

@test "mcp server lists all tools" {
    # Create a coprocess for the MCP server
    coproc MCP { "$JOB_CLI" mcp 2>/dev/null; }
    
    # Send initialize
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}' >&${MCP[1]}
    read -r init_response <&${MCP[0]}
    
    # Send initialized notification
    echo '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&${MCP[1]}
    
    # List tools
    echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' >&${MCP[1]}
    read -r tools_response <&${MCP[0]}
    
    # Close the server
    exec {MCP[1]}>&-
    wait $MCP_PID 2>/dev/null || true
    
    # Verify all tools are listed
    echo "$tools_response" | jq -e '.result.tools[] | select(.name == "gob_add")'
    echo "$tools_response" | jq -e '.result.tools[] | select(.name == "gob_list")'
    echo "$tools_response" | jq -e '.result.tools[] | select(.name == "gob_stop")'
    echo "$tools_response" | jq -e '.result.tools[] | select(.name == "gob_start")'
    echo "$tools_response" | jq -e '.result.tools[] | select(.name == "gob_remove")'
}

@test "mcp gob_add tool creates a job" {
    # Create a coprocess for the MCP server
    coproc MCP { "$JOB_CLI" mcp 2>/dev/null; }
    
    # Send initialize
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}' >&${MCP[1]}
    read -r init_response <&${MCP[0]}
    
    # Send initialized notification
    echo '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&${MCP[1]}
    
    # Call gob_add tool
    echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"gob_add","arguments":{"command":["sleep","60"]}}}' >&${MCP[1]}
    read -r call_response <&${MCP[0]}
    
    # Close the server
    exec {MCP[1]}>&-
    wait $MCP_PID 2>/dev/null || true
    
    # Extract job_id from response
    job_id=$(echo "$call_response" | jq -r '.result.content[0].text | fromjson | .job_id')
    
    # Verify job was created by checking with gob list
    run "$JOB_CLI" list --json
    assert_success
    echo "$output" | jq -e ".[].id == \"$job_id\""
    
    # Stop the job
    "$JOB_CLI" stop "$job_id"
}

@test "mcp gob_list tool lists jobs" {
    # Create a job first
    run "$JOB_CLI" add -- sleep 60
    assert_success
    job_id=$(echo "$output" | head -1 | awk '{print $3}')
    
    # Create a coprocess for the MCP server
    coproc MCP { "$JOB_CLI" mcp 2>/dev/null; }
    
    # Send initialize
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}' >&${MCP[1]}
    read -r init_response <&${MCP[0]}
    
    # Send initialized notification
    echo '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&${MCP[1]}
    
    # Call gob_list tool
    echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"gob_list","arguments":{}}}' >&${MCP[1]}
    read -r call_response <&${MCP[0]}
    
    # Close the server
    exec {MCP[1]}>&-
    wait $MCP_PID 2>/dev/null || true
    
    # Verify job is in the list
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .jobs[] | select(.job_id == \"$job_id\")"
    
    # Cleanup
    "$JOB_CLI" stop "$job_id"
}

@test "mcp gob_stop tool stops a running job" {
    # Create a job first
    run "$JOB_CLI" add -- sleep 60
    assert_success
    job_id=$(echo "$output" | head -1 | awk '{print $3}')
    
    # Create a coprocess for the MCP server
    coproc MCP { "$JOB_CLI" mcp 2>/dev/null; }
    
    # Send initialize
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}' >&${MCP[1]}
    read -r init_response <&${MCP[0]}
    
    # Send initialized notification
    echo '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&${MCP[1]}
    
    # Call gob_stop tool
    echo "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"gob_stop\",\"arguments\":{\"job_id\":\"$job_id\"}}}" >&${MCP[1]}
    read -r call_response <&${MCP[0]}
    
    # Close the server
    exec {MCP[1]}>&-
    wait $MCP_PID 2>/dev/null || true
    
    # Verify the response
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .status == \"stopped\""
    
    # Verify job is stopped
    run "$JOB_CLI" list --json
    echo "$output" | jq -e ".[] | select(.id == \"$job_id\") | .status == \"stopped\""
}

@test "mcp gob_start tool starts a stopped job" {
    # Create and stop a job first
    run "$JOB_CLI" add -- sleep 60
    assert_success
    job_id=$(echo "$output" | head -1 | awk '{print $3}')
    "$JOB_CLI" stop "$job_id"
    
    # Create a coprocess for the MCP server
    coproc MCP { "$JOB_CLI" mcp 2>/dev/null; }
    
    # Send initialize
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}' >&${MCP[1]}
    read -r init_response <&${MCP[0]}
    
    # Send initialized notification
    echo '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&${MCP[1]}
    
    # Call gob_start tool
    echo "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"gob_start\",\"arguments\":{\"job_id\":\"$job_id\"}}}" >&${MCP[1]}
    read -r call_response <&${MCP[0]}
    
    # Close the server
    exec {MCP[1]}>&-
    wait $MCP_PID 2>/dev/null || true
    
    # Verify the response
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .status == \"running\""
    
    # Cleanup
    "$JOB_CLI" stop "$job_id"
}

@test "mcp gob_remove tool removes a stopped job" {
    # Create and stop a job first
    run "$JOB_CLI" add -- echo test
    assert_success
    job_id=$(echo "$output" | head -1 | awk '{print $3}')
    
    # Wait for it to complete
    sleep 1
    
    # Create a coprocess for the MCP server
    coproc MCP { "$JOB_CLI" mcp 2>/dev/null; }
    
    # Send initialize
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}' >&${MCP[1]}
    read -r init_response <&${MCP[0]}
    
    # Send initialized notification
    echo '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&${MCP[1]}
    
    # Call gob_remove tool
    echo "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"gob_remove\",\"arguments\":{\"job_id\":\"$job_id\"}}}" >&${MCP[1]}
    read -r call_response <&${MCP[0]}
    
    # Close the server
    exec {MCP[1]}>&-
    wait $MCP_PID 2>/dev/null || true
    
    # Verify the response
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .success == true"
    
    # Verify job is gone
    run "$JOB_CLI" list --json
    if [ "$output" != "[]" ]; then
        ! echo "$output" | jq -e ".[] | select(.id == \"$job_id\")"
    fi
}

@test "mcp gob_await tool waits for stopped job and returns output" {
    # Create a job that completes quickly
    run "$JOB_CLI" add -- sh -c "echo 'hello from job'; echo 'error output' >&2"
    assert_success
    job_id=$(echo "$output" | head -1 | awk '{print $3}')
    
    # Wait for it to complete
    sleep 1
    
    # Create a coprocess for the MCP server
    coproc MCP { "$JOB_CLI" mcp 2>/dev/null; }
    
    # Send initialize
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}' >&${MCP[1]}
    read -r init_response <&${MCP[0]}
    
    # Send initialized notification
    echo '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&${MCP[1]}
    
    # Call gob_await tool
    echo "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"gob_await\",\"arguments\":{\"job_id\":\"$job_id\"}}}" >&${MCP[1]}
    read -r call_response <&${MCP[0]}
    
    # Close the server
    exec {MCP[1]}>&-
    wait $MCP_PID 2>/dev/null || true
    
    # Verify the response contains stdout and stderr
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .stdout | contains(\"hello from job\")"
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .stderr | contains(\"error output\")"
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .exit_code == 0"
}

@test "mcp gob_await tool waits for running job to complete" {
    # Create a job that takes a moment to complete
    run "$JOB_CLI" add -- sh -c "sleep 1; echo 'completed'"
    assert_success
    job_id=$(echo "$output" | head -1 | awk '{print $3}')
    
    # Create a coprocess for the MCP server
    coproc MCP { "$JOB_CLI" mcp 2>/dev/null; }
    
    # Send initialize
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}' >&${MCP[1]}
    read -r init_response <&${MCP[0]}
    
    # Send initialized notification
    echo '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&${MCP[1]}
    
    # Call gob_await tool (should wait for job to complete)
    echo "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"gob_await\",\"arguments\":{\"job_id\":\"$job_id\",\"timeout\":10}}}" >&${MCP[1]}
    read -r call_response <&${MCP[0]}
    
    # Close the server
    exec {MCP[1]}>&-
    wait $MCP_PID 2>/dev/null || true
    
    # Verify the response contains output
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .stdout | contains(\"completed\")"
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .status == \"stopped\""
}

@test "mcp gob_run tool runs command and returns output" {
    # Create a coprocess for the MCP server
    coproc MCP { "$JOB_CLI" mcp 2>/dev/null; }
    
    # Send initialize
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}' >&${MCP[1]}
    read -r init_response <&${MCP[0]}
    
    # Send initialized notification
    echo '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&${MCP[1]}
    
    # Call gob_run tool
    echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"gob_run","arguments":{"command":["sh","-c","echo hello world; echo error >&2"]}}}' >&${MCP[1]}
    read -r call_response <&${MCP[0]}
    
    # Close the server
    exec {MCP[1]}>&-
    wait $MCP_PID 2>/dev/null || true
    
    # Verify the response contains stdout, stderr, and exit code
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .stdout | contains(\"hello world\")"
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .stderr | contains(\"error\")"
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .exit_code == 0"
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .status == \"stopped\""
}

@test "mcp gob_run tool returns stats for job with previous runs" {
    # Run a command first time (use sleep 0.01 instead of true to avoid timing issues)
    run "$JOB_CLI" run sleep 0.01
    assert_success
    
    # Create a coprocess for the MCP server
    coproc MCP { "$JOB_CLI" mcp 2>/dev/null; }
    
    # Send initialize
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}' >&${MCP[1]}
    read -r init_response <&${MCP[0]}
    
    # Send initialized notification
    echo '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&${MCP[1]}
    
    # Call gob_run tool for the same command - should include stats
    echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"gob_run","arguments":{"command":["sleep","0.01"]}}}' >&${MCP[1]}
    read -r call_response <&${MCP[0]}
    
    # Close the server
    exec {MCP[1]}>&-
    wait $MCP_PID 2>/dev/null || true
    
    # Verify the response contains stats
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .previous_runs == 1"
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .success_rate == 100"
}

@test "mcp gob_add tool returns stats for job with previous runs" {
    # Create a job that completes quickly (use sleep 0.01 instead of true to avoid timing issues)
    run "$JOB_CLI" add -- sleep 0.01
    assert_success
    job_id=$(echo "$output" | head -1 | awk '{print $3}')
    
    # Wait for it to complete
    "$JOB_CLI" await "$job_id"
    
    # Create a coprocess for the MCP server
    coproc MCP { "$JOB_CLI" mcp 2>/dev/null; }
    
    # Send initialize
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}' >&${MCP[1]}
    read -r init_response <&${MCP[0]}
    
    # Send initialized notification
    echo '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&${MCP[1]}
    
    # Call gob_add tool for the same command - should include stats
    echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"gob_add","arguments":{"command":["sleep","0.01"]}}}' >&${MCP[1]}
    read -r call_response <&${MCP[0]}
    
    # Close the server
    exec {MCP[1]}>&-
    wait $MCP_PID 2>/dev/null || true
    
    # Verify the response contains stats
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .previous_runs == 1"
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .success_rate == 100"
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .expected_duration_ms"
}

@test "mcp gob_ports tool returns ports for running job" {
    local port=$(get_random_port)
    
    # Create a job that listens on a port
    "$JOB_CLI" add -- python3 "$BATS_TEST_DIRNAME/fixtures/port_listener.py" "$port"
    job_id=$(get_job_field id)
    
    # Wait for port
    wait_for_port "$port"
    
    # Create a coprocess for the MCP server
    coproc MCP { "$JOB_CLI" mcp 2>/dev/null; }
    
    # Send initialize
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}' >&${MCP[1]}
    read -r init_response <&${MCP[0]}
    
    # Send initialized notification
    echo '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&${MCP[1]}
    
    # Call gob_ports tool
    echo "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"gob_ports\",\"arguments\":{\"job_id\":\"$job_id\"}}}" >&${MCP[1]}
    read -r call_response <&${MCP[0]}
    
    # Close the server
    exec {MCP[1]}>&-
    wait $MCP_PID 2>/dev/null || true
    
    # Verify the response contains the port
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .ports[] | select(.port == $port)"
    
    # Cleanup
    "$JOB_CLI" stop "$job_id"
}

@test "mcp gob_ports tool shows message for stopped job" {
    "$JOB_CLI" add sleep 300
    job_id=$(get_job_field id)
    "$JOB_CLI" stop "$job_id"
    
    # Create a coprocess for the MCP server
    coproc MCP { "$JOB_CLI" mcp 2>/dev/null; }
    
    # Send initialize
    echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}' >&${MCP[1]}
    read -r init_response <&${MCP[0]}
    
    # Send initialized notification
    echo '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&${MCP[1]}
    
    # Call gob_ports tool
    echo "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"gob_ports\",\"arguments\":{\"job_id\":\"$job_id\"}}}" >&${MCP[1]}
    read -r call_response <&${MCP[0]}
    
    # Close the server
    exec {MCP[1]}>&-
    wait $MCP_PID 2>/dev/null || true
    
    # Verify response shows stopped status
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .status == \"stopped\""
    echo "$call_response" | jq -e ".result.content[0].text | fromjson | .message | contains(\"not running\")"
}
