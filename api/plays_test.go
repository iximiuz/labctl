package api

import "testing"

func TestPlayIsBroken(t *testing.T) {
	tests := []struct {
		name       string
		status     *PlayStatus
		wantBroken bool
		wantReason string
	}{
		{
			name:   "no status",
			status: nil,
		},
		{
			name: "starting",
			status: &PlayStatus{
				StateEvents: []StateEvent{{State: StateCreated}, {State: StateWarmingUp}},
				Machines: []MachineStatus{{
					Name:       "m-01",
					Conditions: []Condition{{Name: machineReadyCondition, Status: "False", Message: "waiting for sshd"}},
				}},
			},
		},
		{
			name: "failed with machine condition",
			status: &PlayStatus{
				StateEvents: []StateEvent{
					{State: StateCreated},
					{State: StateWarmingUp},
					{State: StateFailed, Error: true},
					{State: StateDestroying},
					{State: StateDestroyed},
				},
				Machines: []MachineStatus{
					{
						Name: "target-01",
						Conditions: []Condition{{
							Name:    "DriveVolumeCreated",
							Status:  "False",
							Message: "drive 0: could not create overlay drive volume",
						}},
					},
					{Name: "driver-01"},
				},
			},
			wantBroken: true,
			wantReason: "target-01: drive 0: could not create overlay drive volume",
		},
		{
			name: "failed without details",
			status: &PlayStatus{
				StateEvents: []StateEvent{{State: StateCreated}, {State: StateFailed, Error: true}},
			},
			wantBroken: true,
			wantReason: "the playground entered the FAILED state",
		},
		{
			name: "destroyed",
			status: &PlayStatus{
				StateEvents: []StateEvent{{State: StateRunning}, {State: StateDestroying}},
			},
			wantBroken: true,
			wantReason: "the playground is being destroyed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Play{Status: tt.status}
			broken, reason := p.IsBroken()
			if broken != tt.wantBroken || reason != tt.wantReason {
				t.Errorf("IsBroken() = (%v, %q), want (%v, %q)", broken, reason, tt.wantBroken, tt.wantReason)
			}
		})
	}
}
