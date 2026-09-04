package tools_test

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestUserData_Favorite(t *testing.T) {
	var calledEndpoint string
	mc := &mockClient{
		postNoContentFunc: func(_ context.Context, endpoint string, _ url.Values, _ any) error {
			calledEndpoint = endpoint
			return nil
		},
	}

	result := callTool(t, mc, "", "jellyfin_user_data", map[string]any{
		"action":  "favorite",
		"item_id": "item-123",
	})

	text := resultText(t, result)
	if !strings.Contains(text, "favorite") {
		t.Errorf("expected favorite confirmation, got: %s", text)
	}
	if !strings.Contains(calledEndpoint, "FavoriteItems") {
		t.Errorf("expected FavoriteItems endpoint, got: %s", calledEndpoint)
	}
}

func TestUserData_GetUserData(t *testing.T) {
	mc := &mockClient{
		getFunc: func(_ context.Context, endpoint string, _ url.Values, dest any) error {
			if strings.Contains(endpoint, "/UserData") {
				return jsonInto(map[string]any{
					"PlayCount":             3,
					"IsFavorite":            true,
					"Played":                true,
					"Rating":                8.5,
					"PlaybackPositionTicks": 0,
				}, dest)
			}
			return nil
		},
	}

	result := callTool(t, mc, "", "jellyfin_user_data", map[string]any{
		"action":  "get_user_data",
		"item_id": "item-123",
	})

	text := resultText(t, result)
	if result.IsError {
		t.Errorf("unexpected error: %s", text)
	}
}

func TestUserData_Rate(t *testing.T) {
	var capturedBody any
	mc := &mockClient{
		postNoContentFunc: func(_ context.Context, endpoint string, _ url.Values, body any) error {
			if strings.Contains(endpoint, "/UserData") {
				capturedBody = body
			}
			return nil
		},
	}

	result := callTool(t, mc, "", "jellyfin_user_data", map[string]any{
		"action":  "rate",
		"item_id": "item-123",
		"rating":  8.5,
	})

	text := resultText(t, result)
	if !strings.Contains(text, "8.5") {
		t.Errorf("expected rating value in result, got: %s", text)
	}
	if capturedBody == nil {
		t.Error("expected body to be sent")
	}
}
