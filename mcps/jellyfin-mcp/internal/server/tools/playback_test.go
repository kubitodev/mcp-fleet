package tools_test

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestSessions_List(t *testing.T) {
	mc := &mockClient{
		getFunc: func(_ context.Context, endpoint string, _ url.Values, dest any) error {
			if endpoint == "/Sessions" {
				return jsonInto([]map[string]any{
					{
						"Id":         "session-1",
						"DeviceName": "Living Room TV",
						"Client":     "Jellyfin Web",
						"UserName":   "testuser",
						"NowPlayingItem": map[string]any{
							"Name": "Neon Cascade",
							"Type": "Movie",
						},
					},
					{
						"Id":         "session-2",
						"DeviceName": "iPhone",
						"Client":     "Jellyfin Mobile",
						"UserName":   "testuser",
					},
				}, dest)
			}
			return nil
		},
	}

	result := callTool(t, mc, "", "jellyfin_sessions", map[string]any{
		"action": "list",
	})

	text := resultText(t, result)
	if !strings.Contains(text, "Playing") {
		t.Errorf("expected 'Playing' in result, got: %s", text)
	}
	if !strings.Contains(text, "Connected") || !strings.Contains(text, "idle") {
		t.Errorf("expected 'Connected/idle' in result, got: %s", text)
	}
}

func TestSessions_Resume(t *testing.T) {
	mc := &mockClient{
		getFunc: func(_ context.Context, endpoint string, _ url.Values, dest any) error {
			if strings.Contains(endpoint, "/Items") {
				return jsonInto(map[string]any{
					"Items": []map[string]any{
						{
							"Id":   "item-1",
							"Name": "Movie in Progress",
							"Type": "Movie",
							"UserData": map[string]any{
								"PlayedPercentage": 45.0,
							},
						},
					},
					"TotalRecordCount": 1,
				}, dest)
			}
			return nil
		},
	}

	result := callTool(t, mc, "", "jellyfin_sessions", map[string]any{
		"action": "resume",
	})

	text := resultText(t, result)
	if !strings.Contains(text, "Resume watching") {
		t.Errorf("expected 'Resume watching' in result, got: %s", text)
	}
}
