package api

import (
	"context"
	"net/url"

	"github.com/iximiuz/labctl/content"
)

type FeedbackParticipant struct {
	DisplayName string `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	AvatarURL   string `json:"avatarUrl,omitempty" yaml:"avatarUrl,omitempty"`
	Mine        bool   `json:"mine" yaml:"mine"`
}

type FeedbackAttachment struct {
	Name string `json:"name" yaml:"name"`
	Type string `json:"type" yaml:"type"`
	Size int64  `json:"size" yaml:"size"`
}

type FeedbackMessage struct {
	ID          string               `json:"id" yaml:"id"`
	Role        string               `json:"role" yaml:"role"`
	From        FeedbackParticipant  `json:"from" yaml:"from"`
	Body        string               `json:"body" yaml:"body"`
	CreatedAt   string               `json:"createdAt" yaml:"createdAt"`
	Location    string               `json:"location,omitempty" yaml:"location,omitempty"`
	Attachments []FeedbackAttachment `json:"attachments,omitempty" yaml:"attachments,omitempty"`
}

type FeedbackThread struct {
	ID            string              `json:"id" yaml:"id"`
	Role          string              `json:"role,omitempty" yaml:"role,omitempty"`
	Reporter      FeedbackParticipant `json:"reporter" yaml:"reporter"`
	CreatedAt     string              `json:"createdAt" yaml:"createdAt"`
	LastMessageAt string              `json:"lastMessageAt" yaml:"lastMessageAt"`
	MessageCount  int                 `json:"messageCount" yaml:"messageCount"`
	Unread        bool                `json:"unread" yaml:"unread"`
	Messages      []FeedbackMessage   `json:"messages,omitempty" yaml:"messages,omitempty"`
}

type ContentFeedback struct {
	Role      string           `json:"role" yaml:"role"`
	CanReport bool             `json:"canReport" yaml:"canReport"`
	Threads   []FeedbackThread `json:"threads" yaml:"threads"`
}

func (c *Client) ListContentFeedback(ctx context.Context, kind content.ContentKind, name string) (*ContentFeedback, error) {
	query := url.Values{
		"kind": []string{kind.String()},
		"name": []string{name},
	}
	var feedback ContentFeedback
	return &feedback, c.GetInto(ctx, "/content/feedback", query, nil, &feedback)
}

func (c *Client) GetContentFeedbackThread(ctx context.Context, id string) (*FeedbackThread, error) {
	var thread FeedbackThread
	return &thread, c.GetInto(ctx, "/content/feedback/threads/"+url.PathEscape(id), nil, nil, &thread)
}
