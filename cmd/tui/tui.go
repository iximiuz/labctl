package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/browser"
	"github.com/iximiuz/labctl/internal/config"
	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/portforward"
)

func NewCommand(cli labcli.CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Interactive terminal UI to manage and launch playgrounds",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(cli)
		},
	}
}

// Run launches the interactive TUI and blocks until the user exits.
func Run(cli labcli.CLI) error {
	skin, name := loadSkin()
	// ~/.labctl.config theme choice wins over skin.yaml's theme.
	if c := loadUIConfig(); c.Theme != "" {
		if s, ok := presets[c.Theme]; ok {
			skin, name = s, c.Theme
		}
	}
	initStyles(skin)
	m := newModel(cli)
	m.theme = name
	m.themeIdx = themeIndex(name)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return labcli.WrapStatusError(err)
}

type viewTab int

const (
	tabCatalog viewTab = iota
	tabPlays
	tabExports
)

const tabCount = 3

// exportItem is one exposed shell or port across all labs (Exports tab).
type exportItem struct {
	playID   string
	playName string
	kind     string // "shell" or the port number as text
	url      string
	exposeID string // shell/port id, for unexpose
	isPort   bool
}

// forward is a live local background port-forward (labctl side -> playground).
type forward struct {
	playID, playName string
	spec             string // the user's original spec, for restore
	lport, rport     string // for inline display
	cancel           context.CancelFunc
}

type (
	playsMsg struct {
		plays        []*api.Play
		persistedIDs map[string]bool
	}
	catalogMsg struct{ items []api.Playground }
	actionMsg  struct{ info string }
	errMsg     struct{ err error }
	tickMsg    struct{}
	authMsg    struct {
		ok     bool
		user   string
		region string
	}
	regionSetMsg      struct{ region string }
	spawnedMsg        struct{ id, name, region string }
	forwardStartedMsg struct {
		playID, playName, spec, lport, rport string
		cancel                               context.CancelFunc
	}
	loginDoneMsg   struct{ err error }
	disarmQuitMsg  struct{}
	statusClearMsg struct{ seq int }
	exposedMsg     struct{ kind, url string } // a shell/port was just exposed
	exposedListMsg struct {                   // exposed endpoints for the Info popup
		shells []*api.Shell
		ports  []*api.Port
	}
	exportsTabMsg struct{ items []exportItem } // aggregated exports for the Exports tab
)

const statusFlashDuration = 4 * time.Second

// flash sets a transient status message that auto-clears after a few seconds.
func (m *model) flash(msg string) tea.Cmd {
	m.status = msg
	m.statusSeq++
	seq := m.statusSeq
	return tea.Tick(statusFlashDuration, func(time.Time) tea.Msg { return statusClearMsg{seq} })
}

const refreshInterval = 5 * time.Second

func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

type pending struct {
	id, name   string
	persistent bool
}

// modal is the single active overlay/prompt. Exactly one is active at any time,
// which is what makes this an enum instead of a pile of independent booleans.
type modal uint8

const (
	modalNone        modal = iota
	modalFilter            // footer search bar (renders within mainView)
	modalExtend            // lifetime dialog
	modalShare             // share-terminal access choice (private/public)
	modalExposePort        // expose-port number input
	modalAuth              // sign-in popup
	modalConfirm           // destroy confirmation
	modalQuit              // quit confirmation
	modalInfo              // details popup
	modalThemes            // theme picker (live preview)
	modalHelp              // shortcuts popup
	modalRegion            // preferred-region picker
	modalForward           // start-port-forward input
	modalStopForward       // confirm stopping a lab's port-forwards
)

type model struct {
	cli labcli.CLI

	tab          viewTab
	playsTable   table.Model
	catalogTable table.Model
	exportsTable table.Model

	plays         []*api.Play       // full, unfiltered (incl. persistent)
	persistedIDs  map[string]bool   // which plays are persistent (* marker)
	spawnRegions  map[string]string // playID -> region chosen at spawn (column fallback)
	forwards      []forward         // live local background port-forwards
	restore       []savedForward    // forwards to re-establish once plays load
	restored      bool              // restore attempted
	catalog       []api.Playground  // full, unfiltered
	exports       []exportItem      // aggregated exposed shells/ports
	filteredPlays []*api.Play       // rows currently shown (cursor indexes this)
	filteredCat   []api.Playground
	filteredExp   []exportItem

	modal modal // the single active overlay/prompt

	filter    string
	input     textinput.Model // filter + extend lifetime + expose-port field
	extendBtn int             // 0 = field, 1 = Cancel, 2 = OK
	extendID  string          // play being extended

	exposeID       string       // play being shared / port-exposed
	fwdTarget      pending      // lab awaiting a port-forward spec (modalForward)
	shareBtn       int          // 0 = Private, 1 = Public
	regionBtn      int          // index into api.KnownRegions (modalRegion)
	regionForSpawn bool         // modalRegion is choosing a spawn region, not the default
	spawnName      string       // catalog playground awaiting a region choice
	infoShells     []*api.Shell // exposed shells (loaded for the Info popup)
	infoPorts      []*api.Port  // exposed ports (loaded for the Info popup)

	status        string
	statusSeq     int     // bumped per flash; stale auto-clears are ignored
	confirm       pending // destroy target (valid while modal == modalConfirm)
	confirmBtn    int     // 0 = Cancel, 1 = Destroy
	confirmStage  int     // persistent destroy needs two confirmations (0 then 1)
	authBtn       int     // 0 = Dismiss, 1 = Login via browser
	authDismissed bool    // auth popup already dismissed this session
	quitBtn       int     // 0 = Cancel, 1 = Quit
	themePrevIdx  int     // theme to restore if the picker is cancelled
	quitArmed     bool    // first ctrl+c/ctrl+d seen; a second one quits
	theme         string  // active color theme name
	themeIdx      int     // index into themeOrder for the T-key cycler
	user          string  // logged-in user id (from GetMe)
	region        string  // preferred region for new playgrounds (from GetMe)
	defaulted     bool    // initial view default (catalog-if-empty) applied
	width, height int
}

func newModel(cli labcli.CLI) model {
	pt := table.New(table.WithFocused(true))
	ct := table.New()
	ex := table.New()
	applySkin(&pt)
	applySkin(&ct)
	applySkin(&ex)
	ti := textinput.New()
	ti.Prompt = ""
	st := loadUIState()
	m := model{cli: cli, tab: tabPlays, playsTable: pt, catalogTable: ct, exportsTable: ex, input: ti, status: "Loading...", theme: "k9s", spawnRegions: st.Regions, restore: st.Forwards}
	m.setSizes(80, 24)
	return m
}

// persistState writes the per-lab region cache and live forwards to ~/.labctl.rc.
func (m model) persistState() {
	st := uiState{Regions: m.spawnRegions}
	for _, f := range m.forwards {
		st.Forwards = append(st.Forwards, savedForward{PlayID: f.playID, PlayName: f.playName, Spec: f.spec})
	}
	st.save()
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.loadPlays(), m.loadCatalog(), m.checkAuth(), tick())
}

func (m model) checkAuth() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		me, err := m.cli.Client().GetMe(ctx)
		if err != nil {
			return authMsg{ok: false}
		}
		return authMsg{ok: true, user: me.ID, region: me.PreferredRegion}
	}
}

// loginCmd hands the terminal to `labctl auth login` (browser flow), then
// resumes the TUI.
func (m model) loginCmd() tea.Cmd {
	c := exec.Command(os.Args[0], "auth", "login")
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return loginDoneMsg{err}
	})
}

// ponytail: bubbletea cmds run off the UI goroutine; each opens its own context.
func (m model) loadPlays() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		recent, err := m.cli.Client().ListPlays(ctx, api.ListPlaysQueryParams{})
		if err != nil {
			return errMsg{err}
		}
		persistent, err := m.cli.Client().ListPlays(ctx, api.ListPlaysQueryParams{Persistent: true})
		if err != nil {
			return errMsg{err}
		}
		gone := func(p *api.Play) bool { return !p.IsActive() && !p.StateIs(api.StateStopped) }
		byUpdated := func(a, b *api.Play) int { return strings.Compare(b.UpdatedAt, a.UpdatedAt) }

		isPersistent := make(map[string]bool, len(persistent))
		for _, p := range persistent {
			isPersistent[p.ID] = true
		}

		// One unified list of active/stopped plays; persistent ones are kept and
		// flagged (rendered with a * marker) rather than split into their own tab.
		plays := slices.DeleteFunc(append([]*api.Play{}, recent...), gone)
		slices.SortFunc(plays, byUpdated)

		return playsMsg{plays: plays, persistedIDs: isPersistent}
	}
}

func (m model) loadCatalog() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		items, err := m.cli.Client().ListPlaygrounds(ctx, &api.ListPlaygroundsOptions{})
		if err != nil {
			return errMsg{err}
		}
		slices.SortFunc(items, func(a, b api.Playground) int { return strings.Compare(a.Name, b.Name) })
		return catalogMsg{items}
	}
}

func (m model) stopPlay(id string) tea.Cmd {
	return m.playAction("Stopped "+id, func(ctx context.Context) error {
		_, err := m.cli.Client().StopPlay(ctx, id)
		return err
	})
}

func (m model) restartPlay(id string) tea.Cmd {
	return m.playAction("Restarted "+id, func(ctx context.Context) error {
		_, err := m.cli.Client().RestartPlay(ctx, id)
		return err
	})
}

func (m model) destroyPlay(id string) tea.Cmd {
	return m.playAction("Destroyed "+id, func(ctx context.Context) error {
		return m.cli.Client().DestroyPlay(ctx, id)
	})
}

