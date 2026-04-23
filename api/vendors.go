package api

import (
	"context"

	"github.com/iximiuz/labctl/content"
)

type Vendor struct {
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
	UpdatedAt string `json:"updatedAt" yaml:"updatedAt"`

	Name        string   `json:"name" yaml:"name"`
	Title       string   `json:"title" yaml:"title"`
	Description string   `json:"description" yaml:"description"`
	Cover       string   `json:"cover,omitempty" yaml:"cover,omitempty"`
	Categories  []string `json:"categories" yaml:"categories"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`

	Authors []Author `json:"authors" yaml:"authors"`

	PageURL string `json:"pageUrl" yaml:"pageUrl"`
}

var _ content.Content = (*Vendor)(nil)

func (v *Vendor) GetKind() content.ContentKind {
	return content.KindVendor
}

func (v *Vendor) GetName() string {
	return v.Name
}

func (v *Vendor) GetPageURL() string {
	return v.PageURL
}

func (v *Vendor) IsAuthoredBy(userID string) bool {
	for _, a := range v.Authors {
		if a.UserID == userID {
			return true
		}
	}
	return false
}

type CreateVendorRequest struct {
	Name string `json:"name"`
}

func (c *Client) CreateVendor(ctx context.Context, req CreateVendorRequest) (*Vendor, error) {
	body, err := toJSONBody(req)
	if err != nil {
		return nil, err
	}

	var v Vendor
	return &v, c.PostInto(ctx, "/vendors", nil, nil, body, &v)
}

func (c *Client) GetVendor(ctx context.Context, name string) (*Vendor, error) {
	var v Vendor
	return &v, c.GetInto(ctx, "/vendors/"+name, nil, nil, &v)
}

func (c *Client) ListVendors(ctx context.Context) ([]Vendor, error) {
	var vendors []Vendor
	return vendors, c.GetInto(ctx, "/vendors", nil, nil, &vendors)
}

func (c *Client) ListAuthoredVendors(ctx context.Context) ([]Vendor, error) {
	var vendors []Vendor
	return vendors, c.GetInto(ctx, "/author/vendors", nil, nil, &vendors)
}

func (c *Client) DeleteVendor(ctx context.Context, name string) error {
	resp, err := c.Delete(ctx, "/vendors/"+name, nil, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
