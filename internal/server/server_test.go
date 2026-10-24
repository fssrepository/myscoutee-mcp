package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fssrepository/myscoutee-mcp/internal/api"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func fixture(t *testing.T) (*api.Client, *atomic.Int32) {
	t.Helper()
	version := &atomic.Int32{}
	version.Store(2)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/help/active" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-MyScoutee-Client-Id") != "" {
			t.Error("private credentials leaked to help")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"activeRevision":{"id":"help-%d","version":%d,"lang":%q,"active":true,"title":"Help","sections":[{"id":"groups","title":"Csoportok / Groups","blurb":"Profiles","contentHtml":"<p>Use the selector.</p><ul><li>Keep permissions.</li></ul><script>SECRET</script>"},{"id":"assets","title":"Assets","blurb":"Base profile","contentHtml":"<p>Global discovery.</p>"}]},"auditTrail":[{"private":"must not appear"}]}`, version.Load(), version.Load(), r.URL.Query().Get("lang"))
	}))
	t.Cleanup(upstream.Close)
	client, err := api.NewClient(upstream.URL, "", "")
	if err != nil {
		t.Fatal(err)
	}
	return client, version
}
func TestHelpUsesActiveRevisionAndSanitizedStructuredSections(t *testing.T) {
	client, version := fixture(t)
	out, err := ReadHelp(context.Background(), client, "hu", "groups", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Sections) != 1 || out.Language != "hu" || strings.Contains(out.Sections[0].Text, "SECRET") || !strings.Contains(out.Sections[0].Text, "• Keep permissions.") {
		t.Fatalf("unexpected help: %+v", out)
	}
	version.Store(3)
	newer, err := ReadHelp(context.Background(), client, "en", "", "", false)
	if err != nil || newer.Version != 3 || newer.Sections[0].Text != "" {
		t.Fatalf("stale/full catalog: %+v %v", newer, err)
	}
	result, err := ReadHelp(context.Background(), client, "hu", "", "csoport", false)
	if err != nil || len(result.Sections) != 1 {
		t.Fatalf("search: %+v %v", result, err)
	}
	empty, err := ReadHelp(context.Background(), client, "en", "", "no match", false)
	if err != nil || len(empty.Sections) != 0 {
		t.Fatal("empty search must remain empty")
	}
	for _, item := range []struct{ lang, topic, query string }{{"de", "", ""}, {"hu", "missing", ""}, {"en", "", strings.Repeat("a", 201)}} {
		if _, err := ReadHelp(context.Background(), client, item.lang, item.topic, item.query, false); err == nil {
			t.Fatal("invalid help input accepted")
		}
	}
}
func TestMCPHTTPDiscoveryCallResourceAndNoPrivateTools(t *testing.T) {
	client, _ := fixture(t)
	handler, err := PublicHandler(client, testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	remote := httptest.NewServer(handler)
	defer remote.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: remote.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 3 {
		t.Fatalf("public tools: %+v %v", tools, err)
	}
	if !strings.Contains(session.InitializeResult().Instructions, "Anonymous help is available without login or an API key") {
		t.Fatal("initialization does not explain anonymous help")
	}
	for _, tool := range tools.Tools {
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint || tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
			t.Fatalf("help annotations must explicitly describe a bounded read: %s", tool.Name)
		}
		if !strings.Contains(tool.Description, "Available anonymously; no login or API key required") {
			t.Fatalf("discovery omits anonymous access for %s", tool.Name)
		}
	}
	resources, err := session.ListResources(ctx, nil)
	if err != nil || len(resources.Resources) != 2 {
		t.Fatalf("help resources: %+v %v", resources, err)
	}
	for _, resource := range resources.Resources {
		if !strings.Contains(resource.Description, "Available anonymously; no login or API key required") {
			t.Fatalf("discovery omits anonymous access for %s", resource.URI)
		}
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "read_help_topic", Arguments: map[string]any{"language": "hu", "topic": "groups"}})
	if err != nil || result.IsError {
		t.Fatalf("call: %+v %v", result, err)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	if !strings.Contains(string(raw), "revisionId") || strings.Contains(string(raw), "must not appear") {
		t.Fatalf("bad result %s", raw)
	}
	res, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "test://help/en"})
	if err != nil || len(res.Contents) != 1 || !strings.Contains(res.Contents[0].Text, "Global discovery.") {
		t.Fatalf("resource: %+v %v", res, err)
	}
	_, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "createEvents", Arguments: map[string]any{}})
	if err == nil {
		t.Fatal("private operation exposed")
	}
	client.Token = "private"
	if _, err = PublicHandler(client, testConfig(t)); err == nil {
		t.Fatal("HTTP must reject shared API identity")
	}
}
func TestMissingRevisionAndHTTPErrorDoNotFallback(t *testing.T) {
	for _, body := range []string{`{"activeRevision":null}`, `{"activeRevision":{"id":"x","version":1,"active":false,"lang":"en"}}`, `not json`} {
		t.Run(body, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, body) }))
			defer upstream.Close()
			client, _ := api.NewClient(upstream.URL, "", "")
			if _, err := ReadHelp(context.Background(), client, "en", "", "", true); err == nil {
				t.Fatal("invalid revision accepted")
			}
		})
	}
}
func TestPrivateToolsValidateConfiguredSchemasBeforeSending(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/integrations/v1/connect" {
			writeCapabilities(w)
			return
		}
		calls.Add(1)
		fmt.Fprint(w, `{"items":[]}`)
	}))
	defer upstream.Close()
	client, _ := api.NewClient(upstream.URL, "token", "client")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a, b := mcp.NewInMemoryTransports()
	serverSession, err := New(client, true, testConfig(t)).Connect(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 9 {
		t.Fatalf("private tools: %v %v", tools, err)
	}
	for _, tool := range tools.Tools {
		if tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || tool.Annotations.OpenWorldHint == nil {
			t.Fatalf("missing explicit review annotations: %s", tool.Name)
		}
		if tool.Name == "createEvents" && (tool.Annotations.ReadOnlyHint || *tool.Annotations.DestructiveHint || *tool.Annotations.OpenWorldHint) {
			t.Fatal("event import must describe its additive write within the profile")
		}
		if tool.Name == "createEvents" && !strings.Contains(tool.Description, "For a different group use a connection with that group's profile key") {
			t.Fatal("event discovery omits group-key requirements")
		}
		if tool.Name == "createAssets" && !strings.Contains(tool.Description, "Requires a base-profile key") {
			t.Fatal("asset discovery omits base-profile requirement")
		}
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "createAssets", Arguments: map[string]any{"request": map[string]any{"items": []any{}}}})
	if err == nil && !result.IsError {
		t.Fatal("empty batch accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("invalid input reached API")
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "event", Arguments: map[string]any{"externalId": "my-event"}})
	if err != nil || result.IsError {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("valid call not sent")
	}
}

func TestPrivateToolsRespectAPIDenialsAndCannotOverrideIdentity(t *testing.T) {
	var calls atomic.Int32
	var status atomic.Int32
	status.Store(http.StatusForbidden)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer configured-key" || r.Header.Get("X-MyScoutee-Client-Id") != "configured-client" {
			t.Error("configured API identity was changed")
		}
		if r.URL.Path == "/integrations/v1/connect" {
			writeCapabilities(w)
			return
		}
		calls.Add(1)
		w.WriteHeader(int(status.Load()))
		fmt.Fprint(w, `{"private":"sensitive backend detail"}`)
	}))
	defer upstream.Close()
	client, err := api.NewClient(upstream.URL, "configured-key", "configured-client")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a, b := mcp.NewInMemoryTransports()
	serverSession, err := New(client, true, testConfig(t)).Connect(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, field := range []string{"token", "clientId", "userId", "groupId", "headers", "url"} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "event", Arguments: map[string]any{"externalId": "foreign-event", field: "override"}})
		if err == nil && !result.IsError {
			t.Fatalf("accepted authority override: %s", field)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("authority override reached API")
	}
	for _, code := range []int{http.StatusForbidden, http.StatusUnauthorized} {
		status.Store(int32(code))
		before := calls.Load()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "event", Arguments: map[string]any{"externalId": "foreign-event"}})
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsError {
			t.Fatalf("API %d became success", code)
		}
		encoded, _ := json.Marshal(result)
		if !strings.Contains(string(encoded), fmt.Sprintf("HTTP %d", code)) || strings.Contains(string(encoded), "sensitive backend detail") || strings.Contains(string(encoded), "configured-key") {
			t.Fatalf("unsafe denial response: %s", encoded)
		}
		if calls.Load() != before+1 {
			t.Fatal("denial triggered a retry or alternate API request")
		}
	}
}

func writeCapabilities(w http.ResponseWriter) {
	fmt.Fprint(w, `{"connected":true,"profileId":"profile","groupId":"group","groupName":"Team","operations":["event","encounters","createAssets","createEvents","inviteParticipants","inviteGroupParticipants"],"inviteGroups":[{"id":"group","name":"Team"}]}`)
}

func TestDiscoveryUsesLiveAPICapabilitiesAndFailsClosed(t *testing.T) {
	var phase atomic.Int32
	var writes atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/integrations/v1/connect" {
			writes.Add(1)
			t.Error("discovery or forbidden call probed an operation")
			return
		}
		switch phase.Load() {
		case 0:
			fmt.Fprint(w, `{"connected":true,"profileId":"team-profile","groupId":"g1","groupName":"Team One","accessMode":"full","operations":["createEvents","inviteGroupParticipants","listRatings"],"inviteGroups":[{"id":"g1","name":"Team One"}]}`)
		case 4:
			fmt.Fprint(w, `{"connected":true,"profileId":"team-profile","accessMode":"write","operations":["createEvents","inviteGroupParticipants"],"inviteGroups":[]}`)
		case 1:
			fmt.Fprint(w, `{"connected":true,"profileId":"team-profile","groupId":"g1","operations":[],"inviteGroups":[]}`)
		case 2:
			w.WriteHeader(http.StatusUnauthorized)
		case 3:
			fmt.Fprint(w, `{"connected":true,"maxBatchSize":1000}`)
		}
	}))
	defer upstream.Close()
	client, _ := api.NewClient(upstream.URL, "token", "client")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a, b := mcp.NewInMemoryTransports()
	ss, err := New(client, true, testConfig(t)).Connect(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	listed, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 6 {
		t.Fatalf("unexpected advertised tools: %+v", listed.Tools)
	}
	if listed.CacheScope != "private" || listed.TTLMs != 0 {
		t.Fatal("personal discovery must not be publicly cached")
	}
	for _, tool := range listed.Tools {
		if tool.Name == "createAssets" || tool.Name == "event" {
			t.Fatal("unavailable tool advertised")
		}
		if tool.Name == "createEvents" && !strings.Contains(tool.Description, `"groupId":"g1"`) {
			t.Fatal("missing API group context")
		}
	}
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "inviteGroupParticipants", Arguments: map[string]any{"groupId": "other", "request": map[string]any{"participantIds": []string{"p1"}}}})
	if err != nil || !result.IsError {
		t.Fatalf("unauthorized target: %+v %v", result, err)
	}
	phase.Store(4)
	listed, err = cs.ListTools(ctx, nil)
	if err != nil || len(listed.Tools) != 5 {
		t.Fatalf("Full-to-Write downgrade retained a read: %+v %v", listed, err)
	}
	result, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "listRatings", Arguments: map[string]any{}})
	if err != nil || !result.IsError {
		t.Fatalf("a stale Full read survived downgrade: %+v %v", result, err)
	}
	for _, nextPhase := range []int32{1, 2, 3} {
		phase.Store(nextPhase)
		listed, err = cs.ListTools(ctx, nil)
		if err != nil || len(listed.Tools) != 3 {
			t.Fatalf("stale/legacy capabilities: %+v %v", listed, err)
		}
		result, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "event", Arguments: map[string]any{"externalId": "e1"}})
		if err != nil || !result.IsError {
			t.Fatalf("revoked operation accepted: %+v %v", result, err)
		}
	}
	if writes.Load() != 0 {
		t.Fatal("probe reached data operation")
	}
}

// A single shared HTTP process serves independent OAuth identities. Discovery
// and mutations both traverse the same backend permission boundary each time.
func TestRemoteOAuthIsolationRevocationAndAnonymousHelp(t *testing.T) {
	var revoked atomic.Bool
	var handshakes atomic.Int32
	tokenA, tokenB := "mca_"+strings.Repeat("a", 43), "mca_"+strings.Repeat("b", 43)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/help/active" {
			if r.Header.Get("Authorization") != "" {
				t.Error("help received credential")
			}
			fmt.Fprint(w, `{"activeRevision":{"id":"h","version":2,"lang":"en","active":true,"sections":[]}}`)
			return
		}
		if r.Header.Get("X-MyScoutee-Client-Id") != "" {
			t.Error("OAuth must not invent a client header")
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token != tokenA && token != tokenB || token == tokenA && revoked.Load() {
			w.WriteHeader(401)
			return
		}
		if r.URL.Path == "/integrations/v1/connect" {
			handshakes.Add(1)
			if token == tokenA {
				fmt.Fprint(w, `{"connected":true,"profileId":"alice","groupId":"team","operations":["event"],"inviteGroups":[]}`)
			} else {
				fmt.Fprint(w, `{"connected":true,"profileId":"bob","operations":[],"inviteGroups":[]}`)
			}
			return
		}
		if token != tokenA {
			t.Error("unauthorized mutation reached backend")
		}
		fmt.Fprint(w, `{"id":"alice-event"}`)
	}))
	defer upstream.Close()
	apiClient, _ := api.NewClient(upstream.URL, "", "")
	apiClient.PublicBase = "https://myscoutee.example/api"
	handler, err := PublicHandler(apiClient, testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	remote := httptest.NewServer(handler)
	defer remote.Close()
	call := func(token, method, args string) (int, string, http.Header) {
		req, _ := http.NewRequest("POST", remote.URL+"/mcp/private", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"`+method+`","params":`+args+`}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-11-25")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(body), res.Header
	}
	for _, token := range []string{"", "msc_api_key", "mca_invalid"} {
		code, _, h := call(token, "tools/list", `{}`)
		if code != 401 || !strings.Contains(h.Get("WWW-Authenticate"), `https://myscoutee.example/.well-known/oauth-protected-resource/mcp/private`) {
			t.Fatal("missing OAuth challenge", code, h)
		}
	}
	for _, token := range []string{tokenA, tokenB, tokenA} {
		before := handshakes.Load()
		code, body, h := call(token, "tools/list", `{}`)
		if code != 200 || !strings.Contains(h.Get("Cache-Control"), "no-store") {
			t.Fatal(code, body)
		}
		if handshakes.Load() != before+1 {
			t.Fatal("handshake repeated or missing")
		}
		if token == tokenA && (!strings.Contains(body, `alice`) || strings.Contains(body, `bob`)) {
			t.Fatal("wrong Alice context", body)
		}
		if token == tokenB && (strings.Contains(body, `alice`) || strings.Contains(body, `"name":"event"`)) {
			t.Fatal("Alice capability leaked to Bob", body)
		}
	}
	code, body, _ := call(tokenB, "tools/call", `{"name":"event","arguments":{"externalId":"e"}}`)
	if code != 200 || !strings.Contains(body, `"isError":true`) {
		t.Fatal(code, body)
	}
	code, body, _ = call(tokenA, "tools/call", `{"name":"event","arguments":{"externalId":"e"}}`)
	if code != 200 || !strings.Contains(body, "alice-event") {
		t.Fatal(code, body)
	}
	code, body, _ = call(tokenA, "tools/call", `{"name":"list_help_topics","arguments":{}}`)
	if code != 200 || strings.Contains(body, `"isError":true`) {
		t.Fatal(code, body)
	}
	revoked.Store(true)
	code, _, _ = call(tokenA, "tools/list", `{}`)
	if code != 401 {
		t.Fatal("revoked token accepted", code)
	}
	code, _, _ = call(tokenB, "tools/list", `{}`)
	if code != 200 {
		t.Fatal("revocation crossed identities", code)
	}
	if apiClient.Token != "" {
		t.Fatal("process identity mutated")
	}
}

