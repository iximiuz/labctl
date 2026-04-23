package completion

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/content"
	"github.com/iximiuz/labctl/internal/labcli"
)

// CompletionFunc is the function signature for cobra ValidArgsFunction.
type CompletionFunc = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

// noFileComp is a shorthand for returning no completions with no file completion.
var noFileComp = cobra.ShellCompDirectiveNoFileComp

// --- Play ID completions ---

// ActivePlays completes playground IDs that are in an active (non-stopped, non-destroyed) state.
// Use for: stop, ssh, port-forward, expose port/shell/list/remove, ssh-proxy, persist.
func ActivePlays(cli labcli.CLI) CompletionFunc {
	return plays(cli, (*api.Play).IsActive)
}

// StoppedPlays completes playground IDs that are in a stopped state.
// Use for: restart.
func StoppedPlays(cli labcli.CLI) CompletionFunc {
	return plays(cli, func(p *api.Play) bool {
		return p.StateIs(api.StateStopped)
	})
}

// NonDestroyedPlays completes playground IDs that are not destroyed.
// Use for: destroy, machines, tasks.
func NonDestroyedPlays(cli labcli.CLI) CompletionFunc {
	return plays(cli, func(p *api.Play) bool {
		return !p.StateIs(api.StateDestroyed)
	})
}

func plays(cli labcli.CLI, filter func(*api.Play) bool) CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, noFileComp
		}

		if cli.Client() == nil {
			return nil, noFileComp
		}

		plays, err := cli.Client().ListPlays(cmd.Context())
		if err != nil {
			return nil, noFileComp
		}

		var completions []string
		for _, p := range plays {
			if filter(p) {
				desc := fmt.Sprintf("%s (%s)", p.Playground.Name, p.State())
				completions = append(completions, fmt.Sprintf("%s\t%s", p.ID, desc))
			}
		}

		return completions, noFileComp
	}
}

// --- Playground name completions ---

// PlaygroundNames completes playground template names for the start command.
// Returns user's custom playgrounds first, then official, then community.
func PlaygroundNames(cli labcli.CLI) CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, noFileComp
		}

		if cli.Client() == nil {
			return nil, noFileComp
		}

		var completions []string
		seen := map[string]bool{}

		// User's custom playgrounds first.
		if custom, err := cli.Client().ListPlaygrounds(cmd.Context(), &api.ListPlaygroundsOptions{
			Filter: "my-custom",
		}); err == nil {
			for _, p := range custom {
				if !seen[p.Name] {
					seen[p.Name] = true
					completions = append(completions, fmt.Sprintf("%s\t[my] %s", p.Name, p.Description))
				}
			}
		}

		// Official playgrounds.
		if official, err := cli.Client().ListPlaygrounds(cmd.Context(), nil); err == nil {
			for _, p := range official {
				if !seen[p.Name] {
					seen[p.Name] = true
					completions = append(completions, fmt.Sprintf("%s\t%s", p.Name, p.Description))
				}
			}
		}

		// Community playgrounds.
		if community, err := cli.Client().ListPlaygrounds(cmd.Context(), &api.ListPlaygroundsOptions{
			Filter: "community",
		}); err == nil {
			for _, p := range community {
				if !seen[p.Name] {
					seen[p.Name] = true
					completions = append(completions, fmt.Sprintf("%s\t[community] %s", p.Name, p.Description))
				}
			}
		}

		return completions, cobra.ShellCompDirectiveKeepOrder
	}
}

// --- Challenge completions ---

// ChallengeNames completes all challenge names from the catalog.
// Use for: challenge start.
func ChallengeNames(cli labcli.CLI) CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, noFileComp
		}

		if cli.Client() == nil {
			return nil, noFileComp
		}

		challenges, err := cli.Client().ListChallenges(cmd.Context(), &api.ListChallengesOptions{})
		if err != nil {
			return nil, noFileComp
		}

		var completions []string
		for _, c := range challenges {
			completions = append(completions, fmt.Sprintf("%s\t%s", c.Name, c.Title))
		}

		return completions, noFileComp
	}
}

