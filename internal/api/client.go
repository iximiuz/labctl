package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Client struct {
	baseURL string

	sessionID   string
	accessToken string

	userAgent string

	httpClient *http.Client
}

type ClientOptions struct {
	BaseURL     string
	SessionID   string
	AccessToken string
	UserAgent   string
}

func NewClient(opts ClientOptions) *Client {
	return &Client{
		baseURL:     opts.BaseURL,
		sessionID:   opts.SessionID,
		accessToken: opts.AccessToken,
		userAgent:   opts.UserAgent,
		httpClient:  http.DefaultClient,
	}
}

func (c *Client) SetCredentials(sessionID, accessToken string) {
	c.sessionID = sessionID
	c.accessToken = accessToken
}

func (c *Client) Get(
	ctx context.Context,
	path string,
	query url.Values,
	headers http.Header,
) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, query, headers, nil)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req)
}

func (c *Client) GetInto(
	ctx context.Context,
	path string,
	query url.Values,
	headers http.Header,
	into any,
) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, query, headers, nil)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		return err
	}

	return err
}

func (c *Client) Post(
	ctx context.Context,
	path string,
	query url.Values,
	headers http.Header,
	body io.Reader,
) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodPost, path, query, headers, body)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req)
}

func (c *Client) PostInto(
	ctx context.Context,
	path string,
	query url.Values,
	headers http.Header,
	body io.Reader,
	into any,
) error {
	req, err := c.newRequest(ctx, http.MethodPost, path, query, headers, body)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		return err
	}

	return err
}

func (c *Client) Put(
	ctx context.Context,
	path string,
	query url.Values,
	headers http.Header,
	body io.Reader,
) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodPut, path, query, headers, body)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req)
}

func (c *Client) PutInto(
	ctx context.Context,
	path string,
	query url.Values,
	headers http.Header,
	body io.Reader,
	into any,
) error {
	req, err := c.newRequest(ctx, http.MethodPut, path, query, headers, body)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		return err
	}

	return err
}

func (c *Client) Delete(
	ctx context.Context,
	path string,
	query url.Values,
	headers http.Header,
) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodDelete, path, query, headers, nil)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req)
}

func (c *Client) newRequest(
	ctx context.Context,
	method,
	path string,
	query url.Values,
	headers http.Header,
	body io.Reader,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}

	if headers == nil {
		headers = make(http.Header)
	}

	req.Header = headers.Clone()
	req.Header.Set("User-Agent", c.userAgent)

	if c.sessionID != "" && c.accessToken != "" {
		req.Header.Set("Authorization", "Basic "+base64Encode(c.sessionID+":"+c.accessToken))
	}

	req.URL.RawQuery = query.Encode()

	return req, nil
}

func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, body)
	}

	return resp, nil
}

func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func toJSONBody(req any) (io.Reader, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(req); err != nil {
		return nil, fmt.Errorf("failed to encode request body: %w", err)
	}
	return &buf, nil
}
