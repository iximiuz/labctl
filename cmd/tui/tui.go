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
	tabPersisted
)

const tabCount = 3

type (
	playsMsg   struct{ plays, persisted []*api.Play }
	catalogMsg struct{ items []api.Playground }
	actionMsg  struct{ info string }
	errMsg     struct{ err error }
	tickMsg    struct{}
	authMsg    struct {
		ok   bool
		user string
	}
	loginDoneMsg   struct{ err error }
	disarmQuitMsg  struct{}
	statusClearMsg struct{ seq int }
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
	id, name string
}

// modal is the single active overlay/prompt. Exactly one is active at any time,
// which is what makes this an enum instead of a pile of independent booleans.
type modal uint8

const (
	modalNone    modal = iota
	modalFilter        // footer search bar (renders within mainView)
	modalExtend        // lifetime dialog
	modalAuth          // sign-in popup
	modalConfirm       // destroy confirmation
	modalQuit          // quit confirmation
	modalInfo          // details popup
	modalThemes        // theme picker (live preview)
	modalHelp          // shortcuts popup
)

type model struct {
	cli labcli.CLI

	tab            viewTab
	playsTable     table.Model
	persistedTable table.Model
	catalogTable   table.Model

	plays         []*api.Play      // full, unfiltered
	persisted     []*api.Play      // persistent plays (ListPlays{Persistent:true})
	catalog       []api.Playground // full, unfiltered
	filteredPlays []*api.Play      // rows currently shown (cursor indexes this)
	filteredPers  []*api.Play
	filteredCat   []api.Playground

	modal modal // the single active overlay/prompt

	filter    string
	input     textinput.Model // filter + extend lifetime field
	extendBtn int             // 0 = field, 1 = Cancel, 2 = OK
	extendID  string          // play being extended

	status        string
	statusSeq     int     // bumped per flash; stale auto-clears are ignored
	confirm       pending // destroy target (valid while modal == modalConfirm)
	confirmBtn    int     // 0 = Cancel, 1 = Destroy
	authBtn       int     // 0 = Dismiss, 1 = Login via browser
	authDismissed bool    // auth popup already dismissed this session
	quitBtn       int     // 0 = Cancel, 1 = Quit
	themePrevIdx  int     // theme to restore if the picker is cancelled
	quitArmed     bool    // first ctrl+c/ctrl+d seen; a second one quits
	theme         string  // active color theme name
	themeIdx      int     // index into themeOrder for the T-key cycler
	user          string  // logged-in user id (from GetMe)
	defaulted     bool    // initial view default (catalog-if-empty) applied
	width, height int
}

