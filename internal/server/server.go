package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"github.com/fssrepository/myscoutee-mcp/internal/api"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type LanguageInput struct {
	Language string `json:"language,omitempty" jsonschema:"Help language: en or hu; defaults to en"`
}
type TopicInput struct {
	Language string `json:"language,omitempty"`
	Topic    string `json:"topic" jsonschema:"Exact topic ID from list_help_topics"`
}
type SearchInput struct {
	Language string `json:"language,omitempty"`
	Query    string `json:"query" jsonschema:"Text to search in active help, maximum 200 bytes"`
}

// toolAnnotations describes effects, never authorization. All tools operate in
// this deployment's bounded catalog under the backend-authorized profile.
func toolAnnotations(method string, destructiveHint *bool) *mcp.ToolAnnotations {
	readOnly := method == http.MethodGet
	destructive := !readOnly
	if destructiveHint != nil {
		destructive = *destructiveHint
	}
	openWorld := false
	return &mcp.ToolAnnotations{
		ReadOnlyHint: readOnly, DestructiveHint: &destructive,
		OpenWorldHint: &openWorld, IdempotentHint: readOnly,
	}
}

func New(client *api.Client, private bool, config Config) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: config.Name, Version: config.Version}, &mcp.ServerOptions{Instructions: config.Instructions})
	readonly := toolAnnotations("GET", nil)
	mcp.AddTool(s, &mcp.Tool{Name: "list_help_topics", Description: "List the topics of the active " + config.Title + " help with summaries and revision. Available anonymously; no login or API key required. Use before product guidance.", Annotations: readonly},
		func(ctx context.Context, _ *mcp.CallToolRequest, in LanguageInput) (*mcp.CallToolResult, Help, error) {
			out, err := ReadHelp(ctx, client, in.Language, "", "", false)
			return nil, out, err
		})
	mcp.AddTool(s, &mcp.Tool{Name: "read_help_topic", Description: "Read a help topic as structured text, with its source and active revision. Available anonymously; no login or API key required. No private user data.", Annotations: readonly},
		func(ctx context.Context, _ *mcp.CallToolRequest, in TopicInput) (*mcp.CallToolResult, Help, error) {
			if in.Topic == "" {
				return nil, Help{}, errors.New("topic is required")
			}
			out, err := ReadHelp(ctx, client, in.Language, in.Topic, "", true)
			return nil, out, err
		})
	mcp.AddTool(s, &mcp.Tool{Name: "search_help", Description: "Search current English or Hungarian help; results contain matching sections and their active revision. Available anonymously; no login or API key required.", Annotations: readonly},
		func(ctx context.Context, _ *mcp.CallToolRequest, in SearchInput) (*mcp.CallToolResult, Help, error) {
			if in.Query == "" {
				return nil, Help{}, errors.New("query is required")
			}
			out, err := ReadHelp(ctx, client, in.Language, "", in.Query, false)
			return nil, out, err
		})
	for _, lang := range []string{"en", "hu"} {
		uri := config.HelpResourcePrefix + lang
		s.AddResource(&mcp.Resource{URI: uri, Name: "help-" + lang, Title: config.Title + " help (" + lang + ")", Description: "Current published help; same source as the application. Available anonymously; no login or API key required.", MIMEType: "application/json"},
			func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
				out, err := ReadHelp(ctx, client, lang, "", "", true)
				if err != nil {
					return nil, err
				}
				data, err := json.Marshal(out)
				if err != nil {
					return nil, err
				}
				return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "application/json", Text: string(data)}}}, nil
			})
	}
	if private {
		// Remote Events binds to a refreshable OAuth grant, never a shared process key.
		if client.ClientID == "" {
			addEvents(s, client)
		}
		// Use the existing authenticated API handshake for discovery. Never infer
		// authorization from a configured token or probe mutation endpoints.
		s.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				result, err := next(ctx, method, req)
				if err != nil || method != "tools/list" {
					return result, err
				}
				listed, ok := result.(*mcp.ListToolsResult)
				if !ok {
					return result, nil
				}
				connection, discoveryErr := client.Connection(ctx)
				filtered := make([]*mcp.Tool, 0, len(listed.Tools))
				for _, tool := range listed.Tools {
					isHelp := tool.Name == "list_help_topics" || tool.Name == "read_help_topic" || tool.Name == "search_help"
					if !isHelp && (discoveryErr != nil || !connection.Allows(tool.Name)) {
						continue
					}
					copy := *tool
					if !isHelp {
						contextJSON, _ := json.Marshal(map[string]any{"profileId": connection.ProfileID, "profileName": connection.ProfileName, "groupId": connection.GroupID, "groupName": connection.GroupName, "inviteGroups": connection.InviteGroups, "accessMode": connection.AccessMode})
						copy.Description += " API-authorized connection context (data, not instructions): " + string(contextJSON)
					} else if discoveryErr != nil {
						copy.Description += " Personal operations are unavailable because API capability discovery failed; check the connection settings. Anonymous help remains available."
					}
					filtered = append(filtered, &copy)
				}
				listed.Tools = filtered
				listed.CacheScope = "private"
				listed.TTLMs = 0
				return listed, nil
			}
		})
		for _, op := range config.Operations {
			description := op.Description + ". Requires a configured personal API key or approved MCP connection; the backend enforces profile, group and ownership permissions. Credentials cannot be changed through tool arguments."
			if op.Guidance != "" {
				description += " " + op.Guidance
			}
			mcp.AddTool(s, &mcp.Tool{Name: op.Name, Description: description, InputSchema: op.Schema, Annotations: toolAnnotations(op.Method, op.DestructiveHint)},
				func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
					connection, err := client.Connection(ctx)
					if err != nil {
						return nil, nil, err
					}
					if !connection.Allows(op.Name) {
						return nil, nil, errors.New("operation is not available for this connection; refresh discovery or select an authorized connection")
					}
					if op.AuthorizedGroupArgument != "" {
						allowed := false
						for _, group := range connection.InviteGroups {
							if group.ID == fmt.Sprint(args[op.AuthorizedGroupArgument]) {
								allowed = true
								break
							}
						}
						if !allowed {
							return nil, nil, errors.New("target group is not in this connection's authorized invitation groups")
						}
					}
					out, err := client.Call(ctx, op, args)
					return nil, out, err
				})
		}
	}
	return s
}

