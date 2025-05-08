package api

import (
	"context"

	"github.com/iximiuz/labctl/content"
)

type Training struct {
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt string `json:"updatedAt" yaml:"updatedAt"`

	Name  string `json:"name" yaml:"name"`
	Title string `json:"title" yaml:"title"`

	PageURL string `json:"pageUrl" yaml:"pageUrl"`

	Authors []Author `json:"authors" yaml:"authors"`
}

var _ content.Content = (*Training)(nil)

func (t *Training) GetKind() content.ContentKind {
	return content.KindTraining
}

func (t *Training) GetName() string {
	return t.Name
}

func (t *Training) GetPageURL() string {
	return t.PageURL
}

func (t *Training) IsOfficial() bool {
	for _, author := range t.Authors {
		if !author.Official {
			return false
		}
	}
	return len(t.Authors) > 0
}

func (t *Training) IsAuthoredBy(userID string) bool {
	for _, a := range t.Authors {
		if a.UserID == userID {
			return true
		}
	}
	return false
}

type CreateTrainingRequest struct {
	Name   string `json:"name"`
	Sample bool   `json:"sample"`
}

func (c *Client) CreateTraining(ctx context.Context, req CreateTrainingRequest) (*Training, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var t Training
	return &t, c.PostInto(ctx, "/trainings", nil, nil, body, &t)
}

func (c *Client) GetTraining(ctx context.Context, name string) (*Training, error) {
	var t Training
	return &t, c.GetInto(ctx, "/trainings/"+name, nil, nil, &t)
}

func (c *Client) ListTrainings(ctx context.Context) ([]Training, error) {
	var trainings []Training
	return trainings, c.GetInto(ctx, "/trainings", nil, nil, &trainings)
}

func (c *Client) ListAuthoredTrainings(ctx context.Context) ([]Training, error) {
	var trainings []Training
	return trainings, c.GetInto(ctx, "/author/trainings", nil, nil, &trainings)
}

func (c *Client) DeleteTraining(ctx context.Context, name string) error {
	resp, err := c.Delete(ctx, "/trainings/"+name, nil, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
