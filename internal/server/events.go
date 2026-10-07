package server

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/fssrepository/myscoutee-mcp/internal/api"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type eventParams struct {
	mcp.ParamsBase
	Name      string         `json:"name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Delivery  *eventDelivery `json:"delivery,omitempty"`
	TTLMs     *int64         `json:"ttlMs,omitempty"`
	Cursor    *string        `json:"cursor,omitempty"`
}
type eventDelivery struct {
	Mode   string `json:"mode"`
	URL    string `json:"url"`
	Secret string `json:"secret,omitempty"`
}
type eventResult struct {
	mcp.ResultBase
	payload json.RawMessage
}

func (r *eventResult) MarshalJSON() ([]byte, error) { return r.payload, nil }

// The SDK implements MCP 2.0 and custom method dispatch. Events is still a draft
// extension, so only its method types and extra capability need this adapter.
func addEvents(s *mcp.Server, client *api.Client) {
	for _, action := range []string{"list", "subscribe", "unsubscribe"} {
		err := mcp.AddReceivingCustomMethod(s, "events/"+action,
			func(ctx context.Context, _ *mcp.ServerSession, params *eventParams) (*eventResult, error) {
				if params == nil {
					params = &eventParams{}
				}
				if action == "list" && params.Cursor != nil && *params.Cursor != "" {
					return nil, &jsonrpc.Error{Code: -32602, Message: "The event catalog has no additional pages"}
				}
				payload, err := client.Events(ctx, action, params)
				if err != nil {
					var eventError *api.EventError
					if errors.As(err, &eventError) {
						return nil, &jsonrpc.Error{Code: int64(eventError.Code), Message: eventError.Message, Data: eventError.Data}
					}
					return nil, err
				}
				return &eventResult{payload: payload}, nil
			})
		if err != nil {
			panic(err)
		} // Static method names must not shadow standard SDK methods.
	}
	s.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			result, err := next(ctx, method, request)
			if err != nil || method != "server/discover" {
				return result, err
			}
			payload, err := json.Marshal(result)
			if err != nil {
				return nil, err
			}
			var discovery map[string]any
			if err = json.Unmarshal(payload, &discovery); err != nil {
				return nil, err
			}
			capabilities, ok := discovery["capabilities"].(map[string]any)
			if !ok {
				return nil, errors.New("SDK did not return discovery capabilities")
			}
			capabilities["events"] = map[string]any{}
			discovery["cacheScope"], discovery["ttlMs"] = "private", 0
			payload, err = json.Marshal(discovery)
			return &eventResult{payload: payload}, err
		}
	})
}
