package api

import (
	"context"

	"github.com/iximiuz/labctl/internal/content"
)

type Course struct {
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt string `json:"updatedAt" yaml:"updatedAt"`

	Name string `json:"name" yaml:"name"`

	PageURL string `json:"pageUrl" yaml:"pageUrl"`
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

type CourseVariant string

const (
	CourseVariantSimple  CourseVariant = "simple"
	CourseVariantModular CourseVariant = "modular"
)

type CreateCourseRequest struct {
	Name    string        `json:"name"`
	Variant CourseVariant `json:"variant"`
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
	return courses, c.GetInto(ctx, "/courses/authored", nil, nil, &courses)
}

func (c *Client) DeleteCourse(ctx context.Context, name string) error {
	resp, err := c.Delete(ctx, "/courses/"+name, nil, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