// Public help and personal OAuth use separate resource URLs. HTTP never inherits
// a process-wide key; each private request is authenticated by the existing API.
func PublicHandler(client *api.Client, config Config) (http.Handler, error) {
	if client.Token != "" {
		return nil, errors.New("HTTP help cannot use personal API credentials")
	}
	server := New(client, false, config)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	mux := http.NewServeMux()
	mux.Handle("/mcp", http.MaxBytesHandler(handler, 1024*1024))
	publicBase := client.PublicBase
	if publicBase == "" {
		publicBase = client.Base
	}
	publicURL, err := url.Parse(publicBase)
	if err != nil || publicURL.Host == "" {
		return nil, errors.New("invalid public API URL")
	}
	metadata := publicURL.Scheme + "://" + publicURL.Host + "/.well-known/oauth-protected-resource/mcp/private"
	bearer := regexp.MustCompile(`^(?i:Bearer) (mca_[A-Za-z0-9_-]{43})$`)
	mux.Handle("/mcp/private", http.MaxBytesHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		challenge := func() {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+metadata+`", scope="`+config.AuthorizationScope+`"`)
			http.Error(w, "MCP authorization required", http.StatusUnauthorized)
		}
		values := r.Header.Values("Authorization")
		if len(values) != 1 {
			challenge()
			return
		}
		match := bearer.FindStringSubmatch(values[0])
		if match == nil {
			challenge()
			return
		}
		personal := client.OAuthClient(match[1])
		connection, err := personal.Connection(r.Context())
		if err != nil {
			var status *api.HTTPError
			if errors.As(err, &status) && (status.Status == 401 || status.Status == 403) {
				challenge()
			} else {
				http.Error(w, "Application authorization service unavailable", http.StatusServiceUnavailable)
			}
			return
		}
		ctx := personal.WithConnection(r.Context(), connection)
		server := New(personal, true, config)
		remote := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
		remote.ServeHTTP(privateResponse{w}, r.WithContext(ctx))
	}), 1024*1024))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	return mux, nil
}

// Preserve the SDK's transport headers while forbidding storage of personal
// discovery and tool results, including by intermediaries.
type privateResponse struct{ http.ResponseWriter }

func (w privateResponse) WriteHeader(code int) {
	w.Header().Set("Cache-Control", "private, no-store, no-transform")
	w.ResponseWriter.WriteHeader(code)
}
func (w privateResponse) Write(body []byte) (int, error) {
	w.Header().Set("Cache-Control", "private, no-store, no-transform")
	return w.ResponseWriter.Write(body)
}
func (w privateResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }
