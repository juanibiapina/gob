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

	var walk func(pid int32)
	walk = func(pid int32) {
		proc, err := process.NewProcess(pid)
		if err != nil {
			return // Process gone, skip
		}

		conns, err := proc.Connections()
		if err != nil {
			return // Permission issue or process gone, skip
		}

		for _, conn := range conns {
			if conn.Status == "LISTEN" {
				ports = append(ports, PortInfo{
					Port:     uint16(conn.Laddr.Port),
					Protocol: connectionTypeToString(conn.Type),
					PID:      int(pid),
					Address:  conn.Laddr.IP,
				})
			}
		}

		children, _ := proc.Children()
		for _, child := range children {
			walk(child.Pid)
		}
	}

	walk(int32(rootPID))
	return ports, nil
}

// GetJobPorts returns the listening ports for a job's process tree
func (jm *JobManager) GetJobPorts(jobID string) (*JobPorts, error) {
	job, err := jm.GetJob(jobID)
	if err != nil {
		return nil, err
	}

	// Check if job is running
	if !job.IsRunning() {
		return &JobPorts{
			JobID:   jobID,
			PID:     0,
			Ports:   []PortInfo{},
			Status:  "stopped",
			Message: "job is not running",
		}, nil
	}

	run := jm.GetCurrentRun(jobID)
	if run == nil {
		return &JobPorts{
			JobID:   jobID,
			PID:     0,
			Ports:   []PortInfo{},
			Status:  "stopped",
			Message: "job is not running",
		}, nil
	}

	ports, err := getProcessTreePorts(run.PID)
	if err != nil {
		return nil, err
	}

	return &JobPorts{
		JobID: jobID,
		PID:   run.PID,
		Ports: ports,
	}, nil
}

// RefreshJobPorts queries live ports for a job, updates the cache, and emits an event if changed
func (jm *JobManager) RefreshJobPorts(jobID string) (*JobPorts, error) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	job, ok := jm.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	// Check if job is running
	if !job.IsRunning() {
		return &JobPorts{
			JobID:   jobID,
			PID:     0,
			Ports:   []PortInfo{},
			Status:  "stopped",
			Message: "job is not running",
		}, nil
	}

	run := jm.runs[*job.CurrentRunID]
	if run == nil {
		return &JobPorts{
			JobID:   jobID,
			PID:     0,
			Ports:   []PortInfo{},
			Status:  "stopped",
			Message: "job is not running",
		}, nil
	}

	ports, err := getProcessTreePorts(run.PID)
	if err != nil {
		return nil, err
	}

	// Check if ports changed and emit event if so
	if len(ports) > 0 && !portsEqual(run.Ports, ports) {
		run.Ports = ports

		jm.emitEvent(Event{
			Type:            EventTypePortsUpdated,
			JobID:           jobID,
			Job:             jm.jobToResponse(job),
			Ports:           ports,
			JobCount:        len(jm.jobs),
			RunningJobCount: jm.countRunningJobsLocked(),
		})
	} else if len(ports) > 0 {
		// Ports unchanged but non-empty, just update cache (in case it was nil)
		run.Ports = ports
	}

	return &JobPorts{
		JobID: jobID,
		PID:   run.PID,
		Ports: ports,
	}, nil
}

// GetAllJobPorts returns listening ports for all running jobs
func (jm *JobManager) GetAllJobPorts(workdir string) ([]JobPorts, error) {
	jobs := jm.ListJobs(workdir)
	var result []JobPorts

	for _, job := range jobs {
		if !job.IsRunning() {
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
	jobs := jm.ListJobs(workdir)
	var result []JobPorts

	for _, job := range jobs {
		if !job.IsRunning() {
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
