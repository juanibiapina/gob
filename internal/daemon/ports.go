package daemon

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/process"
)

// PortInfo represents a listening port opened by a process
type PortInfo struct {
	Port     uint16 `json:"port"`
	Protocol string `json:"protocol"` // "tcp", "tcp6", "udp", "udp6"
	PID      int    `json:"pid"`
	Address  string `json:"address"` // "0.0.0.0", "127.0.0.1", "::", etc.
}

// JobPorts represents all listening ports for a job's process tree
type JobPorts struct {
	JobID   string     `json:"job_id"`
	PID     int        `json:"pid"` // Root PID of current run
	Ports   []PortInfo `json:"ports"`
	Status  string     `json:"status,omitempty"`  // "stopped" if job not running
	Message string     `json:"message,omitempty"` // Message for stopped jobs
}

// connectionTypeToString converts gopsutil connection type to string
func connectionTypeToString(connType uint32) string {
	switch connType {
	case 1:
		return "tcp"
	case 2:
		return "udp"
	case 3:
		return "tcp6"
	case 4:
		return "udp6"
	default:
		return "unknown"
	}
}

// getProcessTreePorts returns all listening ports for a process and its children
func getProcessTreePorts(rootPID int) ([]PortInfo, error) {
	var ports []PortInfo
	pids, err := getProcessTreePIDs(rootPID)
	if err != nil {
		return nil, err
	}
	for _, pid := range pids {
		proc, err := process.NewProcess(int32(pid))
		if err != nil {
			continue // Process gone, skip
		}
		conns, err := proc.Connections()
		if err != nil {
			continue // Permission issue or process gone, skip
		}
		for _, conn := range conns {
			if conn.Status == "LISTEN" {
				ports = append(ports, PortInfo{
					Port:     uint16(conn.Laddr.Port),
					Protocol: connectionTypeToString(conn.Type),
					PID:      pid,
					Address:  conn.Laddr.IP,
				})
			}
		}
	}
	return ports, nil
}

// GetJobPorts returns the most recent completed scan for a job.
func (jm *JobManager) GetJobPorts(jobID string) (*JobPorts, error) {
	job, err := jm.GetJobResponse(jobID)
	if err != nil {
		return nil, err
	}
	if job.Status == "stopped" {
		return &JobPorts{JobID: jobID, Ports: []PortInfo{}, Status: "stopped", Message: "job is not running"}, nil
	}
	return &JobPorts{JobID: jobID, PID: job.PID, Ports: job.Ports}, nil
}

// RefreshJobPorts queries live ports for a job, updates the cache, and emits an event if changed
func (jm *JobManager) RefreshJobPorts(jobID string) (*JobPorts, error) {
	jm.mu.RLock()
	job, ok := jm.jobs[jobID]
	if !ok {
		jm.mu.RUnlock()
		return nil, fmt.Errorf("job not found: %s", jobID)
	}
	if job.CurrentRunID == nil {
		jm.mu.RUnlock()
		return &JobPorts{JobID: jobID, Ports: []PortInfo{}, Status: "stopped", Message: "job is not running"}, nil
	}
	runID := *job.CurrentRunID
	pid := jm.runs[runID].PID
	jm.mu.RUnlock()

	ports, err := jm.scanPorts(pid)
	if err != nil {
		return nil, err
	}
	jm.mu.Lock()
	job, ok = jm.jobs[jobID]
	if !ok || job.CurrentRunID == nil || *job.CurrentRunID != runID {
		jm.mu.Unlock()
		return &JobPorts{JobID: jobID, Ports: []PortInfo{}, Status: "stopped", Message: "job is not running"}, nil
	}
	run := jm.runs[runID]
	changed := !portsEqual(run.Ports, ports)
	run.Ports = ports
	jm.publishListLocked()
	var event Event
	if changed {
		event = Event{Type: EventTypePortsUpdated, JobID: jobID, Job: jm.jobToResponse(job), Ports: append([]PortInfo(nil), ports...), JobCount: len(jm.jobs), RunningJobCount: jm.countRunningJobsLocked()}
	}
	jm.mu.Unlock()
	if changed {
		jm.emitEvent(event)
	}
	return &JobPorts{JobID: jobID, PID: pid, Ports: ports}, nil
}

// GetAllJobPorts returns listening ports for all running jobs
func (jm *JobManager) GetAllJobPorts(workdir string) ([]JobPorts, error) {
	jobs := jm.ListJobResponses(workdir)
	var result []JobPorts

	for _, job := range jobs {
		if job.Status == "stopped" {
			continue
		}

		jobPorts, err := jm.GetJobPorts(job.ID)
		if err != nil {
			continue // Skip jobs we can't query
		}

		// Only include if running (not stopped message)
		if jobPorts.Status == "" {
			result = append(result, *jobPorts)
		}
	}

	return result, nil
}

// RefreshAllJobPorts queries live ports for all running jobs, updates caches, and emits events
func (jm *JobManager) RefreshAllJobPorts(workdir string) ([]JobPorts, error) {
	jobs := jm.ListJobResponses(workdir)
	var result []JobPorts

	for _, job := range jobs {
		if job.Status == "stopped" {
			continue
		}

		jobPorts, err := jm.RefreshJobPorts(job.ID)
		if err != nil {
			continue // Skip jobs we can't query
		}

		// Only include if running (not stopped message)
		if jobPorts.Status == "" {
			result = append(result, *jobPorts)
		}
	}

	return result, nil
}
