package tui

import (
	"bytes"
	"io"
	"strings"
	"testing"

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
		{"a", "", "", ""},
		{"b", "", "", ""},
		{"c", "", "", ""},
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

// TestExportDialogRoutes guards that x opens the Export dialog and routes to the
// share / expose-port sub-flows.
func TestExportDialogRoutes(t *testing.T) {
	t.Parallel()
	open := func() model {
		m := newModel(testCLI())
		m.plays = []*api.Play{playWithState("p1", api.StateRunning)}
		m.refreshRows()
		out, _ := m.handlePlaysKey(runeKey("x"))
		return out.(model)
	}
	if m := open(); m.modal != modalExport {
		t.Fatalf("x: modal = %v, want modalExport", m.modal)
	}
	// Web terminal (button 0) -> share access choice.
	web, _ := open().handleExportKey(tea.KeyMsg{Type: tea.KeyEnter})
	if web.(model).modal != modalShare {
		t.Fatalf("Export>Web: modal = %v, want modalShare", web.(model).modal)
	}
	// Port (button 1) -> port input.
	m := open()
	m.exportBtn = 1
	port, _ := m.handleExportKey(tea.KeyMsg{Type: tea.KeyEnter})
	if port.(model).modal != modalExposePort {
		t.Fatalf("Export>Port: modal = %v, want modalExposePort", port.(model).modal)
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

// TestPersistOnlyOnPlaygrounds guards that P is a no-op on the Persisted tab
// (persisted labs are already persistent).
func TestPersistOnlyOnPlaygrounds(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	m.persisted = []*api.Play{playWithState("p1", api.StateRunning)}
	m.refreshRows()
	m.tab = tabPersisted

	out, _ := m.handlePlaysKey(runeKey("P"))
	if got := out.(model).status; got == "Persisting..." {
		t.Fatal("persist should be a no-op on the Persisted tab")
	}
}

// TestPersistedTabSelection guards that on the Persisted tab, selectedPlay
// indexes the persisted slice (not the plays slice).
func TestPersistedTabSelection(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	m.plays = []*api.Play{playWithState("aaa", api.StateRunning)}
	m.persisted = []*api.Play{playWithState("bbb", api.StateStopped)}
	m.refreshRows()

	m.switchTab(1) // playgrounds -> persisted
	if m.tab != tabPersisted {
		t.Fatalf("tab = %v, want tabPersisted", m.tab)
	}
	if p := m.selectedPlay(); p == nil || p.ID != "bbb" {
		t.Fatalf("selectedPlay = %v, want persisted play bbb", p)
	}
}

func TestSwitchTabCycles(t *testing.T) {
	t.Parallel()
	m := newModel(testCLI())
	for _, want := range []viewTab{tabPersisted, tabExports, tabCatalog, tabPlays} {
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
