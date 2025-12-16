package daemon

import (
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
