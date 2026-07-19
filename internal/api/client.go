package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type Client struct {
	http *http.Client
}

func NewClient(socketPath string) *Client {
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	return &Client{http: &http.Client{Transport: transport}}
}

func (c *Client) Health(ctx context.Context) (Health, error) {
	var health Health
	if err := c.do(ctx, http.MethodGet, "/v1/health", nil, &health); err != nil {
		return Health{}, err
	}
	return health, nil
}

func (c *Client) Open(ctx context.Context, request OpenRequest) (ProjectResponse, error) {
	return c.projectRequest(ctx, http.MethodPost, "/v1/projects/open", request)
}

func (c *Client) Inspect(ctx context.Context, projectID string) (ProjectResponse, error) {
	return c.projectRequest(ctx, http.MethodGet, "/v1/projects/"+projectID, nil)
}

func (c *Client) Relink(ctx context.Context, projectID string, request RelinkRequest) (ProjectResponse, error) {
	return c.projectRequest(ctx, http.MethodPost, "/v1/projects/"+projectID+"/relink", request)
}

func (c *Client) Replay(ctx context.Context, projectID string) (ProjectResponse, error) {
	return c.projectRequest(ctx, http.MethodPost, "/v1/projects/"+projectID+"/replay", struct{}{})
}

func (c *Client) projectRequest(ctx context.Context, method, path string, input any) (ProjectResponse, error) {
	var response ProjectResponse
	if err := c.do(ctx, method, path, input, &response); err != nil {
		return ProjectResponse{}, err
	}
	return response, nil
}

func (c *Client) do(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode API request: %w", err)
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, "http://nemetond"+path, body)
	if err != nil {
		return fmt.Errorf("create API request: %w", err)
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		var networkError *net.OpError
		if errors.As(err, &networkError) {
			return &Problem{Code: "daemon_unavailable", Title: "Daemon unavailable", Detail: "cannot connect to nemetond; start it with `nemeton daemon start`", Status: 0}
		}
		return fmt.Errorf("call nemetond: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		var problem Problem
		if err := json.NewDecoder(response.Body).Decode(&problem); err != nil {
			return fmt.Errorf("decode daemon error response: %w", err)
		}
		return &problem
	}
	if err := json.NewDecoder(response.Body).Decode(output); err != nil {
		return fmt.Errorf("decode daemon response: %w", err)
	}
	return nil
}
