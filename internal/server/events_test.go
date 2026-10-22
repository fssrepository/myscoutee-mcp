package server

import (
	"encoding/json"
	"fmt"
	"github.com/fssrepository/myscoutee-mcp/internal/api"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestEventsUseMCP2DiscoveryAndKeepOAuthUsersIsolated(t *testing.T) {
	tokenA, tokenB := "mca_"+strings.Repeat("a", 43), "mca_"+strings.Repeat("b", 43)
	var revoked atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token != tokenA && token != tokenB || token == tokenA && revoked.Load() {
			w.WriteHeader(401)
			return
		}
		if r.Header.Get("X-MyScoutee-Client-Id") != "" {
			t.Error("OAuth acquired an invented client ID")
		}
		if r.URL.Path == "/integrations/v1/connect" {
			fmt.Fprint(w, `{"connected":true,"profileId":"owner","accessMode":"full","operations":[],"inviteGroups":[]}`)
			return
		}
		switch r.URL.Path {
		case "/integrations/v1/mcp/events":
			if token == tokenB {
				fmt.Fprint(w, `{"result":{"events":[]}}`)
			} else {
				fmt.Fprint(w, `{"result":{"events":[{"name":"event.changed"}]}}`)
			}
		case "/integrations/v1/mcp/events/subscribe":
			var params map[string]any
			if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
				t.Error(err)
			}
			if params["name"] != "event.changed" {
				t.Error("subscription params changed")
			}
			if token == tokenB {
				fmt.Fprint(w, `{"error":{"code":-32012,"message":"Event access denied","data":{"reason":"access_denied"}}}`)
			} else {
				fmt.Fprint(w, `{"result":{"id":"alice-sub","refreshBefore":"2026-10-07T10:00:00Z","cursor":null,"truncated":false}}`)
			}
		case "/integrations/v1/mcp/events/unsubscribe":
			fmt.Fprint(w, `{"result":{}}`)
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	client, _ := api.NewClient(upstream.URL, "", "")
	client.PublicBase = "https://app.example/api"
	handler, err := PublicHandler(client, testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	call := func(path, token, method, params string) *httptest.ResponseRecorder {
		var body map[string]any
		if err := json.Unmarshal([]byte(params), &body); err != nil {
			t.Fatal(err)
		}
		body["_meta"] = map[string]any{
			"io.modelcontextprotocol/protocolVersion":    "2026-07-28",
			"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "events-test", "version": "1"},
			"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest("POST", path, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"`+method+`","params":`+string(encoded)+`}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		request.Header.Set("MCP-Protocol-Version", "2026-07-28")
		request.Header.Set("Mcp-Method", method)
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, request)
		return out
	}
	result := call("/mcp/private", tokenA, "server/discover", `{}`)
	if result.Code != 200 || !strings.Contains(result.Body.String(), `"events":{}`) || !strings.Contains(result.Body.String(), "2026-07-28") {
		t.Fatal("Events capability missing", result.Code, result.Body.String())
	}
	anonymous := call("/mcp", "", "server/discover", `{}`)
	if anonymous.Code != 200 || strings.Contains(anonymous.Body.String(), `"events":`) {
		t.Fatal("anonymous Events advertised")
	}
	for _, token := range []string{tokenA, tokenB, tokenA} {
		out := call("/mcp/private", token, "events/list", `{}`)
		if out.Code != 200 || !strings.Contains(out.Header().Get("Cache-Control"), "no-store") {
			t.Fatal(out.Code, out.Body.String())
		}
		if strings.Contains(out.Body.String(), "event.changed") != (token == tokenA) {
			t.Fatal("cross-account event discovery")
		}
	}
	params := `{"name":"event.changed","arguments":{"resourceId":"e"},"delivery":{"mode":"webhook","url":"https://callback.example/a","secret":"not-a-real-secret"}}`
	for _, token := range []string{tokenA, tokenB} {
		out := call("/mcp/private", token, "events/subscribe", params)
		expected := "alice-sub"
		if token == tokenB {
			expected = "-32012"
		}
		if !strings.Contains(out.Body.String(), expected) {
			t.Fatal(out.Code, out.Body.String())
		}
		if strings.Contains(out.Body.String(), "not-a-real-secret") {
			t.Fatal("secret echoed")
		}
	}
	revoked.Store(true)
	out := call("/mcp/private", tokenA, "events/list", `{}`)
	if out.Code != 401 {
		body, _ := io.ReadAll(out.Result().Body)
		t.Fatal("revoked connection remained active", string(body))
	}
}
