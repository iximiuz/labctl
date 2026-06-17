package tui

import (
	"bytes"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/config"
	"github.com/iximiuz/labctl/internal/labcli"
)

func testCLI() labcli.CLI {
	cli := labcli.NewCLI(io.NopCloser(bytes.NewReader(nil)), &bytes.Buffer{}, &bytes.Buffer{}, "test")
	cli.SetConfig(config.Default("/tmp"))
	return cli
}

func runeKey(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func playWithState(id string, st api.PlayState) *api.Play {
	return &api.Play{
		ID:         id,
		Playground: api.Playground{Name: "pg"},
		Status:     &api.PlayStatus{StateEvents: []api.StateEvent{{State: st}}},
	}
}

// TestDelegateMovesCursor guards the regression where delegate had a value
// receiver and dropped the table's updated cursor, breaking up/down navigation.
func TestDelegateMovesCursor(t *testing.T) {
	m := model{tab: tabPlays, playsTable: table.New(table.WithFocused(true))}
	m.setSizes(80, 24)
	m.playsTable.SetRows([]table.Row{
		{"a", "", "", "", ""},
		{"b", "", "", "", ""},
		{"c", "", "", "", ""},
	})

	if got := m.playsTable.Cursor(); got != 0 {
		t.Fatalf("start cursor = %d, want 0", got)
	}

	m.delegate(tea.KeyMsg{Type: tea.KeyDown})

	if got := m.playsTable.Cursor(); got != 1 {
		t.Fatalf("after down, cursor = %d, want 1 (delegate must persist table state)", got)
	}
}

// TestViewRenders smoke-tests the k9s-style chrome: it must render the titled
// frame, breadcrumbs, and logo without panicking on a fresh (empty) model.
func TestViewRenders(t *testing.T) {
	cli := labcli.NewCLI(io.NopCloser(bytes.NewReader(nil)), &bytes.Buffer{}, &bytes.Buffer{}, "test")
	cli.SetConfig(config.Default("/tmp"))

	m := newModel(cli)
	m.width, m.height = 100, 30
	m.setSizes(100, 30)

	out := m.View()
	for _, want := range []string{"playgrounds", "catalog", "[0]", "User:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("View() missing %q\n---\n%s", want, out)
		}
	}
}

