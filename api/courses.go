package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/iximiuz/labctl/content"
)

type CourseModule struct {
	Name    string         `json:"name" yaml:"name"`
	Title   string         `json:"title" yaml:"title"`
	Slug    string         `json:"slug" yaml:"slug"`
	Path    string         `json:"path" yaml:"path"`
	Lessons []CourseLesson `json:"lessons" yaml:"lessons"`
}

type CourseLesson struct {
	Name       string              `json:"name" yaml:"name"`
	Title      string              `json:"title" yaml:"title"`
	Slug       string              `json:"slug" yaml:"slug"`
	Path       string              `json:"path" yaml:"path"`
	Playground *CoursePlayground   `json:"playground" yaml:"playground"`
	Tasks      map[string]PlayTask `json:"tasks,omitempty" yaml:"tasks,omitempty"`
}

type CoursePlayground struct {
	Name string `json:"name" yaml:"name"`
}

type CourseLearning struct {
	Modules map[string]CourseLearningModule `json:"modules" yaml:"modules"`
}

type CourseLearningModule struct {
	Lessons map[string]CourseLearningLesson `json:"lessons" yaml:"lessons"`
	Started bool                            `json:"started" yaml:"started"`
}

type CourseLearningLesson struct {
	Started bool   `json:"started" yaml:"started"`
	Play    string `json:"play,omitempty" yaml:"play,omitempty"`
}

type Course struct {
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt string `json:"updatedAt" yaml:"updatedAt"`

	Name  string `json:"name" yaml:"name"`
	Title string `json:"title" yaml:"title"`

	PageURL string `json:"pageUrl" yaml:"pageUrl"`

	Authors []Author `json:"authors" yaml:"authors"`

	Modules  []CourseModule  `json:"modules,omitempty" yaml:"modules,omitempty"`
	Learning *CourseLearning `json:"learning,omitempty" yaml:"learning,omitempty"`
}

var _ content.Content = (*Course)(nil)

func (c *Course) GetKind() content.ContentKind {
	return content.KindCourse
}

func (c *Course) GetName() string {
	return c.Name
}

func (c *Course) GetPageURL() string {
	return c.PageURL
}

func (c *Course) IsOfficial() bool {
	for _, author := range c.Authors {
		if !author.Official {
			return false
		}
	}
	return len(c.Authors) > 0
}

func (c *Course) IsAuthoredBy(userID string) bool {
	for _, a := range c.Authors {
		if a.UserID == userID {
			return true
		}
	}
	return false
}

// FindLesson resolves a lesson by module hint and lesson hint (name or slug).
// If moduleHint is empty, all modules are searched. Returns the resolved module name,
// lesson name, and lesson pointer, or an error if not found or ambiguous.
func (c *Course) FindLesson(moduleHint, lessonHint string) (string, string, *CourseLesson, error) {
	type match struct {
		moduleName string
		lessonName string
		lesson     *CourseLesson
	}

	var matches []match

	for i := range c.Modules {
		mod := &c.Modules[i]

		if moduleHint != "" && mod.Name != moduleHint && mod.Slug != moduleHint {
			continue
		}

		for j := range mod.Lessons {
			les := &mod.Lessons[j]

			if les.Name == lessonHint || les.Slug == lessonHint {
				matches = append(matches, match{
					moduleName: mod.Name,
					lessonName: les.Name,
					lesson:     les,
				})
			}
		}
	}

	if len(matches) == 0 {
		if moduleHint != "" {
			return "", "", nil, fmt.Errorf("lesson %q not found in module %q", lessonHint, moduleHint)
		}
		return "", "", nil, fmt.Errorf("lesson %q not found in any module", lessonHint)
	}

	if len(matches) > 1 {
		var desc []string
		for _, m := range matches {
			desc = append(desc, fmt.Sprintf("  - module %q, lesson %q", m.moduleName, m.lessonName))
		}
		return "", "", nil, fmt.Errorf("ambiguous lesson %q, found in multiple modules:\n%s\n\nUse --module to disambiguate", lessonHint, strings.Join(desc, "\n"))
	}

	return matches[0].moduleName, matches[0].lessonName, matches[0].lesson, nil
}

type CourseVariant string

const (
	CourseVariantSimple  CourseVariant = "simple"
	CourseVariantModular CourseVariant = "modular"
)

type CreateCourseRequest struct {
	Name    string        `json:"name"`
	Variant CourseVariant `json:"variant"`
	Sample  bool          `json:"sample"`
}

func (c *Client) CreateCourse(ctx context.Context, req CreateCourseRequest) (*Course, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var course Course
	return &course, c.PostInto(ctx, "/courses", nil, nil, body, &course)
}

func (c *Client) GetCourse(ctx context.Context, name string) (*Course, error) {
	var course Course
	return &course, c.GetInto(ctx, "/courses/"+name, nil, nil, &course)
}

func (c *Client) ListCourses(ctx context.Context) ([]Course, error) {
	var courses []Course
	return courses, c.GetInto(ctx, "/courses", nil, nil, &courses)
}

func (c *Client) ListAuthoredCourses(ctx context.Context) ([]Course, error) {
	var courses []Course
	return courses, c.GetInto(ctx, "/author/courses", nil, nil, &courses)
}

func (c *Client) DeleteCourse(ctx context.Context, name string) error {
	resp, err := c.Delete(ctx, "/courses/"+name, nil, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

type StartCourseLessonOptions struct {
	SafetyDisclaimerConsent bool
	AsFreeTierUser          bool
}

func (c *Client) StartCourseLesson(ctx context.Context, courseName, moduleName, lessonName string, opts StartCourseLessonOptions) (*Course, error) {
	req := map[string]any{
		"safetyDisclaimerConsent": opts.SafetyDisclaimerConsent,
		"asFreeTierUser":          opts.AsFreeTierUser,
		"modules": map[string]any{
			moduleName: map[string]any{
				"lessons": map[string]any{
					lessonName: map[string]any{
						"started": true,
					},
				},
			},
		},
	}

	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var course Course
	return &course, c.PatchInto(ctx, "/courses/"+courseName+"/learning", nil, nil, body, &course)
}

func (c *Client) StopCourseLesson(ctx context.Context, courseName, moduleName, lessonName string) (*Course, error) {
	req := map[string]any{
		"modules": map[string]any{
			moduleName: map[string]any{
				"lessons": map[string]any{
					lessonName: map[string]any{
						"started": false,
					},
				},
			},
		},
	}

	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var course Course
	return &course, c.PatchInto(ctx, "/courses/"+courseName+"/learning", nil, nil, body, &course)
}
