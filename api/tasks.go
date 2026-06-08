package api

type PlayTaskStatus int

const (
	PlayTaskStatusNone      PlayTaskStatus = 0
	PlayTaskStatusCreated   PlayTaskStatus = 10
	PlayTaskStatusBlocked   PlayTaskStatus = 20
	PlayTaskStatusRunning   PlayTaskStatus = 30
	PlayTaskStatusFailed    PlayTaskStatus = 35
	PlayTaskStatusCompleted PlayTaskStatus = 40
)

type PlayTask struct {
	Name    string         `json:"name"`
	Init    bool           `json:"init"`
	Helper  bool           `json:"helper"`
	Status  PlayTaskStatus `json:"status"`
	Version int            `json:"version"`
}

// PlayTaskDetails is the merged control-plane + data-plane view of a task
// returned by GET /plays/{id}/tasks. The extra (full-view) fields are only
// populated for privileged callers (super-admins, capability holders, authors).
type PlayTaskDetails struct {
	Name    string         `json:"name" yaml:"name"`
	Machine string         `json:"machine,omitempty" yaml:"machine,omitempty"`
	Status  PlayTaskStatus `json:"status" yaml:"status"`
	Version int            `json:"version" yaml:"version"`
	Init    bool           `json:"init,omitempty" yaml:"init,omitempty"`
	Helper  bool           `json:"helper,omitempty" yaml:"helper,omitempty"`

	Needs          []string `json:"needs,omitempty" yaml:"needs,omitempty"`
	Run            string   `json:"run,omitempty" yaml:"run,omitempty"`
	Env            []string `json:"env,omitempty" yaml:"env,omitempty"`
	User           string   `json:"user,omitempty" yaml:"user,omitempty"`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`
	ExitCode       int      `json:"exitCode,omitempty" yaml:"exitCode,omitempty"`
	Stdout         string   `json:"stdout,omitempty" yaml:"stdout,omitempty"`
	Stderr         string   `json:"stderr,omitempty" yaml:"stderr,omitempty"`
	LastRunAt      string   `json:"lastRunAt,omitempty" yaml:"lastRunAt,omitempty"`
	LastDurationMs int      `json:"lastDurationMs,omitempty" yaml:"lastDurationMs,omitempty"`

	Hintcheck         string   `json:"hintcheck,omitempty" yaml:"hintcheck,omitempty"`
	HintcheckExitCode int      `json:"hintcheckExitCode,omitempty" yaml:"hintcheckExitCode,omitempty"`
	HintcheckStdout   []string `json:"hintcheckStdout,omitempty" yaml:"hintcheckStdout,omitempty"`
	HintcheckStderr   []string `json:"hintcheckStderr,omitempty" yaml:"hintcheckStderr,omitempty"`

	Failcheck         string   `json:"failcheck,omitempty" yaml:"failcheck,omitempty"`
	FailcheckExitCode int      `json:"failcheckExitCode,omitempty" yaml:"failcheckExitCode,omitempty"`
	FailcheckStdout   []string `json:"failcheckStdout,omitempty" yaml:"failcheckStdout,omitempty"`
	FailcheckStderr   []string `json:"failcheckStderr,omitempty" yaml:"failcheckStderr,omitempty"`

	// DataPlaneOnly flags tasks reported by the conductor (data plane) that
	// aren't present in the control-plane task list.
	DataPlaneOnly bool `json:"dataPlaneOnly,omitempty" yaml:"dataPlaneOnly,omitempty"`
}
