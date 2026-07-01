package search

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

// kindColors maps each content kind to a 256-color code used for its badge.
// Kinds not listed fall back to a neutral gray.
var kindColors = map[string]string{
	"challenge":  "39",  // blue
	"tutorial":   "42",  // green
	"course":     "135", // purple
	"skill-path": "170", // magenta
	"roadmap":    "208", // orange
	"playground": "45",  // cyan
	"vendor":     "244", // gray
	"lesson":     "78",  // teal
	"doc":        "110", // slate blue
}

const badgeWidth = 10 // widest kind label ("skill-path"/"playground") for aligned badges

// styler renders decorated result fragments. When color is false every method
// returns plain text, so piped/redirected output stays clean.
type styler struct {
	color bool

	dim   lipgloss.Style
	bold  lipgloss.Style
	url   lipgloss.Style
	star  lipgloss.Style
	count lipgloss.Style
}

func newStyler(color bool) *styler {
	return &styler{
		color: color,
		dim:   lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		bold:  lipgloss.NewStyle().Bold(true),
		url:   lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
		star:  lipgloss.NewStyle().Foreground(lipgloss.Color("220")),
		count: lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
	}
}

func (s *styler) kindBadge(kind string) string {
	label := fmt.Sprintf("%-*s", badgeWidth, strings.ToUpper(kind))
	if !s.color {
		return "[" + strings.TrimSpace(label) + "]"
	}

	bg := kindColors[kind]
	if bg == "" {
		bg = "240"
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color(bg)).
		Padding(0, 1).
		Render(label)
}

func (s *styler) difficulty(d string) string {
	if d == "" {
		return ""
	}
	if !s.color {
		return d
	}

	color := "244"
	switch strings.ToLower(d) {
	case "easy", "beginner":
		color = "42"
	case "medium", "intermediate":
		color = "214"
	case "hard", "advanced", "expert":
		color = "203"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(d)
}

func (s *styler) render(style lipgloss.Style, text string) string {
	if !s.color {
		return text
	}
	return style.Render(text)
}

// renderResults prints the pretty, human-facing search output.
func renderResults(cli labcli.CLI, search string, opts *options, result *api.SearchResult) {
	out := cli.OutputStream()
	s := newStyler(out.IsTerminal())

	width := 100
	if _, w := out.GetTtySize(); w > 20 {
		width = int(w)
	}

	cli.PrintOut("\n%s\n\n", summaryLine(s, search, opts, result))

	if len(result.Items) == 0 {
		cli.PrintOut("  %s\n\n", s.render(s.dim, "No matches. Try a broader query or drop some filters."))
		return
	}

	for i, item := range result.Items {
		num := opts.offset + i + 1
		renderItem(cli, s, width, num, item)
	}
}

func summaryLine(s *styler, search string, opts *options, result *api.SearchResult) string {
	var b strings.Builder
	b.WriteString("🔎  ")

	if result.Total == 0 {
		b.WriteString(s.render(s.bold, "No results"))
	} else {
		b.WriteString(s.render(s.bold, fmt.Sprintf("%s result%s", humanize.Comma(int64(result.Total)), plural(result.Total))))
	}

	if search != "" {
		b.WriteString(" for ")
		b.WriteString(s.render(s.bold, fmt.Sprintf("%q", search)))
	}

	// Show the visible window when it's a strict subset of the total.
	if n := len(result.Items); n > 0 && (opts.offset > 0 || n < result.Total) {
		lo := opts.offset + 1
		hi := opts.offset + n
		b.WriteString(s.render(s.dim, fmt.Sprintf("  ·  showing %d–%d", lo, hi)))
	}

	return b.String()
}

func renderItem(cli labcli.CLI, s *styler, width, num int, item api.SearchItem) {
	// Line 1: index, kind badge, title, and an "official" marker.
	title := item.Title
	if title == "" {
		title = item.Name
	}
	head := fmt.Sprintf("  %s  %s  %s",
		s.render(s.dim, fmt.Sprintf("%2d.", num)),
		s.kindBadge(item.Kind),
		s.render(s.bold, title),
	)
	if item.Official {
		head += "  " + s.render(s.star, "★")
	}
	cli.PrintOut("%s\n", head)

	// Line 2: one-line description, truncated to the terminal width.
	if desc := strings.TrimSpace(item.Description); desc != "" {
		cli.PrintOut("      %s\n", s.render(s.dim, truncate(oneLine(desc), width-6)))
	}

	// Line 3: facets and popularity.
	if meta := metaLine(s, item); meta != "" {
		cli.PrintOut("      %s\n", meta)
	}

	// Line 4: the page URL.
	if item.PageURL != "" {
		cli.PrintOut("      %s\n", s.render(s.url, item.PageURL))
	}

	cli.PrintOut("\n")
}

func metaLine(s *styler, item api.SearchItem) string {
	var parts []string

	if d := s.difficulty(item.Difficulty); d != "" {
		parts = append(parts, d)
	}
	if len(item.Categories) > 0 {
		parts = append(parts, s.render(s.dim, strings.Join(item.Categories, ", ")))
	}
	if item.AttemptCount > 0 {
		parts = append(parts, s.render(s.count, fmt.Sprintf("%s attempts", humanize.Comma(int64(item.AttemptCount)))))
	}
	if item.CompletionCount > 0 {
		parts = append(parts, s.render(s.count, fmt.Sprintf("%s completed", humanize.Comma(int64(item.CompletionCount)))))
	}

	return strings.Join(parts, s.render(s.dim, "  ·  "))
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncate(s string, max int) string {
	if max < 1 {
		max = 1
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}