// TestDoubleQuit guards the Claude-CLI-style quit: one ctrl+c arms (no quit),
// a second one actually quits.
func TestDoubleQuit(t *testing.T) {
	m := newModel(testCLI())
	ctrlC := tea.KeyMsg{Type: tea.KeyCtrlC}

	m1, _ := m.handleKey(ctrlC)
	if !m1.(model).quitArmed {
		t.Fatal("first ctrl+c should arm quit, not quit immediately")
	}

	_, cmd := m1.(model).handleKey(ctrlC)
	if cmd == nil {
		t.Fatal("second ctrl+c should return a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("second ctrl+c cmd = %T, want tea.QuitMsg", cmd())
	}
}

// TestFilterMapping guards that filtering keeps selectedPlay pointing at the
// right play (cursor indexes the filtered slice, not the full list).
func TestFilterMapping(t *testing.T) {
	m := newModel(testCLI())
	m.plays = []*api.Play{
		{ID: "aaa", Playground: api.Playground{Name: "docker"}},
		{ID: "bbb", Playground: api.Playground{Name: "k3s"}},
	}
	m.filter = "k3s"
	m.refreshRows()

	if len(m.filteredPlays) != 1 {
		t.Fatalf("filtered len = %d, want 1", len(m.filteredPlays))
	}
	if p := m.selectedPlay(); p == nil || p.ID != "bbb" {
		t.Fatalf("selectedPlay = %v, want play bbb", p)
	}
}

// TestStatusFlashClears guards that an action status auto-clears, and that a
// stale clear (superseded by a newer flash) does not wipe the current status.
func TestStatusFlashClears(t *testing.T) {
	m := newModel(testCLI())

	cmd := m.flash("Destroyed xyz")
	if m.status != "Destroyed xyz" {
		t.Fatalf("status = %q, want flash text", m.status)
	}
	cleared, _ := m.Update(cmd()) // fire the scheduled clear
	if s := cleared.(model).status; s != "" {
		t.Fatalf("status after clear = %q, want empty", s)
	}

	stale := m.flash("first")    // seq N
	m.flash("second")            // seq N+1, status="second"
	kept, _ := m.Update(stale()) // stale clear for seq N must be ignored
	if s := kept.(model).status; s != "second" {
		t.Fatalf("stale clear wiped status = %q, want \"second\"", s)
	}
}

// TestQuitConfirm guards that q opens a confirmation popup and only the Quit
// button actually quits.
func TestQuitConfirm(t *testing.T) {
	m := newModel(testCLI())
	q := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}

	opened, _ := m.handleKey(q)
	mo := opened.(model)
	if mo.modal != modalQuit {
		t.Fatal("q should open the quit-confirmation popup, not quit")
	}

	// Cancel (default button) closes without quitting.
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	cancelled, cmd := mo.handleKey(enter)
	if cancelled.(model).modal != modalNone || cmd != nil {
		t.Fatal("enter on Cancel should close the popup without quitting")
	}

	// Move to Quit, then enter quits.
	mo.quitBtn = 1
	_, cmd = mo.handleKey(enter)
	if cmd == nil {
		t.Fatal("enter on Quit should return a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("quit cmd = %T, want tea.QuitMsg", cmd())
	}
}

// TestStartStopToggle guards that `s` is state-aware: stop a running playground,
// start a stopped one.
func TestStartStopToggle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		state      api.PlayState
		wantStatus string
	}{
		{name: "running stops", state: api.StateRunning, wantStatus: "Stopping..."},
		{name: "stopped starts", state: api.StateStopped, wantStatus: "Starting..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := newModel(testCLI())
			m.plays = []*api.Play{playWithState("p1", tt.state)}
			m.refreshRows()

			out, cmd := m.handlePlaysKey(runeKey("s"))
			if got := out.(model).status; got != tt.wantStatus {
				t.Fatalf("status = %q, want %q", got, tt.wantStatus)
			}
			if cmd == nil {
				t.Fatal("toggle should return an action command")
			}
		})
	}
}

// TestExtendValidation guards the lifetime parsing in the extend dialog: valid
// durations submit, junk and sub-minute values are rejected with a ✗ status.
func TestExtendValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		input      string
		wantPrefix string
	}{
		{name: "minutes", input: "90m", wantPrefix: "Setting lifetime..."},
		{name: "hours", input: "3h", wantPrefix: "Setting lifetime..."},
		{name: "not a duration", input: "abc", wantPrefix: errMark},
		{name: "below one minute", input: "30s", wantPrefix: errMark},
		{name: "empty", input: "", wantPrefix: errMark},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := newModel(testCLI())
			m.modal = modalExtend
			m.extendBtn = 0 // field focused; enter submits
			m.extendID = "p1"
			m.input.SetValue(tt.input)

			out, _ := m.handleExtendSelectKey(tea.KeyMsg{Type: tea.KeyEnter})
			if got := out.(model).status; !strings.HasPrefix(got, tt.wantPrefix) {
				t.Fatalf("input %q: status = %q, want prefix %q", tt.input, got, tt.wantPrefix)
			}
		})
	}
}