func (m model) persistPlay(id string) tea.Cmd {
	return m.playAction("Persisted "+id, func(ctx context.Context) error {
		return m.cli.Client().PersistPlay(ctx, id)
	})
}

func (m model) setRegion(region string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		me, err := m.cli.Client().SetPreferredRegion(ctx, region)
		if err != nil {
			return errMsg{err}
		}
		return regionSetMsg{region: me.PreferredRegion}
	}
}

func (m model) extendPlay(id string, minutes int) tea.Cmd {
	return m.playAction(fmt.Sprintf("Lifetime of %s set to %dm", id, minutes), func(ctx context.Context) error {
		_, err := m.cli.Client().SetPlayMaxPlayTime(ctx, id, minutes)
		return err
	})
}

func access(public bool) api.AccessMode {
	if public {
		return api.AccessPublic
	}
	return api.AccessPrivate
}

// shareTerminal exposes a web terminal for the playground and returns its URL.
func (m model) shareTerminal(id string, public bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		p, err := m.cli.Client().GetPlay(ctx, id)
		if err != nil {
			return errMsg{err}
		}
		machine, err := p.ResolveMachine("")
		if err != nil {
			return errMsg{err}
		}
		user, err := p.ResolveUser(machine, "")
		if err != nil {
			return errMsg{err}
		}
		sh, err := m.cli.Client().ExposeShell(ctx, id, api.ExposeShellRequest{
			Machine: machine, User: user, Access: access(public),
		})
		if err != nil {
			return errMsg{err}
		}
		return exposedMsg{kind: "Terminal", url: sh.URL}
	}
}

// exposePort exposes an HTTP service running in the playground and returns its
// public URL.
// exposePort exposes one or more HTTP ports (the API takes one per call, so we
// fan out) and returns the joined URLs.
func (m model) exposePort(id string, ports []int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		p, err := m.cli.Client().GetPlay(ctx, id)
		if err != nil {
			return errMsg{err}
		}
		machine, err := p.ResolveMachine("")
		if err != nil {
			return errMsg{err}
		}
		var urls []string
		for _, port := range ports {
			pt, err := m.cli.Client().ExposePort(ctx, id, api.ExposePortRequest{
				Machine: machine, Number: port, Access: api.AccessPublic,
			})
			if err != nil {
				return errMsg{err}
			}
			urls = append(urls, pt.URL)
		}
		kind := "Port"
		if len(urls) > 1 {
			kind = fmt.Sprintf("%d ports", len(urls))
		}
		return exposedMsg{kind: kind, url: strings.Join(urls, "  ")}
	}
}

// loadExposed fetches the currently exposed shells/ports for the Info popup.
func (m model) loadExposed(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		shells, _ := m.cli.Client().ListShells(ctx, id)
		ports, _ := m.cli.Client().ListPorts(ctx, id)
		return exposedListMsg{shells: shells, ports: ports}
	}
}

// loadExports aggregates exposed shells/ports across all active labs for the
// Exports tab.
func (m model) loadExports() tea.Cmd {
	plays := append([]*api.Play{}, m.plays...)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var items []exportItem
		for _, p := range plays {
			if !p.IsActive() {
				continue
			}
			for _, sh := range mustShells(m.cli.Client().ListShells(ctx, p.ID)) {
				items = append(items, exportItem{
					playID: p.ID, playName: p.Playground.Name,
					kind: "shell", url: sh.URL, exposeID: sh.ID,
				})
			}
			for _, pt := range mustPorts(m.cli.Client().ListPorts(ctx, p.ID)) {
				items = append(items, exportItem{
					playID: p.ID, playName: p.Playground.Name,
					kind: strconv.Itoa(pt.Number), url: pt.URL, exposeID: pt.ID, isPort: true,
				})
			}
		}
		return exportsTabMsg{items: items}
	}
}

func mustShells(s []*api.Shell, _ error) []*api.Shell { return s }
func mustPorts(p []*api.Port, _ error) []*api.Port    { return p }

// unexpose removes one exposed shell or port.
func (m model) unexpose(e exportItem) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		var err error
		if e.isPort {
			err = m.cli.Client().UnexposePort(ctx, e.playID, e.exposeID)
		} else {
			err = m.cli.Client().UnexposeShell(ctx, e.playID, e.exposeID)
		}
		if err != nil {
			return errMsg{err}
		}
		return actionMsg{"Unexposed " + e.kind}
	}
}

func (m model) startPlay(name, region string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		// Region is an account-level preference ("region for new playgrounds"),
		// not a per-create field — set it first so the lab spawns in that region.
		if region != "" {
			if _, err := m.cli.Client().SetPreferredRegion(ctx, region); err != nil {
				return errMsg{err}
			}
		}
		// ponytail: official catalog playgrounds need no safety consent; auto-ack.
		p, err := m.cli.Client().CreatePlay(ctx, api.CreatePlayRequest{
			Playground:              name,
			SafetyDisclaimerConsent: true,
		})
		if err != nil {
			return errMsg{err}
		}
		return spawnedMsg{id: p.ID, name: name, region: region}
	}
}

// startForward establishes a local background port-forward to the lab. The
// tunnel lives under its own context so it keeps running after this cmd returns;
// the returned cancel func (in forwardStartedMsg) stops it.
func (m model) startForward(playID, playName, spec string) tea.Cmd {
	return func() tea.Msg {
		fs, err := portforward.ParseLocal(spec)
		if err != nil {
			return errMsg{fmt.Errorf("invalid port spec %q", spec)}
		}
		// A local port can only be bound once — reject a duplicate up front
		// instead of silently failing to bind.
		for _, f := range m.forwards {
			if f.lport == fs.LocalPort {
				return errMsg{fmt.Errorf("local port %s is already forwarded", fs.LocalPort)}
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		setup, setupCancel := context.WithTimeout(ctx, 30*time.Second)
		p, err := m.cli.Client().GetPlay(setup, playID)
		if err != nil {
			setupCancel()
			cancel()
			return errMsg{err}
		}
		machine, err := p.ResolveMachine("")
		if err != nil {
			setupCancel()
			cancel()
			return errMsg{err}
		}
		tunnel, err := portforward.StartTunnel(ctx, m.cli.Client(), portforward.TunnelOptions{
			PlayID: playID, Machine: machine,
		})
		setupCancel()
		if err != nil {
			cancel()
			return errMsg{err}
		}
		tunnel.StartForwarding(ctx, fs) // runs in the background under ctx
		return forwardStartedMsg{
			playID: playID, playName: playName, spec: spec,
			lport: fs.LocalPort, rport: fs.RemotePort, cancel: cancel,
		}
	}
}

// stopForwards cancels and removes every forward on the given lab, returning the
// count stopped.
func (m *model) stopForwards(playID string) int {
	kept := m.forwards[:0]
	n := 0
	for _, f := range m.forwards {
		if f.playID == playID {
			if f.cancel != nil {
				f.cancel()
			}
			n++
			continue
		}
		kept = append(kept, f)
	}
	m.forwards = kept
	return n
}

// fwdPortsByPlay maps a lab id to its "local<->remote" forwards (PF column).
func (m model) fwdPortsByPlay() map[string][]string {
	out := make(map[string][]string, len(m.forwards))
	for _, f := range m.forwards {
		out[f.playID] = append(out[f.playID], f.lport+"<->"+f.rport)
	}
	return out
}

func (m model) playAction(info string, fn func(context.Context) error) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := fn(ctx); err != nil {
			return errMsg{err}
		}
		return actionMsg{info}
	}
}

// selectedPlay returns the highlighted play on the Playgrounds tab.
func (m *model) selectedPlay() *api.Play {
	if m.tab != tabPlays {
		return nil
	}
	if i := m.playsTable.Cursor(); i >= 0 && i < len(m.filteredPlays) {
		return m.filteredPlays[i]
	}
	return nil
}

func (m *model) selectedPlayground() *api.Playground {
	i := m.catalogTable.Cursor()
	if i < 0 || i >= len(m.filteredCat) {
		return nil
	}
	return &m.filteredCat[i]
}

// filterPlays returns the subset of plays matching f, along with their rows,
// keeping the cursor->slice mapping in sync. Persistent plays get a * marker.
func filterPlays(plays []*api.Play, f string, persistedIDs map[string]bool, spawnRegions map[string]string, fwdPorts map[string][]string) ([]*api.Play, []table.Row) {
	out := make([]*api.Play, 0, len(plays))
	rows := make([]table.Row, 0, len(plays))
	for _, p := range plays {
		name := p.Playground.Name
		if persistedIDs[p.ID] {
			name = "* " + name
		}
		fwd := strings.Join(fwdPorts[p.ID], ",")
		// Prefer the region the API returns; fall back to the one chosen at spawn.
		region := p.Region
		if region == "" {
			region = spawnRegions[p.ID]
		}
		if region == "" {
			region = "-"
		} else {
			region = strings.ToUpper(region)
		}
		row := table.Row{name, region, shortStatus(p), playAge(p), fwd}
		if f == "" || strings.Contains(strings.ToLower(strings.Join(row, " ")), f) {
			out = append(out, p)
			rows = append(rows, row)
		}
	}
	return out, rows
}

