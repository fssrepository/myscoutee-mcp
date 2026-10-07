// Package api is a transport adapter for the existing MyScoutee integration API.
// User/profile/group permissions remain entirely in the backend.
package api

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

//go:embed contract.json
var contract []byte

type Parameter struct {
	Name string `json:"name"`
	In   string `json:"in"`
}
type Operation struct {
	Name        string         `json:"name"`
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
	Parameters  []Parameter    `json:"parameters"`
}

func Operations() []Operation {
	var result []Operation
	if err := json.Unmarshal(contract, &result); err != nil {
		panic("invalid built-in API contract")
	}
	return result
}

type Client struct {
	Base       string
	PublicBase string
	Token      string
	ClientID   string
	HTTP       *http.Client
}

type HTTPError struct{ Status int }

func (e *HTTPError) Error() string {
	return fmt.Sprintf("MyScoutee API returned HTTP %d; check access and retry only when appropriate", e.Status)
}

type GroupScope struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Connection struct {
	Connected    bool         `json:"connected"`
	ProfileID    string       `json:"profileId"`
	ProfileName  string       `json:"profileName"`
	GroupID      string       `json:"groupId"`
	GroupName    string       `json:"groupName"`
	AccessMode   string       `json:"accessMode"`
	Operations   []string     `json:"operations"`
	InviteGroups []GroupScope `json:"inviteGroups"`
}

type connectionKey struct{}
type verifiedConnection struct {
	token      string
	connection Connection
}

// WithConnection reuses only this HTTP request's verified handshake. It is never
// shared with another request, credential or MCP session.
func (c *Client) WithConnection(ctx context.Context, connection Connection) context.Context {
	return context.WithValue(ctx, connectionKey{}, verifiedConnection{c.Token, connection})
}

// OAuthClient binds a fresh adapter to the already configured backend address.
// A caller cannot override that address or inherit a process-wide identity.
func (c *Client) OAuthClient(token string) *Client {
	copy := *c
	copy.Token, copy.ClientID = token, ""
	return &copy
}

func (c *Client) Connection(ctx context.Context) (Connection, error) {
	if verified, ok := ctx.Value(connectionKey{}).(verifiedConnection); ok && verified.token == c.Token {
		return verified.connection, nil
	}
	var result Connection
	if c.Token == "" {
		return result, errors.New("personal API credentials are not configured")
	}
	err := c.request(ctx, http.MethodPost, "/integrations/v1/connect", nil, true, &result)
	if err != nil {
		return Connection{}, err
	}
	if !result.Connected || result.ProfileID == "" || result.Operations == nil || result.InviteGroups == nil {
		return Connection{}, errors.New("API connection does not provide capability discovery")
	}
	return result, nil
}

func (c Connection) Allows(operation string) bool {
	for _, allowed := range c.Operations {
		if allowed == operation {
			return true
		}
	}
	return false
}

func NewClient(base, token, clientID string) (*Client, error) {
	parsed, err := url.Parse(base)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Scheme != "https" && !(parsed.Scheme == "http" && (token == "" || parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "::1"))) {
		return nil, errors.New("API base requires HTTP(S) without credentials/query/fragment; personal credentials require HTTPS except on loopback")
	}
	if (token == "") != (clientID == "") {
		return nil, errors.New("API token and client ID must be configured together")
	}
	return &Client{Base: strings.TrimRight(base, "/"), Token: token, ClientID: clientID, HTTP: &http.Client{
		Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}
func (c *Client) GetHelp(ctx context.Context, lang string, target any) error {
	return c.request(ctx, http.MethodGet, "/help/active?lang="+url.QueryEscape(lang), nil, false, target)
}
func (c *Client) Call(ctx context.Context, op Operation, args map[string]any) (any, error) {
	if c.Token == "" {
		return nil, errors.New("personal API credentials are not configured")
	}
	path := op.Path
	query := url.Values{}
	for _, p := range op.Parameters {
		value, ok := args[p.Name]
		if !ok {
			continue
		}
		s := fmt.Sprint(value)
		if p.In == "path" {
			if s == "" || s == "." || s == ".." || strings.ContainsAny(s, "/\\") {
				return nil, errors.New("invalid resource identifier")
			}
			path = strings.ReplaceAll(path, "{"+p.Name+"}", url.PathEscape(s))
		} else if p.In == "query" {
			query.Set(p.Name, s)
		}
	}
	if strings.Contains(path, "{") {
		return nil, errors.New("missing resource identifier")
	}
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	var result any
	err := c.request(ctx, op.Method, "/integrations/v1"+path, args["request"], true, &result)
	return result, err
}
func (c *Client) request(ctx context.Context, method, path string, body any, private bool, target any) error {
	var input io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return errors.New("invalid request")
		}
		input = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, input)
	if err != nil {
		return errors.New("invalid API request")
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if private {
		req.Header.Set("Authorization", "Bearer "+c.Token)
		if c.ClientID != "" {
			req.Header.Set("X-MyScoutee-Client-Id", c.ClientID)
		}
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return errors.New("MyScoutee API is unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		// Do not expose raw backend errors, tokens, or personal data in exception text.
		return &HTTPError{Status: res.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024+1))
	if err != nil || len(data) > 2*1024*1024 {
		return errors.New("unable to read bounded API response")
	}
	if err = json.Unmarshal(data, target); err != nil {
		return errors.New("invalid API response")
	}
	return nil
}