func TestThemeIndex(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "default is first", in: "k9s", want: 0},
		{name: "known theme", in: "dracula", want: 1},
		{name: "unknown falls back to 0", in: "nope", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := themeIndex(tt.in); got != tt.want {
				t.Fatalf("themeIndex(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestPersistAction guards that `p` triggers a persist command.
func TestPersistAction(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	m.plays = []*api.Play{playWithState("p1", api.StateRunning)}
	m.refreshRows()

	out, cmd := m.handlePlaysKey(runeKey("P"))
	if got := out.(model).status; got != "Persisting..." {
		t.Fatalf("status = %q, want %q", got, "Persisting...")
	}
	if cmd == nil {
		t.Fatal("persist should return a command")
	}
}

// TestTooSmallFloor guards the minimum-size message below the layout floor.
func TestTooSmallFloor(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	m.width, m.height = 50, 10
	if out := m.View(); !strings.Contains(out, "too small") {
		t.Fatalf("View at 50x10 should show the too-small message, got:\n%s", out)
	}
}

// TestNoOverflowAcrossWidths guards that no rendered line exceeds the terminal
// width (the header degrades and columns shrink instead of overflowing).
func TestNoOverflowAcrossWidths(t *testing.T) {
	t.Parallel()
	for _, wh := range [][2]int{{60, 14}, {62, 16}, {80, 24}, {120, 40}} {
		m := newModel(testCLI())
		m.width, m.height = wh[0], wh[1]
		m.setSizes(wh[0], wh[1])
		m.plays = []*api.Play{playWithState("abcd1234ef56", api.StateRunning)}
		m.refreshRows()
		for _, line := range strings.Split(m.View(), "\n") {
			if got := lipgloss.Width(line); got > wh[0] {
				t.Fatalf("%dx%d: line width %d > %d: %q", wh[0], wh[1], got, wh[0], line)
			}
		}
	}
}

func TestDistribute(t *testing.T) {
	t.Parallel()
	mins := []int{10, 8, 8, 6}
	got := distribute(54, []int{3, 2, 3, 2}, mins)
	sum := 0
	for i, v := range got {
		if v < mins[i] {
			t.Fatalf("col %d = %d below min %d", i, v, mins[i])
		}
		sum += v
	}
	if sum != 54 {
		t.Fatalf("sum = %d, want 54", sum)
	}
}

// TestShareTerminalOpens guards that `w` opens the share-access dialog defaulting
// to Private.
func TestShareTerminalOpens(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	m.plays = []*api.Play{playWithState("p1", api.StateRunning)}
	m.refreshRows()

	out, _ := m.handlePlaysKey(runeKey("w"))
	mo := out.(model)
	if mo.modal != modalShare {
		t.Fatalf("modal = %v, want modalShare", mo.modal)
	}
	if mo.shareBtn != 0 {
		t.Fatalf("shareBtn = %d, want 0 (Private)", mo.shareBtn)
	}
	if mo.exposeID != "p1" {
		t.Fatalf("exposeID = %q, want p1", mo.exposeID)
	}
}

// TestExposePortsDirect guards that x goes straight to the port-expose input
// (the old web-terminal/port chooser is gone; w handles terminals now).
func TestExposePortsDirect(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	m.plays = []*api.Play{playWithState("p1", api.StateRunning)}
	m.refreshRows()

	out, _ := m.handlePlaysKey(runeKey("x"))
	mo := out.(model)
	if mo.modal != modalExposePort {
		t.Fatalf("x: modal = %v, want modalExposePort", mo.modal)
	}
	if mo.exposeID != "p1" {
		t.Fatalf("exposeID = %q, want p1", mo.exposeID)
	}
}

// TestCtrlDDestroys guards that Ctrl+D opens the destroy confirmation on the
// Playgrounds tab.
func TestCtrlDDestroys(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	m.plays = []*api.Play{playWithState("p1", api.StateRunning)}
	m.refreshRows()

	out, _ := m.handlePlaysKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	if mo := out.(model); mo.modal != modalConfirm || mo.confirm.id != "p1" {
		t.Fatalf("ctrl+d: modal=%v confirm=%q, want modalConfirm/p1", mo.modal, mo.confirm.id)
	}
}

// TestUnexposeOnExportsTab guards that Ctrl+D unexposes the selected export.
func TestUnexposeOnExportsTab(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	m.tab = tabExports
	m.exports = []exportItem{{playID: "p1", playName: "pg", kind: "shell", url: "https://x", exposeID: "s1"}}
	m.refreshRows()

	out, cmd := m.handleExportsKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	if got := out.(model).status; got != "Unexposing..." {
		t.Fatalf("status = %q, want Unexposing...", got)
	}
	if cmd == nil {
		t.Fatal("unexpose should return a command")
	}
}

func TestParsePorts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want []int
		ok   bool
	}{
		{in: "8080", want: []int{8080}, ok: true},
		{in: "8080, 9090", want: []int{8080, 9090}, ok: true},
		{in: "80 443 8080", want: []int{80, 443, 8080}, ok: true},
		{in: "abc", ok: false},
		{in: "70000", ok: false},
		{in: "", ok: false},
	}
	for _, tt := range tests {
		got, ok := parsePorts(tt.in)
		if ok != tt.ok {
			t.Fatalf("parsePorts(%q) ok = %v, want %v", tt.in, ok, tt.ok)
		}
		if ok && len(got) != len(tt.want) {
			t.Fatalf("parsePorts(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// TestExposePortValidation guards the port parsing in the expose-port dialog.
func TestExposePortValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		input      string
		wantPrefix string
	}{
		{name: "valid", input: "8080", wantPrefix: "Exposing port(s)..."},
		{name: "multiple", input: "8080, 9090", wantPrefix: "Exposing port(s)..."},
		{name: "not a number", input: "abc", wantPrefix: errMark},
		{name: "zero", input: "0", wantPrefix: errMark},
		{name: "too large", input: "70000", wantPrefix: errMark},
		{name: "empty", input: "", wantPrefix: errMark},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := newModel(testCLI())
			m.modal = modalExposePort
			m.exposeID = "p1"
			m.input.SetValue(tt.input)

			out, _ := m.handleExposePortKey(tea.KeyMsg{Type: tea.KeyEnter})
			if got := out.(model).status; !strings.HasPrefix(got, tt.wantPrefix) {
				t.Fatalf("input %q: status = %q, want prefix %q", tt.input, got, tt.wantPrefix)
			}
		})
	}
}

// TestPersistNoOpWhenAlreadyPersistent guards that P is rejected for a lab that
// is already persistent (marked via persistedIDs).
func TestPersistNoOpWhenAlreadyPersistent(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	m.plays = []*api.Play{playWithState("p1", api.StateRunning)}
	m.persistedIDs = map[string]bool{"p1": true}
	m.refreshRows()

	out, _ := m.handlePlaysKey(runeKey("P"))
	if got := out.(model).status; got == "Persisting..." {
		t.Fatal("persist should be a no-op for an already-persistent lab")
	}
}

// TestPersistentMarker guards that persistent plays render with a * prefix in
// the merged Playgrounds list, and selectedPlay still maps correctly.
func TestPersistentMarker(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	m.plays = []*api.Play{
		playWithState("aaa", api.StateRunning),
		playWithState("bbb", api.StateStopped),
	}
	m.persistedIDs = map[string]bool{"bbb": true}
	m.refreshRows()

	rows := m.playsTable.Rows()
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if !strings.HasPrefix(rows[1][0], "* ") {
		t.Fatalf("persistent row name = %q, want a * prefix", rows[1][0])
	}
	if strings.HasPrefix(rows[0][0], "*") {
		t.Fatalf("non-persistent row name = %q, should have no * prefix", rows[0][0])
	}
}

func TestSwitchTabCycles(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	for _, want := range []viewTab{tabExports, tabCatalog, tabPlays} {
		m.switchTab(1)
		if m.tab != want {
			t.Fatalf("after +1, tab = %v, want %v", m.tab, want)
		}
	}
	m.switchTab(-1)
	if m.tab != tabCatalog {
		t.Fatalf("after -1, tab = %v, want tabCatalog", m.tab)
	}
}

// TestSigningInFlash guards that login replaces the stuck "Signing in..." with a
// "Signed in as <user>" confirmation (which then auto-clears via flash).
func TestSigningInFlash(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	m.status = "Signing in..."

	out, _ := m.Update(authMsg{ok: true, user: "usr_x"})
	got := out.(model).status
	if got == "Signing in..." {
		t.Fatal(`"Signing in..." should be replaced after auth resolves`)
	}
	if !strings.HasPrefix(got, okMark) || !strings.Contains(got, "usr_x") {
		t.Fatalf("status = %q, want a signed-in flash mentioning the user", got)
	}
}

func TestShortStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		play       *api.Play
		wantPrefix string
	}{
		{name: "no status", play: &api.Play{}, wantPrefix: "UNKNOWN"},
		{name: "stopped", play: playWithState("p", api.StateStopped), wantPrefix: "STOPPED"},
		{name: "running", play: playWithState("p", api.StateRunning), wantPrefix: "RUNNING"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := shortStatus(tt.play); !strings.HasPrefix(got, tt.wantPrefix) {
				t.Fatalf("shortStatus = %q, want prefix %q", got, tt.wantPrefix)
			}
		})
	}
}

