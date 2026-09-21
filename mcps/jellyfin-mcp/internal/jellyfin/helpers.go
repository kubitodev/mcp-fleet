package jellyfin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TextResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

func ErrResult(format string, args ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, args...)}},
		IsError: true,
	}
}

// ErrResultWithHint returns an error result with an actionable recovery hint appended.
func ErrResultWithHint(hint string, format string, args ...any) *mcp.CallToolResult {
	msg := fmt.Sprintf(format, args...)
	if hint != "" {
		msg += "\n\nHint: " + hint
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}
}

// ReportProgress sends a progress notification if the request includes a progress token.
func ReportProgress(ctx context.Context, req *mcp.CallToolRequest, progress, total float64, message string) {
	token := req.Params.GetProgressToken()
	if token != nil {
		if err := req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
			ProgressToken: token,
			Progress:      progress,
			Total:         total,
			Message:       message,
		}); err != nil {
			log.Printf("progress notification failed: %v", err)
		}
	}
}

func SanitizeID(id string) string {
	return url.PathEscape(id)
}

func FormatJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("[json error: %v]", err)
	}
	return string(b)
}

func GetString(m map[string]any, key string) string {
	if v, ok := m[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func GetInt(m map[string]any, key string) int {
	if v, ok := m[key]; ok && v != nil {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return 0
}

func GetIntPtr(m map[string]any, key string) *int {
	if v, ok := m[key]; ok && v != nil {
		if f, ok := v.(float64); ok {
			i := int(f)
			return &i
		}
	}
	return nil
}

func GetInt64(m map[string]any, key string) int64 {
	if v, ok := m[key]; ok && v != nil {
		if f, ok := v.(float64); ok {
			return int64(f)
		}
	}
	return 0
}

func GetFloat(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok && v != nil {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}

func GetBool(m map[string]any, key string) bool {
	if v, ok := m[key]; ok && v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// GetNum extracts a numeric value from a map, handling the int/int64/float64
// type variations that arise from JSON unmarshaling and internal map building.
func GetNum[T int | int64 | float64](m map[string]any, key string) T {
	if v, ok := m[key]; ok && v != nil {
		switch n := v.(type) {
		case int:
			return T(n)
		case int64:
			return T(n)
		case float64:
			return T(n)
		case T:
			return n
		}
	}
	var zero T
	return zero
}

func Truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

func ToSlice(v any) []any {
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func ToMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func MapExtract(items []any, fn func(map[string]any) map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		if m := ToMap(raw); m != nil {
			out = append(out, fn(m))
		}
	}
	return out
}

func JoinIDs(ids []string) string {
	return strings.Join(ids, ",")
}

func ToStringSlice(v any) []string {
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func DefaultInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// ClampInt returns v if positive, otherwise def, and caps the result at max.
func ClampInt(v, def, max int) int {
	n := DefaultInt(v, def)
	if n > max {
		return max
	}
	return n
}

func BoolPtr(b bool) *bool {
	return &b
}

func FormatGB(bytes int64) string {
	gb := float64(bytes) / float64(BytesPerGB)
	if gb >= 100 {
		return fmt.Sprintf("%.1f GB", gb)
	}
	return fmt.Sprintf("%.2f GB", gb)
}

// MaskToken reveals only the first/last 4 characters; returns the full string
// if it's too short for masking to be meaningful.
func MaskToken(token string) string {
	if len(token) <= TokenMaskMinLen {
		return token
	}
	return token[:TokenRevealChars] + "..." + token[len(token)-TokenRevealChars:]
}

// ConfirmationGate returns a warning result if confirm is not true.
// Returns nil if confirmed, allowing the caller to proceed.
// When the client supports elicitation, it prompts the user directly
// before falling back to the legacy confirm=true pattern.
func ConfirmationGate(ctx context.Context, req *mcp.CallToolRequest, confirm *bool, warning string) *mcp.CallToolResult {
	if confirm != nil && *confirm {
		return nil
	}
	// Try elicitation if the client supports it
	if sess := req.Session; sess != nil {
		if caps := sess.InitializeParams(); caps != nil && caps.Capabilities.Elicitation != nil {
			result, err := sess.Elicit(ctx, &mcp.ElicitParams{
				Message: warning,
				RequestedSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"confirm": map[string]any{
							"type":        "boolean",
							"description": "Set to true to confirm this destructive operation",
							"default":     false,
						},
					},
					"required": []string{"confirm"},
				},
			})
			if err == nil && result != nil {
				switch result.Action {
				case "accept":
					if v, ok := result.Content["confirm"].(bool); ok && v {
						return nil
					}
					return TextResult("Operation cancelled by user.")
				case "decline", "cancel":
					return TextResult("Operation cancelled by user.")
				}
			}
			// Elicitation failed — fall through to legacy prompt
		}
	}
	return TextResult("⚠️ CONFIRMATION REQUIRED\n\n" + warning +
		"\n\nTo proceed, call this tool again with confirm=true.")
}

// FilterPrefix returns items whose lowercase form starts with the given prefix.
func FilterPrefix(items []string, prefix string) []string {
	if prefix == "" {
		return items
	}
	lower := strings.ToLower(prefix)
	var out []string
	for _, item := range items {
		if strings.HasPrefix(strings.ToLower(item), lower) {
			out = append(out, item)
		}
	}
	return out
}

// ApplyMetadataFields applies user-provided metadata fields onto an existing
// Jellyfin item map (fetched via GET). Only non-zero fields are applied.
func ApplyMetadataFields(current map[string]any, args MetadataInput) {
	if args.Name != "" {
		current["Name"] = args.Name
	}
	if args.Overview != "" {
		current["Overview"] = args.Overview
	}
	if len(args.Genres) > 0 {
		genreObjs := make([]map[string]any, len(args.Genres))
		for i, g := range args.Genres {
			genreObjs[i] = map[string]any{"Name": g}
		}
		current["GenreItems"] = genreObjs
		current["Genres"] = args.Genres
	}
	if len(args.Tags) > 0 {
		tagObjs := make([]map[string]any, len(args.Tags))
		for i, t := range args.Tags {
			tagObjs[i] = map[string]any{"Name": t}
		}
		current["TagItems"] = tagObjs
		current["Tags"] = args.Tags
	}
	if len(args.Studios) > 0 {
		studioObjs := make([]map[string]any, len(args.Studios))
		for i, s := range args.Studios {
			studioObjs[i] = map[string]any{"Name": s}
		}
		current["Studios"] = studioObjs
	}
	if args.Year != nil {
		current["ProductionYear"] = *args.Year
	}
	if args.CommunityRating != nil {
		current["CommunityRating"] = *args.CommunityRating
	}
	if args.OfficialRating != "" {
		current["OfficialRating"] = args.OfficialRating
	}
	if args.SortName != "" {
		current["ForcedSortName"] = args.SortName
	}
	if len(args.LockedFields) > 0 {
		current["LockedFields"] = args.LockedFields
	}
}

// BuildProviderLinks converts provider IDs into clickable URLs.
// itemType is the Jellyfin item type (Movie, Series, etc.) used to pick the
// correct TMDb path segment.
func BuildProviderLinks(providerIDs map[string]string, itemType string) map[string]string {
	links := make(map[string]string)
	if id, ok := providerIDs["Imdb"]; ok && id != "" {
		links["IMDb"] = "https://www.imdb.com/title/" + id
	}
	if id, ok := providerIDs["Tmdb"]; ok && id != "" {
		segment := "movie"
		switch itemType {
		case "Series", "Season", "Episode":
			segment = "tv"
		case "Person":
			segment = "person"
		}
		links["TMDb"] = "https://www.themoviedb.org/" + segment + "/" + id
	}
	if id, ok := providerIDs["Tvdb"]; ok && id != "" {
		links["TVDB"] = "https://thetvdb.com/?id=" + id + "&tab=series"
	}
	return links
}