// refreshRows recomputes the filtered slices and table rows from the full lists,
// applying the current filter.
func (m *model) refreshRows() {
	f := strings.ToLower(strings.TrimSpace(m.filter))

	var pr []table.Row
	m.filteredPlays, pr = filterPlays(m.plays, f, m.persistedIDs, m.spawnRegions, m.fwdPortsByPlay())
	m.playsTable.SetRows(pr)

	m.filteredCat = m.filteredCat[:0]
	cr := make([]table.Row, 0, len(m.catalog))
	for _, p := range m.catalog {
		row := table.Row{p.Name, p.Description}
		if f == "" || strings.Contains(strings.ToLower(strings.Join(row, " ")), f) {
			m.filteredCat = append(m.filteredCat, p)
			cr = append(cr, row)
		}
	}
	m.catalogTable.SetRows(cr)

	m.filteredExp = m.filteredExp[:0]
	er := make([]table.Row, 0, len(m.exports))
	for _, e := range m.exports {
		row := table.Row{e.playName, e.kind, e.url}
		if f == "" || strings.Contains(strings.ToLower(strings.Join(row, " ")), f) {
			m.filteredExp = append(m.filteredExp, e)
			er = append(er, row)
		}
	}
	m.exportsTable.SetRows(er)
}

func (m *model) selectedExport() *exportItem {
	i := m.exportsTable.Cursor()
	if i < 0 || i >= len(m.filteredExp) {
		return nil
	}
	return &m.filteredExp[i]
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.setSizes(msg.Width, msg.Height)
		return m, nil

	case playsMsg:
		m.plays = msg.plays
		m.persistedIDs = msg.persistedIDs
		m.refreshRows()
		// Clear the transient progress statuses on a healthy poll (action results
		// and ✗ errors clear themselves via flash).
		if m.status == "Loading..." || m.status == "Refreshing..." {
			m.status = ""
		}
		// Default to the Catalog view once, if the user has no playgrounds.
		if !m.defaulted {
			m.defaulted = true
			if len(m.plays) == 0 {
				m.tab = tabCatalog
				m.focusActiveTable()
			}
		}
		// Re-establish saved forwards once, for labs that are running again.
		if !m.restored {
			m.restored = true
			running := make(map[string]bool, len(m.plays))
			for _, p := range m.plays {
				if p.StateIs(api.StateRunning) {
					running[p.ID] = true
				}
			}
			var cmds []tea.Cmd
			for _, sf := range m.restore {
				if running[sf.PlayID] {
					cmds = append(cmds, m.startForward(sf.PlayID, sf.PlayName, sf.Spec))
				}
			}
			m.restore = nil
			if len(cmds) > 0 {
				return m, tea.Batch(cmds...)
			}
		}
		return m, nil

	case catalogMsg:
		m.catalog = msg.items
		m.refreshRows()
		return m, nil

	case authMsg:
		fromLogin := m.status == "Signing in..."
		if fromLogin {
			m.status = ""
		}
		if msg.ok {
			m.user = msg.user
			m.region = msg.region
			m.authDismissed = false
			cmds := []tea.Cmd{m.loadPlays(), m.loadCatalog()}
			if fromLogin { // confirm the sign-in, then auto-clear
				note := okMark + " Signed in"
				if msg.user != "" {
					note += " as " + msg.user
				}
				cmds = append(cmds, m.flash(note))
			}
			return m, tea.Batch(cmds...)
		}
		if !m.authDismissed {
			m.modal = modalAuth
			m.authBtn = 1 // default-highlight Login
		}
		return m, nil

	case loginDoneMsg:
		if msg.err != nil {
			m.status = errMark + " Login failed: " + msg.err.Error()
			return m, nil
		}
		// Reload credentials on the UI goroutine (the login subprocess wrote them
		// to disk) so we don't mutate the shared client from a background cmd.
		if home, err := os.UserHomeDir(); err == nil {
			if cfg, err := config.Load(home); err == nil {
				m.cli.Config().SessionID = cfg.SessionID
				m.cli.Config().AccessToken = cfg.AccessToken
				m.cli.Client().SetCredentials(cfg.SessionID, cfg.AccessToken)
			}
		}
		m.status = "Signing in..."
		return m, m.checkAuth()

	case disarmQuitMsg:
		m.quitArmed = false
		if m.status == quitHint {
			m.status = ""
		}
		return m, nil

	case statusClearMsg:
		if msg.seq == m.statusSeq {
			m.status = ""
		}
		return m, nil

	case tickMsg:
		return m, tea.Batch(m.loadPlays(), tick())

	case actionMsg:
		cmds := []tea.Cmd{m.loadPlays(), m.flash(okMark + " " + msg.info)}
		if m.tab == tabExports { // keep the exports list fresh after unexpose etc.
			cmds = append(cmds, m.loadExports())
		}
		return m, tea.Batch(cmds...)

	case errMsg:
		clear := m.flash(errMark + " " + msg.err.Error())
		return m, clear

	case exposedMsg:
		note := okMark + " " + msg.kind + " exposed: " + msg.url
		if err := clipboard.WriteAll(msg.url); err == nil {
			note += " (copied)"
		}
		return m, m.flash(note)

	case exposedListMsg:
		m.infoShells = msg.shells
		m.infoPorts = msg.ports
		return m, nil

	case exportsTabMsg:
		m.exports = msg.items
		m.refreshRows()
		return m, nil

	case regionSetMsg:
		m.region = msg.region
		return m, m.flash(okMark + " Region set to " + strings.ToUpper(msg.region))

	case spawnedMsg:
		// Remember the chosen region (the API doesn't return it per play) so the
		// REGION column can show it for labs spawned in this session. Spawning
		// also updates the account default, so reflect it in the header.
		if msg.region != "" {
			m.spawnRegions[msg.id] = msg.region
			m.region = msg.region
			m.persistState()
		}
		return m, tea.Batch(m.loadPlays(), m.flash(okMark+" Started "+msg.name))

	case forwardStartedMsg:
		m.forwards = append(m.forwards, forward{
			playID: msg.playID, playName: msg.playName, spec: msg.spec,
			lport: msg.lport, rport: msg.rport, cancel: msg.cancel,
		})
		m.persistState()
		m.refreshRows()
		return m, m.flash(okMark + " Forwarding " + msg.lport + " → " + msg.rport)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	cmd := m.delegate(msg)
	return m, cmd
}

const (
	quitHint = "Press ctrl+c again to exit"
	okMark   = "✓"
	errMark  = "✗"
)

// Fixed (theme-independent) status icon colors.
var (
	okStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#3FB950")).Bold(true)
	errStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F85149")).Bold(true)
)

// renderStatus colors the leading ✓/✗ icon green/red, keeping the rest themed.
func renderStatus(s string) string {
	switch {
	case strings.HasPrefix(s, okMark):
		return okStyle.Render(okMark) + statusStyle.Render(strings.TrimPrefix(s, okMark))
	case strings.HasPrefix(s, errMark):
		return errStyle.Render(errMark) + statusStyle.Render(strings.TrimPrefix(s, errMark))
	default:
		return statusStyle.Render(s)
	}
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Double ctrl+c to quit (Claude-CLI style): first press arms and hints, a
	// second within the window quits. (ctrl+d is the delete key, see below.)
	if msg.String() == "ctrl+c" {
		if m.quitArmed {
			return m, tea.Quit
		}
		m.quitArmed = true
		m.status = quitHint
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return disarmQuitMsg{} })
	}
	m.quitArmed = false // any other key disarms

	// An active modal captures all input until dismissed.
	switch m.modal {
	case modalThemes:
		return m.handleThemesKey(msg)
	case modalExtend:
		return m.handleExtendSelectKey(msg)
	case modalShare:
		return m.handleShareKey(msg)
	case modalExposePort:
		return m.handleExposePortKey(msg)
	case modalFilter:
		return m.handlePromptKey(msg)
	case modalQuit:
		return m.handleQuitKey(msg)
	case modalInfo:
		return m.handleInfoKey(msg)
	case modalAuth:
		return m.handleAuthKey(msg)
	case modalConfirm:
		return m.handleConfirmKey(msg)
	case modalRegion:
		return m.handleRegionKey(msg)
	case modalForward:
		return m.handleForwardKey(msg)
	case modalStopForward:
		return m.handleStopForwardKey(msg)
	case modalHelp: // any key closes it
		m.modal = modalNone
		return m, nil
	}

	switch msg.String() {
	case "q":
		m.modal = modalQuit
		m.quitBtn = 0
		return m, nil

	case ":", "/": // open the filter/search prompt (k9s-style)
		m.modal = modalFilter
		m.input.SetValue(m.filter)
		m.input.Placeholder = "filter..."
		m.input.CursorEnd()
		return m, m.input.Focus()

	case "tab", "right", "l":
		m.switchTab(1)
		return m, m.tabEnterCmd()

	case "shift+tab", "left", "h":
		m.switchTab(-1)
		return m, m.tabEnterCmd()

	case "r":
		m.status = "Refreshing..."
		cmds := []tea.Cmd{m.loadPlays(), m.loadCatalog()}
		if m.tab == tabExports {
			cmds = append(cmds, m.loadExports())
		}
		return m, tea.Batch(cmds...)

	case "?": // shortcuts popup
		m.modal = modalHelp
		return m, nil

	case "T": // open the theme picker (live preview)
		m.modal = modalThemes
		m.themePrevIdx = m.themeIdx
		return m, nil

	case "R": // set the default preferred region for new playgrounds
		m.modal = modalRegion
		m.regionForSpawn = false
		m.regionBtn = slices.Index(api.KnownRegions, m.region)
		if m.regionBtn < 0 {
			m.regionBtn = 0
		}
		return m, nil

	case "i": // show full details of the selected row
		m.infoShells, m.infoPorts = nil, nil
		if m.tab == tabCatalog {
			if m.selectedPlayground() != nil {
				m.modal = modalInfo
			}
			return m, nil
		}
		if p := m.selectedPlay(); p != nil {
			m.modal = modalInfo
			return m, m.loadExposed(p.ID) // populate the exposed-endpoints section
		}
		return m, nil
	}

	switch m.tab {
	case tabCatalog:
		return m.handleCatalogKey(msg)
	case tabExports:
		return m.handleExportsKey(msg)
	default:
		return m.handlePlaysKey(msg) // tabPlays
	}
}

