package skills

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Rule is a kit rule - a markdown instruction file - in an agent-neutral form.
type Rule struct {
	Name string

	// Paths are the globs (relative to the project root) of the files the rule
	// applies to. A rule without paths always applies.
	Paths []string

	// Description is the rule's first top-level heading.
	Description string

	// Body is the rule's markdown without the frontmatter.
	Body []byte
}

// parseRule parses a kit rule file: an optional frontmatter with a `paths`
// list followed by the markdown body.
func parseRule(name string, content []byte) (Rule, error) {
	r := Rule{Name: name, Body: content}

	s := string(content)
	if strings.HasPrefix(s, "---\n") {
		end := strings.Index(s[4:], "\n---\n")
		if end == -1 {
			return Rule{}, fmt.Errorf("rule %s: unterminated frontmatter", name)
		}

		var meta struct {
			Paths []string `yaml:"paths"`
		}
		if err := yaml.Unmarshal([]byte(s[4:4+end]), &meta); err != nil {
			return Rule{}, fmt.Errorf("rule %s: invalid frontmatter: %w", name, err)
		}

		r.Paths = meta.Paths
		r.Body = []byte(strings.TrimLeft(s[4+end+len("\n---\n"):], "\n"))
	}

	for _, line := range strings.Split(string(r.Body), "\n") {
		if strings.HasPrefix(line, "# ") {
			r.Description = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			break
		}
	}

	return r, nil
}