// TestRegionPickerPreselects guards that `R` opens the region picker with the
// button preselected to the user's current region.
func TestRegionPickerPreselects(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	m.region = api.RegionAP
	out, _ := m.handleKey(runeKey("R"))
	mo := out.(model)
	if mo.modal != modalRegion {
		t.Fatalf("modal = %v, want modalRegion", mo.modal)
	}
	if got, want := mo.regionBtn, slices.Index(api.KnownRegions, api.RegionAP); got != want {
		t.Fatalf("regionBtn = %d, want %d", got, want)
	}
}

// TestRegionSetUpdatesModel guards that a successful region set updates the
// header value.
func TestRegionSetUpdatesModel(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	out, _ := m.Update(regionSetMsg{region: api.RegionEU})
	if got := out.(model).region; got != api.RegionEU {
		t.Fatalf("region = %q, want %q", got, api.RegionEU)
	}
}

// TestSpawnRegionPicker guards that pressing enter on a catalog playground opens
// the region picker in spawn mode, defaulted to the platform region.
func TestSpawnRegionPicker(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	m.catalog = []api.Playground{{Name: "docker"}}
	m.region = api.RegionAP
	m.refreshRows()
	m.tab = tabCatalog
	m.focusActiveTable()

	out, _ := m.handleCatalogKey(tea.KeyMsg{Type: tea.KeyEnter})
	mo := out.(model)
	if mo.modal != modalRegion || !mo.regionForSpawn {
		t.Fatalf("modal=%v regionForSpawn=%v, want modalRegion/true", mo.modal, mo.regionForSpawn)
	}
	if mo.spawnName != "docker" {
		t.Fatalf("spawnName = %q, want docker", mo.spawnName)
	}
	if want := slices.Index(api.KnownRegions, api.RegionAP); mo.regionBtn != want {
		t.Fatalf("regionBtn = %d, want %d (default to platform region)", mo.regionBtn, want)
	}
}

