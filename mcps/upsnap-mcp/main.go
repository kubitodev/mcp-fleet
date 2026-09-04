// upsnap-mcp — a small read + power MCP over UpSnap's PocketBase API.
//
// UpSnap (seriousm4x/UpSnap) is a Wake-on-LAN app on PocketBase. It has no MCP
// upstream, so this is a first-party one built by kubitodev/mcp-fleet.
//
// Auth: PocketBase has NO static API token — you auth-with-password to get a
// short-lived JWT. So the server logs in at startup with UPSNAP_MCP_USER /
// UPSNAP_MCP_PASSWORD (against UPSNAP_MCP_AUTH_COLLECTION, default "users") and
// re-auths transparently on a 401. The endpoint itself is guarded by a separate
// bearer (UPSNAP_MCP_AUTH_TOKEN), since power tools can turn machines off.
package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const version = "0.1.0"

func main() {
	base := strings.TrimRight(mustEnv("UPSNAP_URL"), "/")
	c := &Client{
		base:     base,
		http:     &http.Client{Timeout: 20 * time.Second},
		authColl: envOr("UPSNAP_MCP_AUTH_COLLECTION", "users"),
		user:     mustEnv("UPSNAP_MCP_USER"),
		pass:     mustEnv("UPSNAP_MCP_PASSWORD"),
	}
	endpointToken := os.Getenv("UPSNAP_MCP_AUTH_TOKEN")
	if endpointToken == "" {
		log.Println("WARNING: UPSNAP_MCP_AUTH_TOKEN is unset — the /mcp endpoint is UNAUTHENTICATED")
	}
	readOnly := envTrue("UPSNAP_MCP_READ_ONLY")
	addr := envOr("UPSNAP_MCP_HTTP_ADDR", ":8080")

	s := server.NewMCPServer("upsnap-mcp", version,
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)
	registerReadTools(s, c)
	if !readOnly {
		registerPowerTools(s, c)
	} else {
		log.Println("read-only mode: power tools not registered")
	}

	// Stateless: single replica, no session affinity — hermes reaches it as a plain URL.
	streamable := server.NewStreamableHTTPServer(s,
		server.WithEndpointPath("/mcp"),
		server.WithStateLess(true),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/mcp", bearerAuth(endpointToken, streamable))

	log.Printf("upsnap-mcp %s listening on %s (upstream %s, read_only=%v)", version, addr, base, readOnly)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}

// ── HTTP endpoint auth ──────────────────────────────────────────────────────

// bearerAuth guards the /mcp endpoint with a shared token. hermes sends
// "Authorization: Bearer <token>"; the "Bearer " prefix is optional here.
func bearerAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" {
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ── UpSnap / PocketBase client ──────────────────────────────────────────────

type Client struct {
	base     string
	http     *http.Client
	authColl string
	user     string
	pass     string

	mu    sync.Mutex
	token string
}

// authenticate logs in with identity+password and caches the JWT.
func (c *Client) authenticate(ctx context.Context) error {
	payload, _ := json.Marshal(map[string]string{"identity": c.user, "password": c.pass})
	url := c.base + "/api/collections/" + c.authColl + "/auth-with-password"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("upsnap auth failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("upsnap auth: decode response: %w", err)
	}
	if out.Token == "" {
		return fmt.Errorf("upsnap auth: empty token in response")
	}
	c.mu.Lock()
	c.token = out.Token
	c.mu.Unlock()
	return nil
}

// request does a GET against the UpSnap API, authenticating on first use and
// retrying once on a 401 (expired JWT). Returns the body and status code.
func (c *Client) request(ctx context.Context, method, path string) ([]byte, int, error) {
	c.mu.Lock()
	tok := c.token
	c.mu.Unlock()
	if tok == "" {
		if err := c.authenticate(ctx); err != nil {
			return nil, 0, err
		}
		c.mu.Lock()
		tok = c.token
		c.mu.Unlock()
	}

	do := func(tok string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, method, c.base+path, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", tok) // PocketBase accepts the raw token
		return c.http.Do(req)
	}

	resp, err := do(tok)
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		if err := c.authenticate(ctx); err != nil {
			return nil, 0, err
		}
		c.mu.Lock()
		tok = c.token
		c.mu.Unlock()
		if resp, err = do(tok); err != nil {
			return nil, 0, err
		}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

// listRecords returns the raw records of a PocketBase collection.
func (c *Client) listRecords(ctx context.Context, collection string) ([]map[string]any, error) {
	body, code, err := c.request(ctx, http.MethodGet,
		"/api/collections/"+collection+"/records?perPage=500&sort=name")
	if err != nil {
		return nil, err
	}
	if code/100 != 2 {
		return nil, fmt.Errorf("list %s failed (%d): %s", collection, code, strings.TrimSpace(string(body)))
	}
	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

// power invokes an UpSnap power route (wake/shutdown/reboot/sleep and the group
// variants). id is a device id (or group id for the *group actions).
func (c *Client) power(ctx context.Context, action, id string) error {
	body, code, err := c.request(ctx, http.MethodGet, "/api/upsnap/"+action+"/"+id)
	if err != nil {
		return err
	}
	if code/100 != 2 {
		return fmt.Errorf("%s failed (%d): %s", action, code, strings.TrimSpace(string(body)))
	}
	return nil
}

// resolve finds a record in a collection by exact id or case-insensitive name.
func (c *Client) resolve(ctx context.Context, collection, ident string) (map[string]any, error) {
	items, err := c.listRecords(ctx, collection)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		if str(it["id"]) == ident {
			return it, nil
		}
	}
	var matches []map[string]any
	for _, it := range items {
		if strings.EqualFold(str(it["name"]), ident) {
			matches = append(matches, it)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no %s matching %q (try upsnap_list_%s)", strings.TrimSuffix(collection, "s"), ident, collection)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("%q is ambiguous in %s; pass the id instead", ident, collection)
	}
}

// ── tools ───────────────────────────────────────────────────────────────────

var deviceFields = []string{"id", "name", "status", "ip", "mac", "netmask", "description", "groups", "link", "ports"}
var groupFields = []string{"id", "name", "description", "devices"}

func registerReadTools(s *server.MCPServer, c *Client) {
	s.AddTool(mcp.NewTool("upsnap_list_devices",
		mcp.WithDescription("List all UpSnap devices with their current status (online/offline/pending), IP, MAC, and group membership."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	), func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		items, err := c.listRecords(ctx, "devices")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(pick(items, deviceFields))
	})

	s.AddTool(mcp.NewTool("upsnap_get_device",
		mcp.WithDescription("Get one UpSnap device by id or name, including its live status."),
		mcp.WithString("device", mcp.Required(), mcp.Description("Device id or name")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ident, err := r.RequireString("device")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		dev, err := c.resolve(ctx, "devices", ident)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(pickOne(dev, deviceFields))
	})

	s.AddTool(mcp.NewTool("upsnap_list_groups",
		mcp.WithDescription("List UpSnap device groups (used by the group wake/shutdown actions)."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	), func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		items, err := c.listRecords(ctx, "device_groups")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(pick(items, groupFields))
	})
}

func registerPowerTools(s *server.MCPServer, c *Client) {
	// Single-device power actions. wake turns a machine ON (non-destructive);
	// shutdown/reboot/sleep change a running machine's state (destructive hint).
	deviceAction(s, c, "upsnap_wake", "wake",
		"Send a Wake-on-LAN magic packet to power ON a device.", false)
	deviceAction(s, c, "upsnap_shutdown", "shutdown",
		"Shut DOWN a device (runs its configured shutdown command).", true)
	deviceAction(s, c, "upsnap_reboot", "reboot",
		"Reboot a device (runs its configured reboot command).", true)
	deviceAction(s, c, "upsnap_sleep", "sleep",
		"Put a device to SLEEP (runs its configured sleep command).", true)

	// Group actions operate on every device in a device group.
	groupAction(s, c, "upsnap_wake_group", "wakegroup",
		"Wake every device in a group (Wake-on-LAN to all members).", false)
	groupAction(s, c, "upsnap_shutdown_group", "shutdowngroup",
		"Shut down every device in a group.", true)
}

func deviceAction(s *server.MCPServer, c *Client, name, action, desc string, destructive bool) {
	s.AddTool(mcp.NewTool(name,
		mcp.WithDescription(desc),
		mcp.WithString("device", mcp.Required(), mcp.Description("Device id or name")),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(destructive),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ident, err := r.RequireString("device")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		dev, err := c.resolve(ctx, "devices", ident)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := c.power(ctx, action, str(dev["id"])); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("%s sent to %q", action, str(dev["name"]))), nil
	})
}

func groupAction(s *server.MCPServer, c *Client, name, action, desc string, destructive bool) {
	s.AddTool(mcp.NewTool(name,
		mcp.WithDescription(desc),
		mcp.WithString("group", mcp.Required(), mcp.Description("Group id or name")),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(destructive),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ident, err := r.RequireString("group")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		grp, err := c.resolve(ctx, "device_groups", ident)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := c.power(ctx, action, str(grp["id"])); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("%s sent to group %q", action, str(grp["name"]))), nil
	})
}

// ── helpers ─────────────────────────────────────────────────────────────────

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

// pick projects each record down to the given fields (present ones only).
func pick(items []map[string]any, fields []string) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, pickOne(it, fields))
	}
	return out
}

func pickOne(it map[string]any, fields []string) map[string]any {
	m := make(map[string]any, len(fields))
	for _, f := range fields {
		if v, ok := it[f]; ok {
			m[f] = v
		}
	}
	return m
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("required env %s is not set", k)
	}
	return v
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envTrue(k string) bool {
	switch strings.ToLower(os.Getenv(k)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
