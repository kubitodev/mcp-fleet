package tools_test

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestSystemInfo_Ping(t *testing.T) {
	mc := &mockClient{
		getFunc: func(_ context.Context, endpoint string, _ url.Values, dest any) error {
			if endpoint == "/System/Ping" {
				// Ping just needs to not error
				return nil
			}
			return nil
		},
		getRawFunc: func(_ context.Context, endpoint string, _ url.Values) (string, error) {
			if endpoint == "/System/Ping" {
				return "Jellyfin Server", nil
			}
			return "", nil
		},
	}

	result := callTool(t, mc, "", "jellyfin_system_info", map[string]any{
		"action": "ping",
	})

	text := resultText(t, result)
	if !strings.Contains(text, "responsive") {
		t.Errorf("expected positive ping result, got: %s", text)
	}
}

func TestSystemInfo_Info(t *testing.T) {
	mc := &mockClient{
		getFunc: func(_ context.Context, endpoint string, _ url.Values, dest any) error {
			if endpoint == "/System/Info" {
				return jsonInto(map[string]any{
					"ServerName":      "My Jellyfin",
					"Version":         "10.9.0",
					"OperatingSystem": "Linux",
					"Id":              "abc123",
				}, dest)
			}
			return nil
		},
	}

	result := callTool(t, mc, "", "jellyfin_system_info", map[string]any{
		"action": "info",
	})

	text := resultText(t, result)
	if !strings.Contains(text, "My Jellyfin") {
		t.Errorf("expected server name in result, got: %s", text)
	}
	if !strings.Contains(text, "10.9.0") {
		t.Errorf("expected version in result, got: %s", text)
	}
}

func TestSystemInfo_Whoami(t *testing.T) {
	mc := &mockClient{
		getFunc: func(_ context.Context, endpoint string, _ url.Values, dest any) error {
			if strings.Contains(endpoint, "/Users/") {
				return jsonInto(map[string]any{
					"Name": "admin",
					"Id":   "test-user-id",
					"Policy": map[string]any{
						"IsAdministrator": true,
					},
				}, dest)
			}
			return nil
		},
	}

	result := callTool(t, mc, "", "jellyfin_system_info", map[string]any{
		"action": "whoami",
	})

	text := resultText(t, result)
	if !strings.Contains(text, "admin") {
		t.Errorf("expected username in result, got: %s", text)
	}
}
