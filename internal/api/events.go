package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// EventError carries only the backend's sanitized protocol category, never an HTTP body.
type EventError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (e *EventError) Error() string { return e.Message }

// Events shares the same request credentials, endpoint and bounded response transport as tools.
func (c *Client) Events(ctx context.Context, action string, params any) (json.RawMessage, error) {
	method, path := http.MethodPost, "/integrations/v1/mcp/events/"+action
	switch action {
	case "list":
		method, path, params = http.MethodGet, "/integrations/v1/mcp/events", nil
	case "subscribe", "unsubscribe":
	default:
		return nil, errors.New("unsupported event method")
	}
	var response struct {
		Result json.RawMessage `json:"result"`
		Error  *EventError     `json:"error"`
	}
	if err := c.request(ctx, method, path, params, true, &response); err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, response.Error
	}
	if len(response.Result) == 0 {
		return nil, errors.New("invalid Events response")
	}
	return response.Result, nil
}
