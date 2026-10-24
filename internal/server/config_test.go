package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fssrepository/myscoutee-mcp/internal/api"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func testConfig(t testing.TB) Config {
	t.Helper()
	config, err := LoadConfig("testdata")
	if err != nil {
		t.Fatal(err)
	}
	return config
}

func TestConfiguredProductsKeepIdentityAndCatalogsIndependent(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/integrations/v1/connect" {
			t.Error("discovery probed a data operation")
		}
		fmt.Fprint(w, `{"connected":true,"profileId":"owner","operations":["alpha","beta"],"inviteGroups":[]}`)
	}))
	defer upstream.Close()
	client, _ := api.NewClient(upstream.URL, "token", "client")
	for _, name := range []string{"alpha", "beta", "alpha"} {
		config := testConfig(t)
		config.Name, config.Title, config.Instructions = name, name, name+" instructions"
		config.HelpResourcePrefix = name + "://help/"
		config.Operations = []api.Operation{{Name: name, Method: "GET", Path: "/" + name, Schema: map[string]any{"type": "object"}}}
		a, b := mcp.NewInMemoryTransports()
		server, err := New(client, true, config).Connect(context.Background(), a, nil)
		if err != nil {
			t.Fatal(err)
		}
		session, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(context.Background(), b, nil)
		if err != nil {
			t.Fatal(err)
		}
		initialized := session.InitializeResult()
		if initialized.ServerInfo.Name != name || initialized.Instructions != config.Instructions {
			t.Fatal("product identity or instructions crossed configuration boundaries")
		}
		tools, err := session.ListTools(context.Background(), nil)
		if err != nil || len(tools.Tools) != 4 {
			t.Fatalf("configured catalog: %+v %v", tools, err)
		}
		for _, tool := range tools.Tools {
			if tool.Name != name && tool.Name != "list_help_topics" && tool.Name != "read_help_topic" && tool.Name != "search_help" {
				t.Fatal("another product's operation was registered")
			}
		}
		resources, err := session.ListResources(context.Background(), nil)
		if err != nil || len(resources.Resources) != 2 {
			t.Fatal(resources, err)
		}
		for _, resource := range resources.Resources {
			if !strings.HasPrefix(resource.URI, config.HelpResourcePrefix) {
				t.Fatal("another product's help URI was registered")
			}
		}
		session.Close()
		server.Close()
	}
}

func TestConfigRejectsMissingMalformedAndAmbiguousInputs(t *testing.T) {
	if _, err := LoadConfig(""); err == nil {
		t.Fatal("missing product selection was accepted")
	}
	for _, test := range []struct{ name, product, contract string }{
		{"unknown product field", `{"unknown":true}`, `[]`},
		{"incomplete product", `{}`, `[]`},
		{"unsafe authorization scope", `{"name":"test","title":"Test","version":"1","helpResourcePrefix":"test://help/","instructions":"test","authorizationScope":"test\r"}`, `[]`},
		{"missing catalog", "", `null`},
		{"trailing JSON", "", `[] {}`},
		{"reserved tool", "", `[{"name":"search_help"}]`},
		{"invalid method", "", `[{"name":"test","method":"CONNECT"}]`},
		{"external path", "", `[{"name":"test","method":"GET","path":"https://other.example","schema":{"type":"object"}}]`},
		{"unknown contract field", "", `[{"name":"test","extra":true}]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			product := []byte(test.product)
			if test.product == "" {
				var err error
				product, err = json.Marshal(testConfig(t))
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "product.json"), product, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "contract.json"), []byte(test.contract), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadConfig(dir); err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
}