// StartedChallengeNames completes only challenge names that have an active play session.
// Use for: challenge stop, challenge complete.
func StartedChallengeNames(cli labcli.CLI) CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, noFileComp
		}

		if cli.Client() == nil {
			return nil, noFileComp
		}

		plays, err := cli.Client().ListPlays(cmd.Context())
		if err != nil {
			return nil, noFileComp
		}

		var completions []string
		for _, p := range plays {
			if p.IsActive() && p.ChallengeName != "" {
				desc := fmt.Sprintf("%s (%s)", p.Playground.Name, p.State())
				completions = append(completions, fmt.Sprintf("%s\t%s", p.ChallengeName, desc))
			}
		}

		return completions, noFileComp
	}
}

// --- Tutorial completions ---

// TutorialNames completes all tutorial names from the catalog.
// Use for: tutorial start.
func TutorialNames(cli labcli.CLI) CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, noFileComp
		}

		if cli.Client() == nil {
			return nil, noFileComp
		}

		tutorials, err := cli.Client().ListTutorials(cmd.Context(), nil)
		if err != nil {
			return nil, noFileComp
		}

		var completions []string
		for _, t := range tutorials {
			completions = append(completions, fmt.Sprintf("%s\t%s", t.Name, t.Title))
		}

		return completions, noFileComp
	}
}

// StartedTutorialNames completes only tutorial names that have an active play session.
// Use for: tutorial stop, tutorial complete.
func StartedTutorialNames(cli labcli.CLI) CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, noFileComp
		}

		if cli.Client() == nil {
			return nil, noFileComp
		}

		plays, err := cli.Client().ListPlays(cmd.Context())
		if err != nil {
			return nil, noFileComp
		}

		var completions []string
		for _, p := range plays {
			if p.IsActive() && p.TutorialName != "" {
				desc := fmt.Sprintf("%s (%s)", p.Playground.Name, p.State())
				completions = append(completions, fmt.Sprintf("%s\t%s", p.TutorialName, desc))
			}
		}

		return completions, noFileComp
	}
}

// --- Course completions ---

// CourseArgs completes course names (first arg) and lesson names (second arg) from the full catalog.
// Use for: course start.
func CourseArgs(cli labcli.CLI) CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if cli.Client() == nil {
			return nil, noFileComp
		}

		switch len(args) {
		case 0:
			courses, err := cli.Client().ListCourses(cmd.Context())
			if err != nil {
				return nil, noFileComp
			}

			var completions []string
			for _, c := range courses {
				completions = append(completions, fmt.Sprintf("%s\t%s", c.Name, c.Title))
			}
			return completions, noFileComp

		case 1:
			course, err := cli.Client().GetCourse(cmd.Context(), args[0])
			if err != nil {
				return nil, noFileComp
			}

			var completions []string
			for _, mod := range course.Modules {
				for _, les := range mod.Lessons {
					desc := les.Title
					if len(course.Modules) > 1 {
						desc = fmt.Sprintf("%s (%s)", les.Title, mod.Title)
					}
					completions = append(completions, fmt.Sprintf("%s\t%s", les.Name, desc))
				}
			}
			return completions, noFileComp

		default:
			return nil, noFileComp
		}
	}
}

// StartedCourseArgs completes only courses with started lessons (first arg)
// and only started lessons within a course (second arg).
// Use for: course stop.
func StartedCourseArgs(cli labcli.CLI) CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if cli.Client() == nil {
			return nil, noFileComp
		}

		switch len(args) {
		case 0:
			// Complete course names that have active play sessions.
			plays, err := cli.Client().ListPlays(cmd.Context())
			if err != nil {
				return nil, noFileComp
			}

			seen := map[string]bool{}
			var completions []string
			for _, p := range plays {
				if p.IsActive() && p.CourseName != "" && !seen[p.CourseName] {
					seen[p.CourseName] = true
					desc := fmt.Sprintf("%s (%s)", p.Playground.Name, p.State())
					completions = append(completions, fmt.Sprintf("%s\t%s", p.CourseName, desc))
				}
			}
			return completions, noFileComp

		case 1:
			// Complete only started lessons for the given course.
			course, err := cli.Client().GetCourse(cmd.Context(), args[0])
			if err != nil || course.Learning == nil {
				return nil, noFileComp
			}

			var completions []string
			for modName, modLearning := range course.Learning.Modules {
				for lesName, lesLearning := range modLearning.Lessons {
					if lesLearning.Started && lesLearning.Play != "" {
						// Find the lesson title from the course structure.
						desc := lesName
						for _, mod := range course.Modules {
							if mod.Name == modName {
								for _, les := range mod.Lessons {
									if les.Name == lesName {
										desc = les.Title
										if len(course.Modules) > 1 {
											desc = fmt.Sprintf("%s (%s)", les.Title, mod.Title)
										}
										break
									}
								}
								break
							}
						}
						completions = append(completions, fmt.Sprintf("%s\t%s", lesName, desc))
					}
				}
			}
			return completions, noFileComp

		default:
			return nil, noFileComp
		}
	}
}

