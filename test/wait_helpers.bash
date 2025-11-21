# wait_helpers.bash
# Polling-based wait utilities for faster, more reliable tests

# Wait for a process to terminate
# Usage: wait_for_process_death <pid> [timeout_seconds]
wait_for_process_death() {
    local pid=$1
    local timeout=${2:-5}
    local elapsed=0
    local interval=0.05

    while kill -0 "$pid" 2>/dev/null; do
        sleep "$interval"
        elapsed=$(awk "BEGIN {print $elapsed + $interval}")
        if (( $(awk "BEGIN {print ($elapsed >= $timeout)}") )); then
            echo "Timeout waiting for process $pid to die" >&2
            return 1
        fi
    done
    return 0
}

# Wait for log file to contain a pattern
# Usage: wait_for_log_content <log_file> <pattern> [timeout_seconds]
wait_for_log_content() {
    local log_file=$1
    local pattern=$2
    local timeout=${3:-5}
    local elapsed=0
    local interval=0.05

    while ! grep -q "$pattern" "$log_file" 2>/dev/null; do
        sleep "$interval"
        elapsed=$(awk "BEGIN {print $elapsed + $interval}")
        if (( $(awk "BEGIN {print ($elapsed >= $timeout)}") )); then
            echo "Timeout waiting for pattern '$pattern' in $log_file" >&2
            return 1
        fi
    done
    return 0
}

# Wait for a job to reach a specific state
# Usage: wait_for_job_state <job_id> <state> [timeout_seconds]
wait_for_job_state() {
    local job_id=$1
    local state=$2
    local timeout=${3:-5}
    local elapsed=0
    local interval=0.05

    while ! "$JOB_CLI" list | grep "$job_id" | grep -q "$state"; do
        sleep "$interval"
        elapsed=$(awk "BEGIN {print $elapsed + $interval}")
        if (( $(awk "BEGIN {print ($elapsed >= $timeout)}") )); then
            echo "Timeout waiting for job $job_id to reach state $state" >&2
            return 1
        fi
    done
    return 0
}