// TestSpawnRegionStarts guards that confirming the spawn-region picker starts the
// lab and returns to the Playgrounds tab.
func TestSpawnRegionStarts(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	m.regionForSpawn = true
	m.spawnName = "docker"
	m.modal = modalRegion
	m.regionBtn = 0

	out, cmd := m.handleRegionKey(tea.KeyMsg{Type: tea.KeyEnter})
	mo := out.(model)
	if mo.modal != modalNone || mo.regionForSpawn {
		t.Fatalf("modal=%v regionForSpawn=%v, want modalNone/false", mo.modal, mo.regionForSpawn)
	}
	if mo.tab != tabPlays {
		t.Fatalf("tab = %v, want tabPlays", mo.tab)
	}
	if !strings.HasPrefix(mo.status, "Starting docker") {
		t.Fatalf("status = %q, want Starting docker...", mo.status)
	}
	if cmd == nil {
		t.Fatal("spawn should return a start command")
	}
}

// TestPersistentDoubleConfirm guards that destroying a persistent lab needs two
// confirmations, while a normal lab needs one.
func TestPersistentDoubleConfirm(t *testing.T) {
	t.Parallel()
	enter := tea.KeyMsg{Type: tea.KeyEnter}

	// Persistent: first Destroy advances to stage 1 without destroying.
	m := newModel(testCLI())
	m.confirm = pending{id: "p1", name: "pg", persistent: true}
	m.confirmBtn = 1
	m.modal = modalConfirm
	out, cmd := m.handleConfirmKey(enter)
	mo := out.(model)
	if mo.modal != modalConfirm || mo.confirmStage != 1 {
		t.Fatalf("first confirm: modal=%v stage=%d, want modalConfirm/1", mo.modal, mo.confirmStage)
	}
	if cmd != nil {
		t.Fatal("first confirm on a persistent lab must not destroy yet")
	}
	// Second Destroy actually destroys.
	mo.confirmBtn = 1
	out2, cmd2 := mo.handleConfirmKey(enter)
	if out2.(model).modal != modalNone || cmd2 == nil {
		t.Fatal("second confirm should destroy and close")
	}

	// Non-persistent: a single Destroy is enough.
	n := newModel(testCLI())
	n.confirm = pending{id: "p2", name: "pg", persistent: false}
	n.confirmBtn = 1
	n.modal = modalConfirm
	out3, cmd3 := n.handleConfirmKey(enter)
	if out3.(model).modal != modalNone || cmd3 == nil {
		t.Fatal("non-persistent lab should destroy on the first confirm")
	}
}