// --- Content completions ---

var contentKinds = []string{
	"challenge\tChallenge content",
	"tutorial\tTutorial content",
	"course\tCourse content",
	"skill-path\tSkill path content",
	"training\tTraining content",
	"vendor\tVendor content",
}

// ContentArgs completes content kind (first arg) and authored content names (second arg).
// Use for: content pull, push, remove.
func ContentArgs(cli labcli.CLI) CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return contentKinds, noFileComp

		case 1:
			if cli.Client() == nil {
				return nil, noFileComp
			}
			return completeAuthoredContentNames(cmd, cli, args[0])

		default:
			return nil, noFileComp
		}
	}
}

// ContentCreateArgs completes only the content kind (first arg) for the create command,
// since the second arg is a new name chosen by the user.
func ContentCreateArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, noFileComp
	}
	return contentKinds, noFileComp
}

func completeAuthoredContentNames(cmd *cobra.Command, cli labcli.CLI, kind string) ([]string, cobra.ShellCompDirective) {
	var k content.ContentKind
	if err := k.Set(kind); err != nil {
		return nil, noFileComp
	}

	var completions []string

	switch k {
	case content.KindChallenge:
		items, err := cli.Client().ListAuthoredChallenges(cmd.Context())
		if err != nil {
			return nil, noFileComp
		}
		for _, c := range items {
			completions = append(completions, fmt.Sprintf("%s\t%s", c.Name, c.Title))
		}

	case content.KindTutorial:
		items, err := cli.Client().ListAuthoredTutorials(cmd.Context())
		if err != nil {
			return nil, noFileComp
		}
		for _, t := range items {
			completions = append(completions, fmt.Sprintf("%s\t%s", t.Name, t.Title))
		}

	case content.KindCourse:
		items, err := cli.Client().ListAuthoredCourses(cmd.Context())
		if err != nil {
			return nil, noFileComp
		}
		for _, c := range items {
			completions = append(completions, fmt.Sprintf("%s\t%s", c.Name, c.Title))
		}

	case content.KindSkillPath:
		items, err := cli.Client().ListAuthoredSkillPaths(cmd.Context())
		if err != nil {
			return nil, noFileComp
		}
		for _, s := range items {
			completions = append(completions, fmt.Sprintf("%s\t%s", s.Name, s.Title))
		}

	case content.KindTraining:
		items, err := cli.Client().ListAuthoredTrainings(cmd.Context())
		if err != nil {
			return nil, noFileComp
		}
		for _, t := range items {
			completions = append(completions, fmt.Sprintf("%s\t%s", t.Name, t.Title))
		}

	case content.KindRoadmap:
		items, err := cli.Client().ListAuthoredRoadmaps(cmd.Context())
		if err != nil {
			return nil, noFileComp
		}
		for _, r := range items {
			completions = append(completions, fmt.Sprintf("%s\t%s", r.Name, r.Title))
		}

	case content.KindVendor:
		items, err := cli.Client().ListAuthoredVendors(cmd.Context())
		if err != nil {
			return nil, noFileComp
		}
		for _, v := range items {
			completions = append(completions, fmt.Sprintf("%s\t%s", v.Name, v.Title))
		}
	}

	return completions, noFileComp
}