func newModel(cli labcli.CLI) model {
	pt := table.New(table.WithFocused(true))
	pp := table.New()
	ct := table.New()
	applySkin(&pt)
	applySkin(&pp)
	applySkin(&ct)
	ti := textinput.New()
	ti.Prompt = ""
	m := model{cli: cli, tab: tabPlays, playsTable: pt, persistedTable: pp, catalogTable: ct, input: ti, status: "Loading...", theme: "k9s"}
	m.setSizes(80, 24)
	return m
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
		return authMsg{ok: true, user: me.ID}
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

		// Playgrounds: active/stopped, non-persistent (persistent labs live only
		// on the Persisted tab).
		plays := slices.DeleteFunc(append([]*api.Play{}, recent...), func(p *api.Play) bool {
			return gone(p) || isPersistent[p.ID]
		})
		slices.SortFunc(plays, byUpdated)

		// Persisted: the persistent labs.
		persisted := slices.DeleteFunc(append([]*api.Play{}, persistent...), gone)
		slices.SortFunc(persisted, byUpdated)

		return playsMsg{plays: plays, persisted: persisted}
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

func (m model) extendPlay(id string, minutes int) tea.Cmd {
	return m.playAction(fmt.Sprintf("Lifetime of %s set to %dm", id, minutes), func(ctx context.Context) error {
		_, err := m.cli.Client().SetPlayMaxPlayTime(ctx, id, minutes)
		return err
	})
}

func (m model) startPlay(name string) tea.Cmd {
	return m.playAction("Started "+name, func(ctx context.Context) error {
		// ponytail: official catalog playgrounds need no safety consent; auto-ack.
		_, err := m.cli.Client().CreatePlay(ctx, api.CreatePlayRequest{
			Playground:              name,
			SafetyDisclaimerConsent: true,
		})
		return err
	})
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

// selectedPlay returns the highlighted play on the active plays-like tab.
func (m *model) selectedPlay() *api.Play {
	var tbl *table.Model
	var rows []*api.Play
	switch m.tab {
	case tabPlays:
		tbl, rows = &m.playsTable, m.filteredPlays
	case tabPersisted:
		tbl, rows = &m.persistedTable, m.filteredPers
	default:
		return nil
	}
	if i := tbl.Cursor(); i >= 0 && i < len(rows) {
		return rows[i]
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
// keeping the cursor->slice mapping in sync.
func filterPlays(plays []*api.Play, f string) ([]*api.Play, []table.Row) {
	out := make([]*api.Play, 0, len(plays))
	rows := make([]table.Row, 0, len(plays))
	for _, p := range plays {
		row := table.Row{p.ID, p.Playground.Name, shortStatus(p), humanize.Time(parseTime(p.CreatedAt))}
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

	var pr, ppr []table.Row
	m.filteredPlays, pr = filterPlays(m.plays, f)
	m.playsTable.SetRows(pr)
	m.filteredPers, ppr = filterPlays(m.persisted, f)
	m.persistedTable.SetRows(ppr)

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
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.setSizes(msg.Width, msg.Height)
		return m, nil

	case playsMsg:
		m.plays = msg.plays
		m.persisted = msg.persisted
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
		clear := m.flash(okMark + " " + msg.info)
		return m, tea.Batch(m.loadPlays(), clear)

	case errMsg:
		clear := m.flash(errMark + " " + msg.err.Error())
		return m, clear

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
	// Double ctrl+c / ctrl+d to quit (Claude-CLI style): first press arms and
	// hints, a second within the window quits. Works in every mode.
	if s := msg.String(); s == "ctrl+c" || s == "ctrl+d" {
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
		return m, nil

	case "shift+tab", "left", "h":
		m.switchTab(-1)
		return m, nil

	case "r":
		m.status = "Refreshing..."
		return m, tea.Batch(m.loadPlays(), m.loadCatalog())

	case "?": // shortcuts popup
		m.modal = modalHelp
		return m, nil

	case "T": // open the theme picker (live preview)
		m.modal = modalThemes
		m.themePrevIdx = m.themeIdx
		return m, nil

	case "i": // show full details of the selected row
		if (m.tab == tabCatalog && m.selectedPlayground() != nil) ||
			(m.tab != tabCatalog && m.selectedPlay() != nil) {
			m.modal = modalInfo
		}
		return m, nil
	}

	if m.tab == tabCatalog {
		return m.handleCatalogKey(msg)
	}
	return m.handlePlaysKey(msg) // tabPlays + tabPersisted share actions
}

// switchTab moves to the next/previous tab and focuses its table.
func (m *model) switchTab(delta int) {
	m.tab = viewTab((int(m.tab) + delta + tabCount) % tabCount)
	m.focusActiveTable()
}

func (m *model) focusActiveTable() {
	m.playsTable.Blur()
	m.persistedTable.Blur()
	m.catalogTable.Blur()
	switch m.tab {
	case tabPersisted:
		m.persistedTable.Focus()
	case tabCatalog:
		m.catalogTable.Focus()
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
			id := m.confirm.id
			m.modal = modalNone
			m.confirmBtn = 0
			m.status = "Destroying..."
			return m, m.destroyPlay(id)
		}
		fallthrough
	case "esc":
		m.modal = modalNone
		m.confirmBtn = 0
		m.status = "Cancelled"
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
		if p == nil || !p.IsActive() {
			m.status = errMark + " Select a running playground to SSH"
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
		if p.StateIs(api.StateStopped) {
			m.status = "Starting..."
			return m, m.restartPlay(p.ID)
		}
		m.status = "Stopping..."
		return m, m.stopPlay(p.ID)
	case "t":
		if p == nil {
			return m, nil
		}
		m.status = "Restarting..."
		return m, m.restartPlay(p.ID)
	case "P": // make the playground persistent (Playgrounds tab, active labs only)
		if m.tab != tabPlays {
			return m, nil // already persistent / not a playground
		}
		if p == nil || !p.IsActive() {
			m.status = errMark + " Select a running playground to persist"
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
	case "x":
		if p == nil {
			return m, nil
		}
		m.confirm = pending{id: p.ID, name: p.Playground.Name}
		m.confirmBtn = 0 // default-highlight Cancel for a destructive action
		m.modal = modalConfirm
		m.status = ""
		return m, nil
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
		m.tab = tabPlays
		m.focusActiveTable()
		m.status = "Starting " + pg.Name + "..."
		return m, m.startPlay(pg.Name)
	}
	cmd := m.delegate(msg)
	return m, cmd
}

// delegate forwards navigation keys to the focused table, persisting its
// updated state (cursor position) back onto the model.
func (m *model) delegate(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.tab {
	case tabPersisted:
		m.persistedTable, cmd = m.persistedTable.Update(msg)
	case tabCatalog:
		m.catalogTable, cmd = m.catalogTable.Update(msg)
	default:
		m.playsTable, cmd = m.playsTable.Update(msg)
	}
	return cmd
}

func (m *model) setSizes(w, h int) {
	// Reserve rows: header(4) + gap(2) + box border(2) + footer(1) + table top
	// gap(1). Fixed so the table is the same height on both tabs.
	bodyH := h - headerRows - headerGap - 3 - tableGap
	if bodyH < 3 {
		bodyH = 3
	}
	m.playsTable.SetHeight(bodyH)
	m.persistedTable.SetHeight(bodyH)
	m.catalogTable.SetHeight(bodyH)

	inner := w - 2 // titledBox eats one column per side
	idW, nameW, statusW := 26, 18, 26
	ageW := inner - idW - nameW - statusW - 6
	if ageW < 10 {
		ageW = 10
	}
	playCols := []table.Column{
		{Title: "ID", Width: idW},
		{Title: "NAME", Width: nameW},
		{Title: "STATUS", Width: statusW},
		{Title: "AGE", Width: ageW},
	}
	m.playsTable.SetColumns(playCols)
	m.playsTable.SetWidth(inner)
	m.persistedTable.SetColumns(playCols)
	m.persistedTable.SetWidth(inner)

	cNameW := 22
	descW := inner - cNameW - 4
	if descW < 20 {
		descW = 20
	}
	m.catalogTable.SetColumns([]table.Column{
		{Title: "PLAYGROUND", Width: cNameW},
		{Title: "DESCRIPTION", Width: descW},
	})
	m.catalogTable.SetWidth(inner)
}

func shortStatus(p *api.Play) string {
	st := p.State()
	if st == "" {
		return "UNKNOWN"
	}
	if p.StateIs(api.StateRunning) {
		return "RUNNING " + humanize.Time(time.Now().Add(time.Duration(p.ExpiresIn)*time.Millisecond))
	}
	return string(st)
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

// dims returns the window size with an 80x24 fallback before the first resize.
func (m model) dims() (int, int) {
	if m.width == 0 {
		return 80, 24
	}
	return m.width, m.height
}

func (m model) View() string {
	switch m.modal {
	case modalAuth:
		return m.authView()
	case modalConfirm:
		return m.confirmView()
	case modalQuit:
		return m.quitView()
	case modalExtend:
		return m.extendSelectView()
	case modalInfo:
		return m.infoView()
	case modalThemes:
		return m.themeView()
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
	case tabPersisted:
		body = m.persistedTable.View()
	case tabCatalog:
		body = m.catalogTable.View()
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

	body = strings.Repeat("\n", tableGap) + body // gap between tab title and header row

	return strings.Join([]string{
		m.headerView(w),
		"", // headerGap blank row between header and table
		titledBox(m.tabsTitle(), body, w-2),
		footer,
	}, "\n")
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
	case tabPersisted:
		count = len(m.filteredPers)
	case tabCatalog:
		count = len(m.filteredCat)
	default:
		count = len(m.filteredPlays)
	}
	suffix := fmt.Sprintf("  [%d]", count)
	if m.filter != "" {
		suffix += " (/" + m.filter + ")"
	}
	return tab("catalog", m.tab == tabCatalog) + " " +
		tab("playgrounds", m.tab == tabPlays) + " " +
		tab("persisted", m.tab == tabPersisted) + dialogTitle.Render(suffix)
}

// applyTheme switches the active color scheme immediately (used for the live
// theme preview).
func (m *model) applyTheme(name string) {
	m.theme = name
	initStyles(presets[name])
	applySkin(&m.playsTable)
	applySkin(&m.persistedTable)
	applySkin(&m.catalogTable)
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

func (m model) quitView() string {
	return m.centered(kDialog("Quit", []string{
		menuText.Render("Quit labctl?"),
		"",
		buttonRow([]string{"Cancel", "Quit"}, m.quitBtn),
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
			infoField("Title", p.Title),
			infoField("Status", shortStatus(p)),
			infoField("Created", humanize.Time(parseTime(p.CreatedAt))),
			infoField("Lifetime", p.MaxPlayTime),
			infoField("Machines", strconv.Itoa(len(p.Machines))),
			infoField("URL", p.PageURL),
		}
		desc = p.Playground.Description
	}

	parts := []string{strings.Join(fields, "\n")}
	if desc != "" {
		wrapped := lipgloss.NewStyle().Width(contentW).Foreground(cBody).Render(desc)
		parts = append(parts, "", wrapped)
	}
	parts = append(parts, "", helpStyle.Render("o open in browser · any key to close"))

	return m.centered(titledBox(dialogTitle.Render(title), strings.Join(parts, "\n"), contentW+2))
}

func (m model) headerView(w int) string {
	user := m.user
	if user == "" {
		user = "-"
	}
	info := lipgloss.JoinVertical(lipgloss.Left,
		infoLine("Labs", strings.TrimPrefix(m.cli.Config().BaseURL, "https://")),
		infoLine("User", user),
		infoLine("Plays", strconv.Itoa(len(m.plays))),
		infoLine("Catalog", strconv.Itoa(len(m.catalog))),
	)
	logo := logoStyle.Render(strings.Join(logoSmall, "\n"))
	left := lipgloss.JoinHorizontal(lipgloss.Top, info, "     ", m.menuView())
	gap := w - lipgloss.Width(left) - lipgloss.Width(logo)
	if gap < 1 {
		gap = 1
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), logo)
	// Pin the header to exactly headerRows so the table never shifts between tabs.
	lines := strings.Split(header, "\n")
	for len(lines) < headerRows {
		lines = append(lines, "")
	}
	return strings.Join(lines[:headerRows], "\n")
}

const (
	headerRows = 4
	headerGap  = 1 // blank row between header and table
	tableGap   = 1 // blank row between the tab title and the table header
)

// menuView shows only the essential shortcuts (everything else lives in the ?
// popup). Both tabs use the same item count so the header height is stable.
func (m model) menuView() string {
	enter := "SSH"
	if m.tab == tabCatalog {
		enter = "Start"
	}
	items := [][2]string{
		{"enter", enter}, {"i", "Info"}, {":", "Filter"},
		{"tab", "Switch"}, {"r", "Refresh"}, {"?", "Shortcuts"},
	}
	if m.tab == tabPlays { // persist only applies to (non-persistent) playgrounds
		items = append(items, [2]string{"P", "Persist"})
	}
	half := (len(items) + 1) / 2
	var col1, col2 []string
	for i, it := range items {
		line := menuKey.Render("<"+it[0]+"> ") + menuText.Render(it[1])
		if i < half {
			col1 = append(col1, line)
		} else {
			col2 = append(col2, line)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.JoinVertical(lipgloss.Left, col1...),
		"   ",
		lipgloss.JoinVertical(lipgloss.Left, col2...),
	)
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
		key("t", "restart"),
		key("P", "persist"),
		key("e", "extend lifetime"),
		key("x", "destroy"),
	}, "\n")
	right := strings.Join([]string{
		hdr("Catalog"),
		key("enter", "start"),
		key("i", "info"),
		"",
		hdr("General"),
		key("r", "refresh"),
		key("T", "theme picker"),
		key("q", "quit"),
		key("?", "close help"),
	}, "\n")

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, "     ", right)
	return m.centered(titledBox(dialogTitle.Render("Shortcuts"), body, 72))
}

func (m model) authView() string {
	return m.centered(kDialog("Sign in", []string{
		menuText.Render("Not signed in to iximiuz Labs"),
		"",
		buttonRow([]string{"Dismiss", "Login via browser"}, m.authBtn),
	}))
}

func (m model) confirmView() string {
	return m.overlayCentered(kDialog("Confirm", []string{
		menuText.Render("Destroy " + m.confirm.name + "?"),
		infoVal.Render(m.confirm.id),
		"",
		buttonRow([]string{"Cancel", "Destroy"}, m.confirmBtn),
	}))
}
