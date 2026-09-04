package tools_test

import (
	"context"
	"encoding/json"
	"net/url"

	jf "github.com/jaredtrent/jellyfin-mcp/internal/jellyfin"
)

// compile-time interface satisfaction check
var _ jf.Client = (*mockClient)(nil)

// mockClient implements jf.Client with configurable function fields.
type mockClient struct {
	getFunc           func(ctx context.Context, endpoint string, params url.Values, dest any) error
	getRawFunc        func(ctx context.Context, endpoint string, params url.Values) (string, error)
	postFunc          func(ctx context.Context, endpoint string, params url.Values, reqBody any, dest any) error
	postNoContentFunc func(ctx context.Context, endpoint string, params url.Values, reqBody any) error
	postRawFunc       func(ctx context.Context, endpoint string, params url.Values, body []byte, contentType string) error
	delFunc           func(ctx context.Context, endpoint string, params url.Values) error
	doRequestFunc     func(ctx context.Context, method, endpoint string, params url.Values, body any) ([]byte, error)
	getUserIDFunc     func(ctx context.Context) (string, error)
	baseURLVal        string
	apiKeyVal         string
}

func (m *mockClient) Get(ctx context.Context, endpoint string, params url.Values, dest any) error {
	if m.getFunc != nil {
		return m.getFunc(ctx, endpoint, params, dest)
	}
	return nil
}

func (m *mockClient) GetRaw(ctx context.Context, endpoint string, params url.Values) (string, error) {
	if m.getRawFunc != nil {
		return m.getRawFunc(ctx, endpoint, params)
	}
	return "", nil
}

func (m *mockClient) Post(ctx context.Context, endpoint string, params url.Values, reqBody any, dest any) error {
	if m.postFunc != nil {
		return m.postFunc(ctx, endpoint, params, reqBody, dest)
	}
	return nil
}

func (m *mockClient) PostNoContent(ctx context.Context, endpoint string, params url.Values, reqBody any) error {
	if m.postNoContentFunc != nil {
		return m.postNoContentFunc(ctx, endpoint, params, reqBody)
	}
	return nil
}

func (m *mockClient) PostRaw(ctx context.Context, endpoint string, params url.Values, body []byte, contentType string) error {
	if m.postRawFunc != nil {
		return m.postRawFunc(ctx, endpoint, params, body, contentType)
	}
	return nil
}

func (m *mockClient) Del(ctx context.Context, endpoint string, params url.Values) error {
	if m.delFunc != nil {
		return m.delFunc(ctx, endpoint, params)
	}
	return nil
}

func (m *mockClient) DoRequest(ctx context.Context, method, endpoint string, params url.Values, body any) ([]byte, error) {
	if m.doRequestFunc != nil {
		return m.doRequestFunc(ctx, method, endpoint, params, body)
	}
	return nil, nil
}

func (m *mockClient) GetUserID(ctx context.Context) (string, error) {
	if m.getUserIDFunc != nil {
		return m.getUserIDFunc(ctx)
	}
	return "test-user-id", nil
}

func (m *mockClient) BaseURL() string {
	if m.baseURLVal != "" {
		return m.baseURLVal
	}
	return "http://localhost:8096"
}

func (m *mockClient) APIKey() string {
	if m.apiKeyVal != "" {
		return m.apiKeyVal
	}
	return "test-api-key"
}

// jsonInto marshals data to JSON then unmarshals into dest.
// Used to simulate API responses in the mock client.
func jsonInto(data any, dest any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}