// switchTab moves to the next/previous tab and focuses its table.
func (m *model) switchTab(delta int) {
	m.tab = viewTab((int(m.tab) + delta + tabCount) % tabCount)
	m.focusActiveTable()
}

// tabEnterCmd loads data needed by the freshly-entered tab (exports are fetched
// lazily, not on the 5s poll).
func (m model) tabEnterCmd() tea.Cmd {
	if m.tab == tabExports {
		return m.loadExports()
	}
	return nil
}

func (m *model) focusActiveTable() {
	m.playsTable.Blur()
	m.catalogTable.Blur()
	m.exportsTable.Blur()
	switch m.tab {
	case tabCatalog:
		m.catalogTable.Focus()
	case tabExports:
		m.exportsTable.Focus()
	default:
		m.playsTable.Focus()
	}
}

// handleThemesKey drives the theme picker: arrows live-preview, enter keeps, esc
// reverts to the theme active when it was opened.
func (m model) handleThemesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k", "left", "h":
		m.themeIdx = (m.themeIdx - 1 + len(themeOrder)) % len(themeOrder)
		m.applyTheme(themeOrder[m.themeIdx])
		return m, nil
	case "down", "j", "right", "l", "tab":
		m.themeIdx = (m.themeIdx + 1) % len(themeOrder)
		m.applyTheme(themeOrder[m.themeIdx])
		return m, nil
	case "enter":
		m.modal = modalNone
		uiConfig{Theme: m.theme}.save()
		return m, m.flash("Theme: " + m.theme)
	case "esc":
		m.themeIdx = m.themePrevIdx
		m.applyTheme(themeOrder[m.themeIdx])
		m.modal = modalNone
		return m, nil
	default:
		return m, nil
	}
}

func (m model) handleQuitKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "down", "left", "right", "tab", "j", "k", "h", "l":
		m.quitBtn = 1 - m.quitBtn
		return m, nil
	case "esc":
		m.modal = modalNone
		m.quitBtn = 0
		return m, nil
	case "enter":
		if m.quitBtn == 1 {
			return m, tea.Quit
		}
		m.modal = modalNone
		m.quitBtn = 0
		return m, nil
	default:
		return m, nil
	}
}

// handleInfoKey closes the details popup on any key; 'o' opens the page first.
func (m model) handleInfoKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "o" {
		if url := m.selectedURL(); url != "" {
			_ = browser.Open(url)
		}
		return m, nil
	}
	m.modal = modalNone
	return m, nil
}

func (m model) handleAuthKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "down", "left", "right", "tab", "j", "k", "h", "l":
		m.authBtn = 1 - m.authBtn
		return m, nil
	case "esc":
		m.modal = modalNone
		m.authDismissed = true
		return m, nil
	case "enter":
		m.modal = modalNone
		if m.authBtn == 1 {
			m.status = "Opening browser for login..."
			return m, m.loginCmd()
		}
		m.authDismissed = true
		return m, nil
	default:
		return m, nil
	}
}

func (m model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "down", "left", "right", "tab", "j", "k", "h", "l":
		m.confirmBtn = 1 - m.confirmBtn
		return m, nil
	case "enter":
		if m.confirmBtn == 1 {
			// Persistent labs require a second confirmation before destroying.
			if m.confirm.persistent && m.confirmStage == 0 {
				m.confirmStage = 1
				m.confirmBtn = 0 // re-default to Cancel
				return m, nil
			}
			id := m.confirm.id
			m.modal = modalNone
			m.confirmBtn = 0
			m.confirmStage = 0
			m.status = "Destroying..."
			if m.stopForwards(id) > 0 { // drop now-dead forwards
				m.persistState()
			}
			return m, m.destroyPlay(id)
		}
		fallthrough
	case "esc":
		m.modal = modalNone
		m.confirmBtn = 0
		m.confirmStage = 0
		m.status = "Cancelled"
		return m, nil
	default:
		return m, nil
	}
}

func (m model) handleShareKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "down", "left", "right", "tab", "j", "k", "h", "l":
		m.shareBtn = 1 - m.shareBtn
		return m, nil
	case "esc":
		m.modal = modalNone
		return m, nil
	case "enter":
		m.modal = modalNone
		m.status = "Sharing terminal..."
		return m, m.shareTerminal(m.exposeID, m.shareBtn == 1)
	default:
		return m, nil
	}
}

// parsePorts parses a comma/space-separated list of valid ports.
func parsePorts(s string) ([]int, bool) {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' })
	ports := make([]int, 0, len(fields))
	for _, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 1 || n > 65535 {
			return nil, false
		}
		ports = append(ports, n)
	}
	return ports, len(ports) > 0
}

// handleExposePortKey drives the expose-port input (accepts a list).
func (m model) handleExposePortKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		m.input.Blur()
		return m, nil
	case "enter":
		ports, ok := parsePorts(m.input.Value())
		id := m.exposeID
		m.modal = modalNone
		m.input.Blur()
		if !ok {
			m.status = errMark + " Invalid port(s): " + m.input.Value()
			return m, nil
		}
		m.status = "Exposing port(s)..."
		return m, m.exposePort(id, ports)
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

// handleRegionKey drives the preferred-region picker (EU/AP). enter applies.
func (m model) handleRegionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "down", "left", "right", "tab", "j", "k", "h", "l":
		m.regionBtn = (m.regionBtn + 1) % len(api.KnownRegions)
		return m, nil
	case "esc":
		m.modal = modalNone
		m.regionForSpawn = false
		return m, nil
	case "enter":
		region := api.KnownRegions[m.regionBtn]
		m.modal = modalNone
		if m.regionForSpawn { // spawn the catalog playground in the chosen region
			name := m.spawnName
			m.regionForSpawn = false
			m.tab = tabPlays
			m.focusActiveTable()
			m.status = "Starting " + name + "..."
			return m, m.startPlay(name, region)
		}
		if region == m.region {
			return m, nil
		}
		m.status = "Setting region..."
		return m, m.setRegion(region)
	default:
		return m, nil
	}
}

// handleForwardKey drives the port-forward input: enter starts it, esc cancels.
func (m model) handleForwardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		m.input.Blur()
		return m, nil
	case "enter":
		spec := strings.TrimSpace(m.input.Value())
		tgt := m.fwdTarget
		m.modal = modalNone
		m.input.Blur()
		if spec == "" {
			m.status = errMark + " Port required"
			return m, nil
		}
		m.status = "Forwarding..."
		return m, m.startForward(tgt.id, tgt.name, spec)
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

// handleStopForwardKey confirms stopping a lab's port-forwards.
func (m model) handleStopForwardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "down", "left", "right", "tab", "j", "k", "h", "l":
		m.confirmBtn = 1 - m.confirmBtn
		return m, nil
	case "enter":
		stop := m.confirmBtn == 1
		id := m.fwdTarget.id
		m.modal = modalNone
		m.confirmBtn = 0
		if !stop {
			return m, nil
		}
		n := m.stopForwards(id)
		m.persistState()
		m.refreshRows()
		return m, m.flash(fmt.Sprintf("%s Stopped %d forward(s)", okMark, n))
	case "esc":
		m.modal = modalNone
		m.confirmBtn = 0
		return m, nil
	default:
		return m, nil
	}
}

// handlePromptKey drives the filter input (live-filters as you type).
func (m model) handlePromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filter = ""
		m.refreshRows()
		m.modal = modalNone
		m.input.Blur()
		return m, nil
	case "enter":
		m.filter = m.input.Value()
		m.refreshRows()
		m.modal = modalNone
		m.input.Blur()
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.filter = m.input.Value()
		m.refreshRows()
		return m, cmd
	}
}

