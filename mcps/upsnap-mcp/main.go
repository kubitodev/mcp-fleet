// upsnap-mcp — a small read + power MCP over UpSnap's PocketBase API.
//
// UpSnap (seriousm4x/UpSnap) is a Wake-on-LAN app on PocketBase. It has no MCP
// upstream, so this is a first-party one built by kubitodev/mcp-fleet.
//
// Auth: PocketBase has NO static API token — you auth-with-password to get a
// short-lived JWT. So the server logs in with UPSNAP_MCP_USER / UPSNAP_MCP_PASSWORD
// (auto-detecting the collection like UpSnap's own login: _superusers then users,
// or pinned via UPSNAP_MCP_AUTH_COLLECTION) and re-auths transparently on a 401.
// The endpoint itself is guarded by a separate bearer (UPSNAP_MCP_AUTH_TOKEN),
// since power tools can turn machines off.
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

const version = "0.1.3"

func main() {
	base := strings.TrimRight(mustEnv("UPSNAP_URL"), "/")
	c := &Client{
		base: base,
		http: &http.Client{Timeout: 20 * time.Second},
		user: mustEnv("UPSNAP_MCP_USER"),
		pass: mustEnv("UPSNAP_MCP_PASSWORD"),
	}
	// Which PocketBase collection to auth against. If unset, mirror UpSnap's own
	// login (login/+page.svelte): try _superusers, then users — so the MCP works
	// with either kind of account without the operator needing to know which.
	if col := os.Getenv("UPSNAP_MCP_AUTH_COLLECTION"); col != "" {
		c.authColls = []string{col}
	} else {
		c.authColls = []string{"_superusers", "users"}
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
	base string
	http *http.Client
	user string
	pass string

	mu        sync.Mutex
	token     string
	authColls []string // candidate collections; the one that works is pinned first
}

// authenticate logs in and caches the JWT, trying each candidate collection in
// order (mirrors UpSnap's login: _superusers then users). The winning collection
// is moved to the front so later re-auths hit it first.
func (c *Client) authenticate(ctx context.Context) error {
	c.mu.Lock()
	cols := append([]string(nil), c.authColls...)
	c.mu.Unlock()

	var errs []string
	for _, col := range cols {
		token, err := c.login(ctx, col)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", col, err))
			continue
		}
		c.mu.Lock()
		c.token = token
		c.authColls = moveFront(c.authColls, col)
		c.mu.Unlock()
		return nil
	}
	return fmt.Errorf("upsnap auth failed for [%s]: %s", strings.Join(cols, ", "), strings.Join(errs, "; "))
}

// login does one auth-with-password against a single collection.
func (c *Client) login(ctx context.Context, col string) (string, error) {
	payload, _ := json.Marshal(map[string]string{"identity": c.user, "password": c.pass})
	url := c.base + "/api/collections/" + col + "/auth-with-password"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	if out.Token == "" {
		return "", fmt.Errorf("empty token")
	}
	return out.Token, nil
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
//
// ?async=true: UpSnap otherwise BLOCKS the response until the action settles —
// wake pings until the device boots (20s+), shutdown runs a remote command and
// waits. async dispatches it in a goroutine and returns 200 immediately, which
// is the right fit here (WoL is fire-and-forget; a long block also stalls the
// caller's approval/callback flow). The device's status updates in UpSnap after.
func (c *Client) power(ctx context.Context, action, id string) error {
	body, code, err := c.request(ctx, http.MethodGet, "/api/upsnap/"+action+"/"+id+"?async=true")
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

	registerSwitchBoot(s, c)
}

// registerSwitchBoot adds upsnap_switch_boot: reboot a machine into an alternate
// boot target. Generic — the "boot switch" is whatever the given HELPER device's
// shutdown command does (e.g. an SSH bootloader one-shot + reboot). The helper
// shares the real machine's MAC/IP, so its status reflects the machine: online →
// switch now; offline → wake it (boots the default OS first) and switch once it's
// back up, completed in the background so the call returns immediately.
func registerSwitchBoot(s *server.MCPServer, c *Client) {
	s.AddTool(mcp.NewTool("upsnap_switch_boot",
		mcp.WithDescription("Reboot a machine into an alternate boot target (e.g. the other OS of a dual-boot). Pass the UpSnap HELPER device whose shutdown command performs the boot switch. If the machine is online it switches immediately; if offline it wakes it (which boots the default OS first) and switches automatically once it is back online."),
		mcp.WithString("device", mcp.Required(), mcp.Description("The helper device (id or name) whose shutdown command performs the boot switch")),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
	), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ident, err := r.RequireString("device")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		dev, err := c.resolve(ctx, "devices", ident)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		id, name := str(dev["id"]), str(dev["name"])

		if strings.EqualFold(str(dev["status"]), "online") {
			if err := c.power(ctx, "shutdown", id); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("%q is online — ran its boot-switch command; the machine will reboot into the alternate OS", name)), nil
		}

		// Offline: wake the machine, then finish the switch once it's back up.
		if err := c.power(ctx, "wake", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		go c.switchWhenOnline(id, name)
		return mcp.NewToolResultText(fmt.Sprintf("%q was off — woke it (it boots the default OS first); it will switch to the alternate OS automatically once it's online (~1-2 min)", name)), nil
	})
}

// switchWhenOnline polls until the device reports online, waits a moment for SSH
// to come up, then triggers its boot-switch (shutdown) command. Detached from the
// request context so it survives the tool returning; best-effort (logs outcome).
// If the MCP pod restarts mid-wait, the pending switch is dropped — just retry.
func (c *Client) switchWhenOnline(id, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("switch_boot: %q did not come online within timeout; aborting", name)
			return
		case <-ticker.C:
			dev, err := c.getDevice(ctx, id)
			if err != nil {
				continue
			}
			if strings.EqualFold(str(dev["status"]), "online") {
				time.Sleep(8 * time.Second) // ping is up; give SSH a moment
				if err := c.power(ctx, "shutdown", id); err != nil {
					log.Printf("switch_boot: %q online but boot-switch failed: %v", name, err)
					return
				}
				log.Printf("switch_boot: %q online — boot-switch triggered", name)
				return
			}
		}
	}
}

// getDevice fetches a single device record by id.
func (c *Client) getDevice(ctx context.Context, id string) (map[string]any, error) {
	body, code, err := c.request(ctx, http.MethodGet, "/api/collections/devices/records/"+id)
	if err != nil {
		return nil, err
	}
	if code/100 != 2 {
		return nil, fmt.Errorf("get device failed (%d): %s", code, strings.TrimSpace(string(body)))
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return m, nil
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

// moveFront returns s with v placed first (used to pin the winning auth collection).
func moveFront(s []string, v string) []string {
	out := make([]string, 0, len(s))
	out = append(out, v)
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
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
