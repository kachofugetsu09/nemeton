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
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kachofugetsu09/nemeton/internal/realtime"
)

type Client struct {
	http       *http.Client
	socketPath string
}

func NewClient(socketPath string) *Client {
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	return &Client{http: &http.Client{Transport: transport}, socketPath: socketPath}
}

func (c *Client) CreateMeeting(ctx context.Context, projectID string, request CreateMeetingRequest) (MeetingResponse, error) {
	return c.meetingRequest(ctx, http.MethodPost, "/v1/projects/"+projectID+"/meetings", request)
}

func (c *Client) InspectMeeting(ctx context.Context, meetingID string) (MeetingResponse, error) {
	return c.meetingRequest(ctx, http.MethodGet, "/v1/meetings/"+meetingID, nil)
}

func (c *Client) StartMeeting(ctx context.Context, meetingID string) (MeetingResponse, error) {
	return c.meetingRequest(ctx, http.MethodPost, "/v1/meetings/"+meetingID+"/start", struct{}{})
}

func (c *Client) AnswerMeeting(ctx context.Context, meetingID string, request HumanInputRequest) (MeetingResponse, error) {
	return c.meetingRequest(ctx, http.MethodPost, "/v1/meetings/"+meetingID+"/inputs", request)
}

func (c *Client) RatifyMeeting(ctx context.Context, meetingID string, request RatificationRequest) (MeetingResponse, error) {
	return c.meetingRequest(ctx, http.MethodPost, "/v1/meetings/"+meetingID+"/ratifications", request)
}

func (c *Client) ReviewMeeting(ctx context.Context, meetingID string, request MeetingReviewRequest) (MeetingResponse, error) {
	return c.meetingRequest(ctx, http.MethodPost, "/v1/meetings/"+meetingID+"/reviews", request)
}

func (c *Client) MeetingContent(ctx context.Context, meetingID, contentID string) (ContentResponse, error) {
	var response ContentResponse
	if err := c.do(ctx, http.MethodGet, "/v1/meetings/"+meetingID+"/contents/"+contentID, nil, &response); err != nil {
		return ContentResponse{}, err
	}
	return response, nil
}

func (c *Client) MeetingHandoff(ctx context.Context, meetingID string) (HandoffResponse, error) {
	var response HandoffResponse
	if err := c.do(ctx, http.MethodGet, "/v1/meetings/"+meetingID+"/handoff", nil, &response); err != nil {
		return HandoffResponse{}, err
	}
	return response, nil
}

func (c *Client) meetingRequest(ctx context.Context, method, path string, input any) (MeetingResponse, error) {
	var response MeetingResponse
	if err := c.do(ctx, method, path, input, &response); err != nil {
		return MeetingResponse{}, err
	}
	return response, nil
}

func (c *Client) WatchMeeting(ctx context.Context, meetingID string, after int64) (*websocket.Conn, error) {
	dialer := websocket.Dialer{NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", c.socketPath)
	}}
	connection, response, err := dialer.DialContext(ctx,
		"ws://nemetond/v1/meetings/"+meetingID+"/ws?after_sequence="+strconv.FormatInt(after, 10), nil)
	if err != nil {
		if response != nil {
			defer response.Body.Close()
			var problem Problem
			if json.NewDecoder(response.Body).Decode(&problem) == nil {
				return nil, &problem
			}
		}
		return nil, fmt.Errorf("connect Meeting WebSocket: %w", err)
	}
	return connection, nil
}

func ReadWatchMessage(connection *websocket.Conn) (realtime.Message, error) {
	var message realtime.Message
	if err := connection.ReadJSON(&message); err != nil {
		return realtime.Message{}, err
	}
	return message, nil
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

func (c *Client) CurrentState(ctx context.Context, projectID string) (CurrentStateResponse, error) {
	var response CurrentStateResponse
	if err := c.do(ctx, http.MethodGet, "/v1/projects/"+projectID+"/current-state", nil, &response); err != nil {
		return CurrentStateResponse{}, err
	}
	return response, nil
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