func (m model) handlePlaysKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := m.selectedPlay()
	switch msg.String() {
	case "enter": // SSH by handing the terminal to `labctl ssh <id>`.
		if p == nil {
			return m, nil
		}
		// SSH only works on a RUNNING lab; every other state (stopped, stopping,
		// starting, warming up, …) can't accept a session.
		if !p.StateIs(api.StateRunning) {
			m.status = errMark + " Can't SSH into a " + shortStatus(p) + " lab — press ? for help"
			return m, nil
		}
		c := exec.Command(os.Args[0], "ssh", p.ID)
		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			if err != nil {
				return errMsg{err}
			}
			return actionMsg{"SSH session ended"}
		})
	case "o":
		if p == nil {
			return m, nil
		}
		if err := browser.Open(p.PageURL); err != nil {
			m.status = errMark + " Error opening browser: " + err.Error()
		}
		return m, nil
	case "s": // toggle: stop a running playground, start a stopped one
		if p == nil {
			return m, nil
		}
		// Either action drops the lab's tunnels; clear them so the PF column
		// doesn't show stale ports.
		if m.stopForwards(p.ID) > 0 {
			m.persistState()
			m.refreshRows()
		}
		if p.StateIs(api.StateStopped) {
			m.status = "Starting..."
			return m, m.restartPlay(p.ID)
		}
		m.status = "Stopping..."
		return m, m.stopPlay(p.ID)
	case "P": // make the playground persistent (active, non-persistent labs only)
		if p == nil || !p.IsActive() {
			m.status = errMark + " Select a running playground to persist"
			return m, nil
		}
		if m.persistedIDs[p.ID] {
			m.status = errMark + " Already persistent"
			return m, nil
		}
		m.status = "Persisting..."
		return m, m.persistPlay(p.ID)
	case "e": // extend lifetime (opens the lifetime dialog)
		if p == nil || !p.IsActive() {
			m.status = errMark + " Select a running playground to extend"
			return m, nil
		}
		m.modal = modalExtend
		m.extendBtn = 0
		m.extendID = p.ID
		m.input.SetValue("")
		m.input.Placeholder = "90m, 3h"
		return m, m.input.Focus()
	case "w": // share a web terminal (choose access)
		if p == nil || !p.IsActive() {
			m.status = errMark + " Select a running playground to share"
			return m, nil
		}
		m.modal = modalShare
		m.exposeID = p.ID
		m.shareBtn = 0 // default to Private
		return m, nil
	case "x": // expose HTTP port(s)
		if p == nil || !p.IsActive() {
			m.status = errMark + " Select a running playground to expose a port"
			return m, nil
		}
		m.modal = modalExposePort
		m.exposeID = p.ID
		m.input.SetValue("")
		m.input.Placeholder = "ports e.g. 8080, 9090"
		return m, m.input.Focus()
	case "F": // start a local background port-forward
		if p == nil || !p.StateIs(api.StateRunning) {
			m.status = errMark + " Select a running playground to forward a port"
			return m, nil
		}
		m.modal = modalForward
		m.fwdTarget = pending{id: p.ID, name: p.Playground.Name}
		m.input.SetValue("")
		m.input.Placeholder = "8080 or 3000:80"
		return m, m.input.Focus()
	case "ctrl+f": // stop this lab's background port-forwards (with confirmation)
		if p == nil {
			return m, nil
		}
		if len(m.fwdPortsByPlay()[p.ID]) == 0 {
			m.status = errMark + " No forwards on this lab"
			return m, nil
		}
		m.modal = modalStopForward
		m.fwdTarget = pending{id: p.ID, name: p.Playground.Name}
		m.confirmBtn = 0 // default to Cancel
		return m, nil
	case "ctrl+d": // destroy the playground (with confirmation)
		if p == nil {
			return m, nil
		}
		m.confirm = pending{id: p.ID, name: p.Playground.Name, persistent: m.persistedIDs[p.ID]}
		m.confirmBtn = 0 // default-highlight Cancel for a destructive action
		m.confirmStage = 0
		m.modal = modalConfirm
		m.status = ""
		return m, nil
	}
	cmd := m.delegate(msg)
	return m, cmd
}

// handleExportsKey drives the Exports tab: enter copies the URL, o opens it,
// ctrl+d unexposes the selected endpoint.
func (m model) handleExportsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	e := m.selectedExport()
	switch msg.String() {
	case "enter":
		if e == nil {
			return m, nil
		}
		note := okMark + " " + e.url
		if err := clipboard.WriteAll(e.url); err == nil {
			note += " (copied)"
		}
		return m, m.flash(note)
	case "o":
		if e != nil {
			_ = browser.Open(e.url)
		}
		return m, nil
	case "ctrl+d": // unexpose the selected endpoint
		if e == nil {
			return m, nil
		}
		m.status = "Unexposing..."
		return m, m.unexpose(*e)
	}
	cmd := m.delegate(msg)
	return m, cmd
}

func (m model) handleCatalogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		pg := m.selectedPlayground()
		if pg == nil {
			return m, nil
		}
		// Pick a region before spawning, defaulted to the platform preference.
		m.modal = modalRegion
		m.regionForSpawn = true
		m.spawnName = pg.Name
		m.regionBtn = slices.Index(api.KnownRegions, m.region)
		if m.regionBtn < 0 {
			m.regionBtn = 0
		}
		return m, nil
	}
	cmd := m.delegate(msg)
	return m, cmd
}

// delegate forwards navigation keys to the focused table, persisting its
// updated state (cursor position) back onto the model.
func (m *model) delegate(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.tab {
	case tabCatalog:
		m.catalogTable, cmd = m.catalogTable.Update(msg)
	case tabExports:
		m.exportsTable, cmd = m.exportsTable.Update(msg)
	default:
		m.playsTable, cmd = m.playsTable.Update(msg)
	}
	return cmd
}

// gapsFor returns the blank-row breathing space (header→table, title→header),
// collapsed to 0 on short terminals to reclaim data rows.
func gapsFor(h int) (header, table int) {
	if h < 22 {
		return 0, 0
	}
	return 1, 1
}

// distribute splits total across columns by weight, honoring per-column minimums
// so columns shrink gracefully on narrow terminals instead of overflowing.
func distribute(total int, weights, mins []int) []int {
	out := make([]int, len(weights))
	sumMin, sumW := 0, 0
	for i := range weights {
		sumMin += mins[i]
		sumW += weights[i]
	}
	extra := total - sumMin
	if extra < 0 {
		extra = 0
	}
	used := 0
	for i := range weights {
		out[i] = mins[i]
		if sumW > 0 {
			add := extra * weights[i] / sumW
			out[i] += add
			used += add
		}
	}
	out[len(out)-1] += extra - used // rounding remainder to the last column
	return out
}

func (m *model) setSizes(w, h int) {
	hGap, tGap := gapsFor(h)
	// Reserve rows: header(4) + header gap + box border(2) + footer(1) + table gap.
	bodyH := h - headerRows - hGap - 3 - tGap
	if bodyH < 3 {
		bodyH = 3
	}
	m.playsTable.SetHeight(bodyH)
	m.catalogTable.SetHeight(bodyH)

	inner := w - 2 // titledBox eats one column per side

	pc := distribute(inner-8, []int{4, 1, 2, 2, 2}, []int{12, 6, 8, 8, 12}) // NAME REGION STATUS TTL PF
	m.playsTable.SetColumns([]table.Column{
		{Title: "NAME", Width: pc[0]},
		{Title: "REGION", Width: pc[1]},
		{Title: "STATUS", Width: pc[2]},
		{Title: "TTL", Width: pc[3]},
		{Title: "PF", Width: pc[4]},
	})
	m.playsTable.SetWidth(inner)

	cc := distribute(inner-4, []int{1, 3}, []int{12, 15}) // PLAYGROUND DESCRIPTION
	m.catalogTable.SetColumns([]table.Column{
		{Title: "PLAYGROUND", Width: cc[0]},
		{Title: "DESCRIPTION", Width: cc[1]},
	})
	m.catalogTable.SetWidth(inner)

	ec := distribute(inner-6, []int{2, 1, 4}, []int{8, 6, 16}) // LAB KIND URL
	m.exportsTable.SetColumns([]table.Column{
		{Title: "LAB", Width: ec[0]},
		{Title: "KIND", Width: ec[1]},
		{Title: "URL", Width: ec[2]},
	})
	m.exportsTable.SetWidth(inner)
	m.exportsTable.SetHeight(bodyH)
}

func shortStatus(p *api.Play) string {
	st := p.State()
	if st == "" {
		return "UNKNOWN"
	}
	return string(st)
}

// playAge renders the remaining lifetime for running labs, counting down
// (e.g. "7h50m"); for non-running labs it shows the elapsed time.
func playAge(p *api.Play) string {
	created := parseTime(p.CreatedAt)
	if created.IsZero() {
		return "-"
	}
	elapsed := time.Since(created)
	if elapsed < 0 {
		elapsed = 0
	}
	if p.StateIs(api.StateRunning) {
		// Prefer the configured total (maxPlayTime, e.g. "480m"); fall back to
		// elapsed + remaining when it's missing.
		total, _ := time.ParseDuration(p.MaxPlayTime)
		remaining := total - elapsed
		if total <= 0 && p.ExpiresIn > 0 {
			remaining = time.Duration(p.ExpiresIn) * time.Millisecond
			total = elapsed + remaining
		}
		if total > 0 {
			if remaining < 0 {
				remaining = 0
			}
			return fmtDur(remaining)
		}
	}
	return fmtDur(elapsed)
}

// fmtDur renders a compact duration: 45s, 12m, 1h, 3h20m.
func fmtDur(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		h := int(d.Hours())
		if mins := int(d.Minutes()) % 60; mins > 0 {
			return fmt.Sprintf("%dh%dm", h, mins)
		}
		return fmt.Sprintf("%dh", h)
	}
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// Skin is the color scheme. Any field is a hex code (#RRGGBB) or ANSI-256
// number. Pick a built-in theme and/or override fields via skin.yaml next to
// the labctl config (~/.iximiuz/labctl/skin.yaml):
//
//	theme: dracula        # one of: k9s dracula nord gruvbox solarized catppuccin
//	border: "#ff00ff"     # optional per-field overrides on top of the theme
type Skin struct {
	Cells  string `yaml:"cells"`  // table cell text
	Border string `yaml:"border"` // frame borders, hotkeys, focused button
	Logo   string `yaml:"logo"`   // logo, status line, active breadcrumb
	Body   string `yaml:"body"`   // body text, help, info values
	Header string `yaml:"header"` // table header, menu labels, dialog titles
	Button string `yaml:"button"` // unfocused dialog button bg
	SelFg  string `yaml:"selFg"`  // selected-row foreground
	SelBg  string `yaml:"selBg"`  // selected-row background
}