func TestCampaignCreationIsDiscoverableOnlyForWorkAndUsesItsConnection(t *testing.T) {
	for _, work := range []bool{false, true} {
		t.Run(fmt.Sprint(work), func(t *testing.T) {
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer profile-key" {
					t.Error("profile key was not preserved")
				}
				if r.URL.Path == "/integrations/v1/connect" {
					if work {
						fmt.Fprint(w, `{"connected":true,"profileId":"work-profile","groupId":"myscoutee-work","operations":["createCampaigns"],"inviteGroups":[]}`)
					} else {
						fmt.Fprint(w, `{"connected":true,"profileId":"dating-profile","operations":[],"inviteGroups":[]}`)
					}
					return
				}
				if r.URL.Path != "/integrations/v1/campaigns" || r.Method != "POST" {
					t.Errorf("unexpected operation: %s %s", r.Method, r.URL.Path)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if len(body["items"].([]any)) != 1 {
					t.Error("missing campaign")
				}
				calls.Add(1)
				fmt.Fprint(w, `{"items":[]}`)
			}))
			defer upstream.Close()
			client, _ := api.NewClient(upstream.URL, "profile-key", "client")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			a, b := mcp.NewInMemoryTransports()
			ss, err := New(client, true, testConfig(t)).Connect(ctx, a, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer ss.Close()
			cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, b, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer cs.Close()
			list, err := cs.ListTools(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, tool := range list.Tools {
				if tool.Name == "createCampaigns" {
					found = true
				}
			}
			if found != work {
				t.Fatalf("campaign visibility = %v, work = %v", found, work)
			}
			result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "createCampaigns", Arguments: map[string]any{"request": map[string]any{"items": []any{map[string]any{"id": "b8186d5d-e222-4e22-9b84-486918a35b11", "title": "Build a team", "description": "Collaborate", "kind": "business", "category": "technology"}}}}})
			if work && (err != nil || result.IsError || calls.Load() != 1) {
				t.Fatalf("campaign call: %+v %v calls=%d", result, err, calls.Load())
			}
			if !work && ((err == nil && !result.IsError) || calls.Load() != 0) {
				t.Fatal("Dating key reached campaign creation")
			}
		})
	}
}
