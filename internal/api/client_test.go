package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientKeepsIdentityAndAPIPathAndNeverAddsGroupHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/integrations/v1/events/test-event" || r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("X-MyScoutee-Client-Id") != "client" {
			t.Errorf("wrong path or identity")
		}
		for name := range r.Header {
			if strings.Contains(strings.ToLower(name), "group") {
				t.Error("group override header")
			}
		}
		fmt.Fprint(w, `{"id":"test-event"}`)
	}))
	defer upstream.Close()
	c, err := NewClient(upstream.URL+"/api", "secret", "client")
	if err != nil {
		t.Fatal(err)
	}
	op := testOperation()
	if _, err = c.Call(context.Background(), op, map[string]any{"externalId": "test-event"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"..", "/other", "a\\b"} {
		if _, err = c.Call(context.Background(), op, map[string]any{"externalId": id}); err == nil {
			t.Fatal("unsafe path accepted")
		}
	}
}
func TestRedirectsAndErrorsDoNotExposeCredentials(t *testing.T) {
	hit := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer target.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer upstream.Close()
	c, _ := NewClient(upstream.URL, "secret", "client")
	_, err := c.Call(context.Background(), testOperation(), map[string]any{"externalId": "event"})
	if err == nil || hit || strings.Contains(err.Error(), "secret") {
		t.Fatal("unsafe redirect/error")
	}
}
func TestRejectUnsafeBaseURLsAndPartialCredentials(t *testing.T) {
	for _, base := range []string{"https://user:secret@example.com", "https://example.com?x=1", "file:///etc/passwd"} {
		if _, err := NewClient(base, "", ""); err == nil {
			t.Fatalf("accepted %s", base)
		}
	}
	for _, base := range []string{"http://localhost.attacker.test", "http://profilemanagement:8080"} {
		if _, err := NewClient(base, "token", "client"); err == nil {
			t.Fatal("credentials allowed over non-loopback HTTP")
		}
	}
	if _, err := NewClient("http://profilemanagement:8080", "", ""); err != nil {
		t.Fatal("anonymous internal help rejected", err)
	}
	if _, err := NewClient("https://example.com/api", "token", ""); err == nil {
		t.Fatal("partial identity accepted")
	}
}

func testOperation() Operation {
 return Operation{Name: "item", Method: "GET", Path: "/integrations/v1/events/{externalId}", Parameters: []Parameter{{Name: "externalId", In: "path"}}}
}