// presets are well-known terminal color schemes. "k9s" is the default.
var presets = map[string]Skin{
	"k9s":         {Cells: "#00FFFF", Border: "#1E90FF", Logo: "#FFA500", Body: "#5F9EA0", Header: "#FFFFFF", Button: "#483D8B", SelFg: "#FFFFFF", SelBg: "#005F87"},
	"dracula":     {Cells: "#F8F8F2", Border: "#BD93F9", Logo: "#FFB86C", Body: "#6272A4", Header: "#F8F8F2", Button: "#44475A", SelFg: "#F8F8F2", SelBg: "#44475A"},
	"nord":        {Cells: "#D8DEE9", Border: "#81A1C1", Logo: "#EBCB8B", Body: "#616E88", Header: "#ECEFF4", Button: "#434C5E", SelFg: "#ECEFF4", SelBg: "#3B4252"},
	"gruvbox":     {Cells: "#EBDBB2", Border: "#83A598", Logo: "#FE8019", Body: "#928374", Header: "#FBF1C7", Button: "#3C3836", SelFg: "#FBF1C7", SelBg: "#504945"},
	"solarized":   {Cells: "#93A1A1", Border: "#268BD2", Logo: "#CB4B16", Body: "#586E75", Header: "#FDF6E3", Button: "#073642", SelFg: "#FDF6E3", SelBg: "#0A4B55"},
	"catppuccin":  {Cells: "#CDD6F4", Border: "#89B4FA", Logo: "#FAB387", Body: "#A6ADC8", Header: "#CDD6F4", Button: "#45475A", SelFg: "#1E1E2E", SelBg: "#89B4FA"},
	"github-dark": {Cells: "#C9D1D9", Border: "#58A6FF", Logo: "#D29922", Body: "#8B949E", Header: "#F0F6FC", Button: "#21262D", SelFg: "#F0F6FC", SelBg: "#1F6FEB"},
	"tango-dark":  {Cells: "#D3D7CF", Border: "#729FCF", Logo: "#FCAF3E", Body: "#888A85", Header: "#EEEEEC", Button: "#555753", SelFg: "#EEEEEC", SelBg: "#204A87"},

	// VS Code built-in themes.
	"dark-plus":      {Cells: "#D4D4D4", Border: "#569CD6", Logo: "#CE9178", Body: "#858585", Header: "#FFFFFF", Button: "#264F78", SelFg: "#FFFFFF", SelBg: "#264F78"},
	"light-plus":     {Cells: "#1F1F1F", Border: "#0000FF", Logo: "#A31515", Body: "#6E6E6E", Header: "#000000", Button: "#ADD6FF", SelFg: "#000000", SelBg: "#ADD6FF"},
	"monokai":        {Cells: "#F8F8F2", Border: "#66D9EF", Logo: "#FD971F", Body: "#75715E", Header: "#F8F8F2", Button: "#49483E", SelFg: "#F8F8F2", SelBg: "#49483E"},
	"abyss":          {Cells: "#6688CC", Border: "#2277FF", Logo: "#FF9900", Body: "#406385", Header: "#FFFFFF", Button: "#103050", SelFg: "#FFFFFF", SelBg: "#103050"},
	"kimbie-dark":    {Cells: "#D3AF86", Border: "#98676A", Logo: "#F79A32", Body: "#A57A4C", Header: "#FBEBD4", Button: "#51412C", SelFg: "#FBEBD4", SelBg: "#5E452B"},
	"red":            {Cells: "#F8F8F8", Border: "#FB9FB1", Logo: "#FFB454", Body: "#C5808F", Header: "#FFFFFF", Button: "#86181D", SelFg: "#FFFFFF", SelBg: "#86181D"},
	"tomorrow-night": {Cells: "#CCCCCC", Border: "#81A2BE", Logo: "#DE935F", Body: "#969896", Header: "#FFFFFF", Button: "#373B41", SelFg: "#FFFFFF", SelBg: "#373B41"},
}

// themeOrder is the cycle/selection order for the in-TUI theme switcher.
var themeOrder = []string{
	"k9s", "dracula", "nord", "gruvbox", "solarized", "catppuccin", "github-dark", "tango-dark",
	"dark-plus", "light-plus", "monokai", "abyss", "kimbie-dark", "red", "tomorrow-night",
}

func themeIndex(name string) int {
	for i, n := range themeOrder {
		if n == name {
			return i
		}
	}
	return 0
}

// loadSkin resolves the theme name and overlays any per-field overrides from
// skin.yaml. Best-effort: a missing/invalid file just yields the k9s default.
func loadSkin() (Skin, string) {
	name := "k9s"
	home, err := os.UserHomeDir()
	if err != nil {
		return presets[name], name
	}
	path := filepath.Join(filepath.Dir(config.ConfigFilePath(home)), "skin.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return presets[name], name
	}

	var head struct {
		Theme string `yaml:"theme"`
	}
	_ = yaml.Unmarshal(data, &head)
	if _, ok := presets[head.Theme]; ok {
		name = head.Theme
	}

	s := presets[name]
	_ = yaml.Unmarshal(data, &s) // overlay explicit field overrides
	return s, name
}

var (
	cCells, cBorder, cLogo, cBody, cHeader, cButton, cSelFg, cSelBg lipgloss.Color

	statusStyle, helpStyle, infoKey, infoVal, menuKey, menuText, logoStyle lipgloss.Style
	dialogTitle, btnSel, btnUnsel                                          lipgloss.Style
)

func init() { initStyles(presets["k9s"]) }

func initStyles(s Skin) {
	cCells = lipgloss.Color(s.Cells)
	cBorder = lipgloss.Color(s.Border)
	cLogo = lipgloss.Color(s.Logo)
	cBody = lipgloss.Color(s.Body)
	cHeader = lipgloss.Color(s.Header)
	cButton = lipgloss.Color(s.Button)
	cSelFg = lipgloss.Color(s.SelFg)
	cSelBg = lipgloss.Color(s.SelBg)

	statusStyle = lipgloss.NewStyle().Foreground(cLogo)
	helpStyle = lipgloss.NewStyle().Foreground(cBody)
	infoKey = lipgloss.NewStyle().Foreground(cBorder)
	infoVal = lipgloss.NewStyle().Foreground(cBody)
	menuKey = lipgloss.NewStyle().Foreground(cBorder)
	menuText = lipgloss.NewStyle().Foreground(cHeader)
	logoStyle = lipgloss.NewStyle().Foreground(cLogo).Bold(true)

	dialogTitle = lipgloss.NewStyle().Foreground(cBorder).Bold(true)
	btnSel = lipgloss.NewStyle().Padding(0, 2).Background(cBorder).Foreground(lipgloss.Color("#000000")).Bold(true)
	btnUnsel = lipgloss.NewStyle().Padding(0, 2).Background(cButton).Foreground(cHeader)
}

// labctl wordmark shown top-right, k9s-style (rendered in the logo color).
var logoSmall = []string{
	"╻  ┏━┓┏┓ ┏━╸╺┳╸╻  ",
	"┃  ┣━┫┣┻┓┃   ┃ ┃  ",
	"┗━╸╹ ╹┗━┛┗━╸ ╹ ┗━╸",
}

func applySkin(t *table.Model) {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(cBorder).
		BorderBottom(true).
		Bold(true).
		Foreground(cHeader)
	s.Selected = s.Selected.Foreground(cSelFg).Background(cSelBg).Bold(true)
	s.Cell = s.Cell.Foreground(cCells)
	t.SetStyles(s)
}

func infoLine(k, v string) string {
	return infoKey.Render(fmt.Sprintf("%-9s", k+":")) + infoVal.Render(v)
}

// titledBox draws a k9s-style frame: a colored border with the title embedded
// in the top edge. It normalizes every body line to one uniform inner width
// (truncating/padding, ANSI-aware) so the border stays square regardless of the
// table's internal line widths. maxW caps the frame to the terminal width.
func titledBox(title, body string, maxW int) string {
	lines := strings.Split(body, "\n")
	innerW := lipgloss.Width(title) + 4
	for _, ln := range lines {
		if wdt := lipgloss.Width(ln); wdt > innerW {
			innerW = wdt
		}
	}
	if innerW > maxW {
		innerW = maxW
	}

	bs := lipgloss.NewStyle().Foreground(cBorder)
	dashes := innerW - lipgloss.Width(title) - 3
	if dashes < 0 {
		dashes = 0
	}
	side := bs.Render("│")

	var b strings.Builder
	b.WriteString(bs.Render("╭─ ") + title + bs.Render(" "+strings.Repeat("─", dashes)+"╮") + "\n")
	for _, ln := range lines {
		ln = ansi.Truncate(ln, innerW, "")
		pad := innerW - lipgloss.Width(ln)
		if pad < 0 {
			pad = 0
		}
		b.WriteString(side + ln + strings.Repeat(" ", pad) + side + "\n")
	}
	b.WriteString(bs.Render("╰" + strings.Repeat("─", innerW) + "╯"))
	return b.String()
}

