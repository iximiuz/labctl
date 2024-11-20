package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var (
	ErrAuthenticationRequired = errors.New("authentication required")
	ErrGatewayTimeout         = errors.New("gateway timeout")
)

func isAuthenticationRequiredResponse(resp *http.Response) bool {
	return resp.StatusCode == http.StatusUnauthorized
}

func isGatewayTimeoutResponse(resp *http.Response) bool {
	return resp.StatusCode == http.StatusGatewayTimeout
}

type Client struct {
	baseURL    string
	apiBaseURL string

	sessionID   string
	accessToken string

	userAgent string

	httpClient *http.Client
}

type ClientOptions struct {
	BaseURL     string
	APIBaseURL  string
	SessionID   string
	AccessToken string
	UserAgent   string
}

func NewClient(opts ClientOptions) *Client {
	return &Client{
		baseURL:     opts.BaseURL,
		apiBaseURL:  opts.APIBaseURL,
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
	req, err := c.newRequest(ctx, http.MethodGet, c.apiBaseURL+path, query, headers, nil)
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
	req, err := c.newRequest(ctx, http.MethodGet, c.apiBaseURL+path, query, headers, nil)
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

func (c *Client) Patch(
	ctx context.Context,
	path string,
	query url.Values,
	headers http.Header,
	body io.Reader,
) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodPatch, c.apiBaseURL+path, query, headers, body)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req)
}

func (c *Client) PatchInto(
	ctx context.Context,
	path string,
	query url.Values,
	headers http.Header,
	body io.Reader,
	into any,
) error {
	req, err := c.newRequest(ctx, http.MethodPatch, c.apiBaseURL+path, query, headers, body)
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
	req, err := c.newRequest(ctx, http.MethodPost, c.apiBaseURL+path, query, headers, body)
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
	req, err := c.newRequest(ctx, http.MethodPost, c.apiBaseURL+path, query, headers, body)
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
	req, err := c.newRequest(ctx, http.MethodPut, c.apiBaseURL+path, query, headers, body)
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
	req, err := c.newRequest(ctx, http.MethodPut, c.apiBaseURL+path, query, headers, body)
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
	req, err := c.newRequest(ctx, http.MethodDelete, c.apiBaseURL+path, query, headers, nil)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req)
}

func (c *Client) Download(
	ctx context.Context,
	path string,
	query url.Values,
	headers http.Header,
	dest io.Writer,
) error {
	req, err := c.newRequest(
		ctx, http.MethodGet,
		c.baseURL+path,
		query, headers, nil,
	)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}

	if _, err := io.Copy(dest, resp.Body); err != nil {
		return err
	}

	return nil
}

func (c *Client) DownloadTo(
	ctx context.Context,
	path string,
	query url.Values,
	headers http.Header,
	file string,
) error {
	dest, err := os.Create(file)
	if err != nil {
		return err
	}
	defer dest.Close()

	return c.Download(ctx, path, query, headers, dest)
}

func (c *Client) Upload(
	ctx context.Context,
	url string,
	src io.Reader,
) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodPut, url, nil, nil, src)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req)
}

func (c *Client) UploadFrom(
	ctx context.Context,
	url string,
	file string,
) (*http.Response, error) {
	src, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer src.Close()

	// FIX+HACK: This is a hacky way of ensuring the Content-Length header is set.
	// This is necessary because Tigris seems to expect the Content-Length header to be set.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, src); err != nil {
		return nil, err
	}

	return c.Upload(ctx, url, &buf)
}

func (c *Client) newRequest(
	ctx context.Context,
	method,
	url string,
	query url.Values,
	headers http.Header,
	body io.Reader,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	if headers == nil {
		headers = make(http.Header)
	}

	req.Header = headers.Clone()
	req.Header.Set("User-Agent", c.userAgent)

	if strings.HasPrefix(url, c.baseURL) || strings.HasPrefix(url, c.apiBaseURL) {
		if c.sessionID != "" && c.accessToken != "" {
			req.Header.Set("Authorization", "Basic "+base64Encode(c.sessionID+":"+c.accessToken))
		}
	}

	if query != nil {
		req.URL.RawQuery = query.Encode()
	}

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

		if isAuthenticationRequiredResponse(resp) {
			return nil, ErrAuthenticationRequired
		}
		if isGatewayTimeoutResponse(resp) {
			return nil, ErrGatewayTimeout
		}

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
