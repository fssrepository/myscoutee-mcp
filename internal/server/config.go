package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fssrepository/myscoutee-mcp/internal/api"
)

// Config belongs to the consuming application. It is loaded once at startup,
// then shared read-only by anonymous and independently authorized connections.
type Config struct {
	Name               string          `json:"name"`
	Title              string          `json:"title"`
	Version            string          `json:"version"`
	HelpResourcePrefix string          `json:"helpResourcePrefix"`
	Instructions       string          `json:"instructions"`
	AuthorizationScope string          `json:"authorizationScope"`
	Operations         []api.Operation `json:"-"`
}

func LoadConfig(directory string) (Config, error) {
	var config Config
	if directory == "" {
		return config, fmt.Errorf("MCP_CONFIG_DIR must contain product.json and contract.json")
	}
	if err := readConfig(filepath.Join(directory, "product.json"), &config); err != nil {
		return config, err
	}
	if config.Name == "" || config.Title == "" || config.Version == "" || config.Instructions == "" || config.AuthorizationScope == "" {
		return config, fmt.Errorf("product.json requires name, title, version, instructions and authorizationScope")
	}
	if strings.ContainsAny(config.AuthorizationScope, "\r\n\"\\") {
		return config, fmt.Errorf("product.json authorizationScope contains invalid header characters")
	}
	prefix, err := url.Parse(config.HelpResourcePrefix)
	if err != nil || prefix.Scheme == "" || prefix.RawQuery != "" || prefix.Fragment != "" || !strings.HasSuffix(config.HelpResourcePrefix, "/") {
		return config, fmt.Errorf("product.json requires an absolute helpResourcePrefix ending in /")
	}
	if err := readConfig(filepath.Join(directory, "contract.json"), &config.Operations); err != nil {
		return config, err
	}
	if config.Operations == nil {
		return config, fmt.Errorf("contract.json must be an operation array; use [] for help only")
	}
	names := map[string]bool{"list_help_topics": true, "read_help_topic": true, "search_help": true}
	validName := regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
	for _, op := range config.Operations {
		if !validName.MatchString(op.Name) || names[op.Name] {
			return config, fmt.Errorf("contract.json has a duplicate, reserved or invalid operation name: %q", op.Name)
		}
		names[op.Name] = true
		switch op.Method {
		case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			return config, fmt.Errorf("contract.json has an unsupported method for %q", op.Name)
		}
		path, err := url.Parse(op.Path)
		if err != nil || !strings.HasPrefix(op.Path, "/") || path.Host != "" || path.RawQuery != "" || path.Fragment != "" || op.Schema["type"] != "object" {
			return config, fmt.Errorf("contract.json requires a relative API path and object input schema for %q", op.Name)
		}
		if op.AuthorizedGroupArgument != "" {
			properties, ok := op.Schema["properties"].(map[string]any)
			if !ok || properties[op.AuthorizedGroupArgument] == nil {
				return config, fmt.Errorf("contract.json references a missing authorizedGroupArgument for %q", op.Name)
			}
		}
	}
	return config, nil
}

func readConfig(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	defer file.Close()
	const limit = 2 << 20
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || len(data) > limit {
		return fmt.Errorf("unable to read bounded %s", filepath.Base(path))
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid %s: %w", filepath.Base(path), err)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return fmt.Errorf("unexpected trailing content in %s", filepath.Base(path))
	}
	return nil
}