// buttonRow renders k9s-style filled buttons side by side (SetButtonsAlign
// center), highlighting the selected one. selected < 0 highlights none.
func buttonRow(labels []string, selected int) string {
	parts := make([]string, 0, len(labels)*2-1)
	for i, l := range labels {
		if i > 0 {
			parts = append(parts, "  ")
		}
		if i == selected {
			parts = append(parts, btnSel.Render(l))
		} else {
			parts = append(parts, btnUnsel.Render(l))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
}

// kDialog renders a roomy k9s-style modal (tview.ModalForm): a bordered box with
// the title embedded in the top edge as "< Title >", message/fields centered,
// and buttons centered at the bottom.
func kDialog(title string, lines []string) string {
	innerW := lipgloss.Width("< "+title+" >") + 2
	for _, l := range lines {
		if w := lipgloss.Width(l); w > innerW {
			innerW = w
		}
	}
	innerW += 12 // generous breathing room
	if innerW < 40 {
		innerW = 40
	}

	bs := lipgloss.NewStyle().Foreground(cBorder)
	side := bs.Render("│")
	blank := side + strings.Repeat(" ", innerW) + side

	titleStr := dialogTitle.Render("< " + title + " >")
	dash := innerW - lipgloss.Width("< "+title+" >")
	ld := dash / 2
	rd := dash - ld

	var b strings.Builder
	b.WriteString(bs.Render("╭"+strings.Repeat("─", ld)) + titleStr + bs.Render(strings.Repeat("─", rd)+"╮") + "\n")
	b.WriteString(blank + "\n" + blank + "\n") // top padding
	for _, l := range lines {
		lw := lipgloss.Width(l)
		lp := (innerW - lw) / 2
		rp := innerW - lw - lp
		if lp < 0 {
			lp = 0
		}
		if rp < 0 {
			rp = 0
		}
		b.WriteString(side + strings.Repeat(" ", lp) + l + strings.Repeat(" ", rp) + side + "\n")
	}
	b.WriteString(blank + "\n" + blank + "\n") // bottom padding
	b.WriteString(bs.Render("╰" + strings.Repeat("─", innerW) + "╯"))
	return b.String()
}

// centered draws box on a blank canvas (fully opaque).
func (m model) centered(box string) string {
	w, h := m.dims()
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

// overlayCentered composites box centered over the live main view so the table
// behind stays visible (transparent popup).
func (m model) overlayCentered(box string) string {
	w, h := m.dims()
	x := (w - lipgloss.Width(box)) / 2
	y := (h - lipgloss.Height(box)) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return overlay(m.mainView(), box, x, y)
}

func (m model) tooSmallView() string {
	w, h := m.dims()
	msg := lipgloss.NewStyle().Foreground(cLogo).Bold(true).Render("Terminal too small")
	sub := helpStyle.Render(fmt.Sprintf("needs at least %dx%d  (now %dx%d)", minWidth, minHeight, w, h))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, msg+"\n"+sub)
}

// dims returns the window size with an 80x24 fallback before the first resize.
func (m model) dims() (int, int) {
	if m.width == 0 {
		return 80, 24
	}
	return m.width, m.height
}

func (m model) View() string {
	if w, h := m.dims(); w < minWidth || h < minHeight {
		return m.tooSmallView()
	}
	switch m.modal {
	case modalAuth:
		return m.authView()
	case modalConfirm:
		return m.confirmView()
	case modalQuit:
		return m.quitView()
	case modalExtend:
		return m.extendSelectView()
	case modalShare:
		return m.shareView()
	case modalExposePort:
		return m.exposePortView()
	case modalInfo:
		return m.infoView()
	case modalThemes:
		return m.themeView()
	case modalRegion:
		return m.regionView()
	case modalForward:
		return m.forwardView()
	case modalStopForward:
		return m.stopForwardView()
	case modalHelp:
		return m.helpView()
	default: // modalNone or modalFilter (filter renders as a footer bar)
		return m.mainView()
	}
}

func (m model) mainView() string {
	w, _ := m.dims()

	var body string
	switch m.tab {
	case tabCatalog:
		body = m.catalogTable.View()
	case tabExports:
		body = m.exportsTable.View()
	default:
		body = m.playsTable.View()
	}

	var footer string
	switch {
	case m.modal == modalFilter:
		footer = menuKey.Render("/ ") + m.input.View()
	case m.filter != "":
		footer = menuKey.Render("/" + m.filter)
	default:
		footer = renderStatus(m.status)
	}

	_, h := m.dims()
	hGap, tGap := gapsFor(h)
	body = strings.Repeat("\n", tGap) + body // gap between tab title and header row

	rows := []string{m.headerView(w)}
	for range hGap {
		rows = append(rows, "")
	}
	rows = append(rows, titledBox(m.tabsTitle(), body, w-2), footer)
	return strings.Join(rows, "\n")
}

// tabsTitle renders the playgrounds/catalog tabs as filled blocks embedded in
// the table's title bar, with the active view's row count.
func (m model) tabsTitle() string {
	tab := func(label string, active bool) string {
		bg := cCells
		if active {
			bg = cLogo
		}
		return lipgloss.NewStyle().
			Background(bg).Foreground(lipgloss.Color("#000000")).Bold(true).
			Padding(0, 1).Render(label)
	}
	var count int
	switch m.tab {
	case tabCatalog:
		count = len(m.filteredCat)
	case tabExports:
		count = len(m.filteredExp)
	default:
		count = len(m.filteredPlays)
	}
	suffix := fmt.Sprintf("  [%d]", count)
	if m.filter != "" {
		suffix += " (/" + m.filter + ")"
	}
	return tab("catalog", m.tab == tabCatalog) + " " +
		tab("playgrounds", m.tab == tabPlays) + " " +
		tab("exports", m.tab == tabExports) + dialogTitle.Render(suffix)
}

// applyTheme switches the active color scheme immediately (used for the live
// theme preview).
func (m *model) applyTheme(name string) {
	m.theme = name
	initStyles(presets[name])
	applySkin(&m.playsTable)
	applySkin(&m.catalogTable)
	applySkin(&m.exportsTable)
}

// overlay composites box onto bg at column x, row y (ANSI-aware), leaving the
// rest of bg visible — a "transparent" popup over the live themed view.
func overlay(bg, box string, x, y int) string {
	bgLines := strings.Split(bg, "\n")
	boxW := lipgloss.Width(box)
	for i, bl := range strings.Split(box, "\n") {
		row := y + i
		if row < 0 || row >= len(bgLines) {
			continue
		}
		base := bgLines[row]
		left := ansi.Truncate(base, x, "")
		if lw := lipgloss.Width(left); lw < x {
			left += strings.Repeat(" ", x-lw)
		}
		right := ""
		if lipgloss.Width(base) > x+boxW {
			right = ansi.TruncateLeft(base, x+boxW, "")
		}
		bgLines[row] = left + bl + right
	}
	return strings.Join(bgLines, "\n")
}

func (m model) themePickerBox() string {
	const listW = 13
	rows := make([]string, 0, len(themeOrder)+2)
	for i, name := range themeOrder {
		label := fmt.Sprintf("%-*s", listW, name)
		if i == m.themeIdx {
			rows = append(rows, lipgloss.NewStyle().Foreground(cSelFg).Background(cSelBg).Bold(true).Render("▸ "+label))
		} else {
			rows = append(rows, lipgloss.NewStyle().Foreground(cBody).Render("  "+label))
		}
	}
	rows = append(rows, "", helpStyle.Render("↑/↓ preview · enter ok"))
	return titledBox(dialogTitle.Render("Theme"), strings.Join(rows, "\n"), 40)
}

func (m model) themeView() string {
	w, h := m.dims()
	box := m.themePickerBox()
	x := (w - lipgloss.Width(box)) / 2
	y := (h - lipgloss.Height(box)) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return overlay(m.mainView(), box, x, y)
}

// formField renders a "Label: [input]" row, k9s-style, with the label
// highlighted when its field is focused.
func formField(label string, in textinput.Model, focused bool) string {
	lbl := menuText.Render(fmt.Sprintf("%-13s", label))
	if focused {
		lbl = btnSel.Render(label) + strings.Repeat(" ", 13-lipgloss.Width(label))
	}
	return lbl + " " + in.View()
}

// handleExtendSelectKey drives the extend form (k9s ModalForm): a Lifetime field
// you type into, plus Cancel/OK buttons. Focus 0=field, 1=Cancel, 2=OK.
func (m model) handleExtendSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modalNone
		m.input.Blur()
		return m, nil
	case "tab", "down":
		m.extendBtn = (m.extendBtn + 1) % 3
		return m, m.focusExtend()
	case "up":
		m.extendBtn = (m.extendBtn + 2) % 3
		return m, m.focusExtend()
	case "left", "right": // toggle between Cancel/OK
		switch m.extendBtn {
		case 1:
			m.extendBtn = 2
		case 2:
			m.extendBtn = 1
		}
		return m, nil
	case "enter":
		m.input.Blur()
		if m.extendBtn == 1 { // Cancel
			m.modal = modalNone
			return m, nil
		}
		val := strings.TrimSpace(m.input.Value())
		d, err := time.ParseDuration(val)
		m.modal = modalNone
		if err != nil || int(d.Minutes()) < 1 {
			m.status = errMark + " Invalid lifetime: " + val + " (try 90m, 3h)"
			return m, nil
		}
		m.status = "Setting lifetime..."
		return m, m.extendPlay(m.extendID, int(d.Minutes()))
	default:
		if m.extendBtn == 0 {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		return m, nil
	}
}

func (m *model) focusExtend() tea.Cmd {
	if m.extendBtn == 0 {
		return m.input.Focus()
	}
	m.input.Blur()
	return nil
}

func (m model) extendSelectView() string {
	btn := -1
	if m.extendBtn >= 1 {
		btn = m.extendBtn - 1
	}
	return m.overlayCentered(kDialog("Extend lifetime", []string{
		menuText.Render("New total lifetime from start"),
		"",
		formField("Lifetime:", m.input, m.extendBtn == 0),
		"",
		buttonRow([]string{"Cancel", "OK"}, btn),
	}))
}

func (m model) regionView() string {
	labels := make([]string, len(api.KnownRegions))
	for i, r := range api.KnownRegions {
		labels[i] = strings.ToUpper(r)
	}
	title, msg := "Preferred region", "Region for new playgrounds:"
	if m.regionForSpawn {
		title, msg = "Spawn region", "Spawn "+m.spawnName+" in region:"
	}
	return m.overlayCentered(kDialog(title, []string{
		menuText.Render(msg),
		"",
		buttonRow(labels, m.regionBtn),
	}))
}

func (m model) quitView() string {
	return m.overlayCentered(kDialog("Quit", []string{
		menuText.Render("Quit labctl?"),
		"",
		buttonRow([]string{"Cancel", "Quit"}, m.quitBtn),
	}))
}

func (m model) shareView() string {
	return m.overlayCentered(kDialog("Share terminal", []string{
		menuText.Render("Web terminal access:"),
		"",
		buttonRow([]string{"Private", "Public"}, m.shareBtn),
	}))
}

func (m model) exposePortView() string {
	return m.overlayCentered(kDialog("Expose port(s)", []string{
		menuText.Render("HTTP port(s) to expose publicly"),
		"",
		formField("Port(s):", m.input, true),
	}))
}

func (m model) forwardView() string {
	return m.overlayCentered(kDialog("Port-forward", []string{
		menuText.Render("Forward a local port (runs in the background)"),
		"",
		formField("Port:", m.input, true),
	}))
}

func (m model) stopForwardView() string {
	ports := strings.Join(m.fwdPortsByPlay()[m.fwdTarget.id], ", ")
	return m.overlayCentered(kDialog("Stop forwards", []string{
		menuText.Render("Stop port-forward(s) on " + m.fwdTarget.name + "?"),
		infoVal.Render(ports),
		"",
		buttonRow([]string{"Cancel", "Stop"}, m.confirmBtn),
	}))
}

func (m *model) selectedURL() string {
	if m.tab == tabCatalog {
		if pg := m.selectedPlayground(); pg != nil {
			return pg.PageURL
		}
		return ""
	}
	if p := m.selectedPlay(); p != nil {
		return p.PageURL
	}
	return ""
}

func infoField(k, v string) string {
	if v == "" {
		v = "-"
	}
	return infoKey.Render(fmt.Sprintf("%-12s", k+":")) + infoVal.Render(v)
}

func (m model) infoView() string {
	w, _ := m.dims()
	contentW := min(max(w*2/3, 44), 96) - 4 // text width inside the frame

	var title string
	var fields []string
	var desc string
	if m.tab == tabCatalog {
		pg := m.selectedPlayground()
		if pg == nil {
			return ""
		}
		title = pg.Name
		fields = []string{
			infoField("Title", pg.Title),
			infoField("Categories", strings.Join(pg.Categories, ", ")),
			infoField("Machines", strconv.Itoa(len(pg.Machines))),
			infoField("URL", pg.PageURL),
		}
		desc = pg.Description
	} else {
		p := m.selectedPlay()
		if p == nil {
			return ""
		}
		title = p.Playground.Name
		fields = []string{
			infoField("ID", p.ID),
			infoField("Status", shortStatus(p)),
			infoField("Created", humanize.Time(parseTime(p.CreatedAt))),
			infoField("Lifetime", p.MaxPlayTime),
			infoField("Machines", strconv.Itoa(len(p.Machines))),
			infoField("URL", p.PageURL),
		}
		desc = p.Playground.Description
	}

	parts := []string{strings.Join(fields, "\n")}
	if endpoints := m.exposedLines(contentW); len(endpoints) > 0 {
		parts = append(parts, "", logoStyle.Render("Exposed"))
		parts = append(parts, endpoints...)
	}
	if desc != "" {
		wrapped := lipgloss.NewStyle().Width(contentW).Foreground(cBody).Render(desc)
		parts = append(parts, "", wrapped)
	}
	parts = append(parts, "", helpStyle.Render("o open in browser · any key to close"))

	return m.overlayCentered(titledBox(dialogTitle.Render(title), strings.Join(parts, "\n"), contentW+2))
}

// exposedLines formats the loaded exposed shells/ports for the Info popup.
func (m model) exposedLines(w int) []string {
	var out []string
	trunc := func(s string) string { return ansi.Truncate(s, w, "…") }
	for _, sh := range m.infoShells {
		out = append(out, trunc(infoVal.Render("shell  ")+menuText.Render(sh.URL)))
	}
	for _, pt := range m.infoPorts {
		out = append(out, trunc(infoVal.Render(fmt.Sprintf("%-7d", pt.Number))+menuText.Render(pt.URL)))
	}
	return out
}

func (m model) headerView(w int) string {
	user := m.user
	if user == "" {
		user = "-"
	}
	region := m.region
	if region == "" {
		region = "-"
	} else {
		region = strings.ToUpper(region)
	}
	info := lipgloss.JoinVertical(lipgloss.Left,
		infoLine("Labs", strings.TrimPrefix(m.cli.Config().BaseURL, "https://")),
		infoLine("User", user),
		infoLine("Region", region),
		infoLine("Plays", strconv.Itoa(len(m.plays))),
		infoLine("Catalog", strconv.Itoa(len(m.catalog))),
	)

	// Greedily add the menu, then the logo, only when each still fits the width —
	// so the header degrades instead of overflowing on narrow terminals.
	header := info
	if menu := m.menuView(); lipgloss.Width(header)+5+lipgloss.Width(menu) <= w {
		header = lipgloss.JoinHorizontal(lipgloss.Top, header, "     ", menu)
	}
	if logo := logoStyle.Render(strings.Join(logoSmall, "\n")); lipgloss.Width(header)+2+lipgloss.Width(logo) <= w {
		gap := w - lipgloss.Width(header) - lipgloss.Width(logo)
		header = lipgloss.JoinHorizontal(lipgloss.Top, header, strings.Repeat(" ", gap), logo)
	}

	// Pin the header to exactly headerRows so the table never shifts between tabs.
	lines := strings.Split(header, "\n")
	for len(lines) < headerRows {
		lines = append(lines, "")
	}
	return strings.Join(lines[:headerRows], "\n")
}

const headerRows = 5 // header pinned to this many rows for cross-tab stability

// Below this the layout can't render usefully; show a "too small" message.
const (
	minWidth  = 60
	minHeight = 14
)

// menuView shows the per-tab essential shortcuts (everything else lives in the
// ? popup). At most 3 rows tall, so the 4-row header stays stable.
func (m model) menuView() string {
	var items [][2]string
	switch m.tab {
	case tabCatalog:
		items = [][2]string{
			{"enter", "Start"}, {"i", "Info"}, {":", "Filter"},
			{"r", "Refresh"}, {"tab", "Switch"}, {"?", "Shortcuts"},
		}
	case tabExports:
		items = [][2]string{
			{"enter", "Copy"}, {"o", "Open"}, {"ctrl+d", "Unexpose"},
			{":", "Filter"}, {"r", "Refresh"}, {"?", "Shortcuts"},
		}
	default: // tabPlays
		items = [][2]string{
			{"enter", "SSH"}, {"i", "Info"}, {"w", "Share"},
			{"x", "Ports"}, {"F", "Forward"}, {"P", "Persist"},
			{":", "Filter"}, {"r", "Refresh"}, {"?", "Shortcuts"},
		}
	}
	return menuColumns(items)
}

// menuColumns lays items out column-major in 3-row columns.
func menuColumns(items [][2]string) string {
	const rows = 3
	cols := (len(items) + rows - 1) / rows
	parts := make([]string, 0, cols*2)
	for c := range cols {
		var lines []string
		for r := range rows {
			if idx := c*rows + r; idx < len(items) {
				it := items[idx]
				lines = append(lines, menuKey.Render("<"+it[0]+"> ")+menuText.Render(it[1]))
			}
		}
		if c > 0 {
			parts = append(parts, "   ")
		}
		parts = append(parts, lipgloss.JoinVertical(lipgloss.Left, lines...))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m model) helpView() string {
	key := func(k, d string) string {
		return menuKey.Render(fmt.Sprintf(" %-10s", k)) + menuText.Render(d)
	}
	hdr := func(s string) string { return logoStyle.Render(s) }

	left := strings.Join([]string{
		hdr("Navigation"),
		key("↑/↓ j/k", "move"),
		key("g / G", "top / bottom"),
		key("tab", "switch view"),
		key("/ or :", "filter"),
		"",
		hdr("Playground"),
		key("enter", "ssh"),
		key("o", "open in browser"),
		key("i", "info"),
		key("s", "start/stop toggle"),
		key("P", "persist"),
		key("e", "extend lifetime"),
		key("ctrl+d", "destroy"),
	}, "\n")
	right := strings.Join([]string{
		hdr("Export"),
		key("w", "share terminal"),
		key("x", "expose port(s)"),
		key("F", "port-forward (bg)"),
		key("ctrl+f", "stop forwards"),
		"",
		hdr("Exports tab"),
		key("enter", "copy url"),
		key("o", "open url"),
		key("ctrl+d", "unexpose"),
		"",
		hdr("General"),
		key("r", "refresh"),
		key("R", "preferred region"),
		key("T", "theme picker"),
		key("q", "quit"),
		key("?", "close help"),
	}, "\n")

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, "     ", right)
	return m.overlayCentered(titledBox(dialogTitle.Render("Shortcuts"), body, 72))
}

func (m model) authView() string {
	return m.overlayCentered(kDialog("Sign in", []string{
		menuText.Render("Not signed in to iximiuz Labs"),
		"",
		buttonRow([]string{"Dismiss", "Login via browser"}, m.authBtn),
	}))
}

func (m model) confirmView() string {
	msg := "Destroy " + m.confirm.name + "?"
	if m.confirm.persistent {
		if m.confirmStage == 0 {
			msg = "Destroy PERSISTENT lab " + m.confirm.name + "?"
		} else {
			msg = "Confirm again — permanently destroy " + m.confirm.name + "?"
		}
	}
	return m.overlayCentered(kDialog("Confirm", []string{
		menuText.Render(msg),
		infoVal.Render(m.confirm.id),
		"",
		buttonRow([]string{"Cancel", "Destroy"}, m.confirmBtn),
	}))
}
