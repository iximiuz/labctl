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