func TestFmtDur(t *testing.T) {
	t.Parallel()
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{45 * time.Second, "45s"},
		{30 * time.Minute, "30m"},
		{60 * time.Minute, "1h"},
		{90 * time.Minute, "1h30m"},
	}
	for _, tt := range tests {
		if got := fmtDur(tt.d); got != tt.want {
			t.Fatalf("fmtDur(%s) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

// TestPlayAge guards that a running lab shows the remaining time (counting down,
// no slash) while a stopped one shows the elapsed time.
func TestPlayAge(t *testing.T) {
	t.Parallel()
	running := playWithState("p", api.StateRunning)
	running.CreatedAt = time.Now().Add(-10 * time.Minute).Format(time.RFC3339)
	running.ExpiresIn = int((50 * time.Minute) / time.Millisecond)
	gotRunning := playAge(running)
	if strings.Contains(gotRunning, "/") || gotRunning == "" {
		t.Fatalf("running playAge = %q, want remaining only (no slash)", gotRunning)
	}

	stopped := playWithState("p", api.StateStopped)
	stopped.CreatedAt = time.Now().Add(-10 * time.Minute).Format(time.RFC3339)
	gotStopped := playAge(stopped)
	if strings.Contains(gotStopped, "/") {
		t.Fatalf("stopped playAge = %q, want elapsed only", gotStopped)
	}
	// Remaining (~50m) must differ from elapsed (~10m).
	if gotRunning == gotStopped {
		t.Fatalf("running %q should differ from stopped %q", gotRunning, gotStopped)
	}
}

// TestSpawnRegionCached guards that the region chosen at spawn is remembered and
// shown in the REGION column (the API returns no per-play region).
func TestSpawnRegionCached(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())

	out, _ := m.Update(spawnedMsg{id: "p1", name: "docker", region: api.RegionAP})
	mo := out.(model)
	if got := mo.spawnRegions["p1"]; got != api.RegionAP {
		t.Fatalf("spawnRegions[p1] = %q, want %q", got, api.RegionAP)
	}

	mo.plays = []*api.Play{playWithState("p1", api.StateRunning)} // API play has no region
	mo.refreshRows()
	if got := mo.playsTable.Rows()[0][1]; got != strings.ToUpper(api.RegionAP) {
		t.Fatalf("REGION cell = %q, want %q (from spawn cache)", got, strings.ToUpper(api.RegionAP))
	}
}

// TestForwardStartValidation guards the port-forward prompt: empty is rejected,
// a real spec kicks off the forward.
func TestForwardStartValidation(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ in, want string }{
		{"8080", "Forwarding..."},
		{"3000:80", "Forwarding..."},
		{"  ", errMark},
		{"", errMark},
	} {
		m := newModel(testCLI())
		m.modal = modalForward
		m.fwdTarget = pending{id: "p1", name: "pg"}
		m.input.SetValue(tt.in)
		out, _ := m.handleForwardKey(tea.KeyMsg{Type: tea.KeyEnter})
		if got := out.(model).status; !strings.HasPrefix(got, tt.want) {
			t.Fatalf("in %q: status=%q want prefix %q", tt.in, got, tt.want)
		}
	}
}

// TestStopForwards guards that ctrl+f's helper cancels and removes only the
// selected lab's forwards.
func TestStopForwards(t *testing.T) {
	t.Parallel()
	canceled := map[string]int{}
	mk := func(id string) forward {
		return forward{playID: id, cancel: func() { canceled[id]++ }}
	}
	m := newModel(testCLI())
	m.forwards = []forward{mk("p1"), mk("p1"), mk("p2")}

	if n := m.stopForwards("p1"); n != 2 {
		t.Fatalf("stopForwards = %d, want 2", n)
	}
	if len(m.forwards) != 1 || m.forwards[0].playID != "p2" {
		t.Fatalf("remaining forwards = %+v, want only p2", m.forwards)
	}
	if canceled["p1"] != 2 {
		t.Fatalf("p1 cancel called %d times, want 2", canceled["p1"])
	}
}

// TestForwardColumn guards that forwarded ports render in the PF column.
func TestForwardColumn(t *testing.T) {
	t.Parallel()
	_, rows := filterPlays(
		[]*api.Play{playWithState("p1", api.StateRunning)},
		"", nil, nil, map[string][]string{"p1": {"8080<->9090"}},
	)
	if len(rows) != 1 || rows[0][4] != "8080<->9090" {
		t.Fatalf("PF cell = %q, want 8080<->9090", rows[0][4])
	}
}

// TestStopForwardConfirm guards that ctrl+f asks first and Cancel leaves the
// forward running.
func TestStopForwardConfirm(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	m.plays = []*api.Play{playWithState("p1", api.StateRunning)}
	m.forwards = []forward{{playID: "p1", lport: "8080", cancel: func() {}}}
	m.refreshRows()

	out, _ := m.handlePlaysKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	mo := out.(model)
	if mo.modal != modalStopForward || mo.fwdTarget.id != "p1" {
		t.Fatalf("ctrl+f: modal=%v target=%q, want modalStopForward/p1", mo.modal, mo.fwdTarget.id)
	}
	// Cancel (button 0) keeps the forward.
	mo.confirmBtn = 0
	out2, _ := mo.handleStopForwardKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := out2.(model)
	if m2.modal != modalNone || len(m2.forwards) != 1 {
		t.Fatalf("cancel: modal=%v forwards=%d, want modalNone/1", m2.modal, len(m2.forwards))
	}
}

// TestForwardDuplicatePort guards that forwarding an already-bound local port is
// rejected before any network call.
func TestForwardDuplicatePort(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	m.forwards = []forward{{playID: "p1", lport: "1337", rport: "80"}}

	msg := m.startForward("p2", "pg", "1337:80")()
	e, ok := msg.(errMsg)
	if !ok || !strings.Contains(e.err.Error(), "already forwarded") {
		t.Fatalf("msg = %#v, want errMsg about port already forwarded", msg)
	}
}
