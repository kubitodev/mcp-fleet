package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	jf "github.com/jaredtrent/jellyfin-mcp/internal/jellyfin"
)

func RegisterMediaTools(server *mcp.Server, client jf.Client, enabled func(string, *mcp.ToolAnnotations) bool) {

	// --- jellyfin_tv_shows ---
	if enabled("jellyfin_tv_shows", AnnotReadOnly) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_tv_shows",
			Title: "TV Shows",
			InputSchema: jf.WithEnums[jf.TVShowsInput](map[string][]any{
				"action": {"seasons", "episodes", "next_up"},
			}),
			Description: "Navigate TV show structure: list seasons of a series, list episodes in a season, or get the next unplayed episode. " +
				"Use action 'seasons' with a series_id to see all seasons and episode counts. " +
				"Use action 'episodes' with a series_id and season_number to list all episodes in that season. " +
				"Use action 'next_up' to get the next episode to watch (optionally scoped to a specific series with series_id). " +
				"Get series IDs from jellyfin_search with type 'Series'.",
			Annotations: AnnotReadOnly,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.TVShowsInput) (*mcp.CallToolResult, *jf.TVShowsOutput, error) {
			userID, err := client.GetUserID(ctx)
			if err != nil {
				return jf.ErrResult("Jellyfin error: %v", err), nil, nil
			}

			switch args.Action {
			case "seasons":
				if args.SeriesID == "" {
					return jf.ErrResult("series_id is required for 'seasons' action. Use jellyfin_search with type 'Series' to find the series ID."), nil, nil
				}
				params := url.Values{
					"ParentId":         {args.SeriesID},
					"IncludeItemTypes": {"Season"},
					"Fields":           {"ChildCount"},
				}
				var result map[string]any
				endpoint := fmt.Sprintf("/Users/%s/Items", jf.SanitizeID(userID))
				if err := client.Get(ctx, endpoint, params, &result); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				rawItems := jf.ToSlice(result["Items"])
				items := make([]map[string]any, 0, len(rawItems))
				seasons := make([]jf.SeasonInfo, 0, len(rawItems))
				for _, raw := range rawItems {
					m := jf.ToMap(raw)
					items = append(items, map[string]any{
						"id":            jf.GetString(m, "Id"),
						"name":          jf.GetString(m, "Name"),
						"season_number": jf.GetInt(m, "IndexNumber"),
						"episode_count": jf.GetInt(m, "ChildCount"),
					})
					seasons = append(seasons, jf.SeasonInfo{
						ID:           jf.GetString(m, "Id"),
						Name:         jf.GetString(m, "Name"),
						SeasonNumber: jf.GetInt(m, "IndexNumber"),
						EpisodeCount: jf.GetInt(m, "ChildCount"),
					})
				}
				return jf.TextResult(fmt.Sprintf("Found %d seasons:\n\n%s", len(items), jf.FormatJSON(items))), &jf.TVShowsOutput{Seasons: seasons}, nil

			case "episodes":
				if args.SeriesID == "" {
					return jf.ErrResult("series_id is required for 'episodes' action."), nil, nil
				}
				if args.SeasonNumber == nil {
					return jf.ErrResult("season_number is required for 'episodes' action (e.g. 1 for Season 1)."), nil, nil
				}

				jf.ReportProgress(ctx, req, 0, 2, "Finding season...")

				// Find season by number
				params := url.Values{
					"ParentId":         {args.SeriesID},
					"IncludeItemTypes": {"Season"},
				}
				var seasonsResult map[string]any
				endpoint := fmt.Sprintf("/Users/%s/Items", jf.SanitizeID(userID))
				if err := client.Get(ctx, endpoint, params, &seasonsResult); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				var seasonID string
				for _, s := range jf.ToSlice(seasonsResult["Items"]) {
					m := jf.ToMap(s)
					if jf.GetInt(m, "IndexNumber") == *args.SeasonNumber {
						seasonID = jf.GetString(m, "Id")
						break
					}
				}
				if seasonID == "" {
					return jf.ErrResult("Season %d not found for this series.", *args.SeasonNumber), nil, nil
				}

				jf.ReportProgress(ctx, req, 1, 2, "Fetching episodes...")

				// Get episodes
				params = url.Values{
					"ParentId":         {seasonID},
					"IncludeItemTypes": {"Episode"},
					"Fields":           {"Overview,CommunityRating,RunTimeTicks"},
				}
				var result map[string]any
				endpoint = fmt.Sprintf("/Users/%s/Items", jf.SanitizeID(userID))
				if err := client.Get(ctx, endpoint, params, &result); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				rawItems := jf.ToSlice(result["Items"])
				items := make([]map[string]any, 0, len(rawItems))
				episodes := make([]jf.EpisodeInfo, 0, len(rawItems))
				for _, raw := range rawItems {
					m := jf.ToMap(raw)
					ep := map[string]any{
						"id":             jf.GetString(m, "Id"),
						"episode_number": jf.GetInt(m, "IndexNumber"),
						"name":           jf.GetString(m, "Name"),
					}
					epInfo := jf.EpisodeInfo{
						ID:            jf.GetString(m, "Id"),
						Name:          jf.GetString(m, "Name"),
						SeasonNumber:  *args.SeasonNumber,
						EpisodeNumber: jf.GetInt(m, "IndexNumber"),
					}
					if ov := jf.GetString(m, "Overview"); ov != "" {
						ep["overview"] = jf.Truncate(ov, jf.SummaryMaxLen)
						epInfo.Overview = jf.Truncate(ov, jf.SummaryMaxLen)
					}
					if rt := jf.GetInt64(m, "RunTimeTicks"); rt > 0 {
						ep["runtime_minutes"] = rt / jf.TicksPerMinute
						epInfo.RuntimeMinutes = rt / jf.TicksPerMinute
					}
					if rating := jf.GetFloat(m, "CommunityRating"); rating > 0 {
						ep["community_rating"] = rating
						epInfo.CommunityRating = rating
					}
					items = append(items, ep)
					episodes = append(episodes, epInfo)
				}
				return jf.TextResult(fmt.Sprintf("Season %d — %d episodes:\n\n%s", *args.SeasonNumber, len(items), jf.FormatJSON(items))), &jf.TVShowsOutput{Episodes: episodes}, nil

			case "next_up":
				maxItems := jf.ClampInt(args.Limit, 100, jf.MaxLimitCap)
				params := url.Values{
					"UserId": {userID},
					"Fields": {"Overview,ProductionYear"},
				}
				if args.SeriesID != "" {
					params.Set("SeriesId", args.SeriesID)
				}
				rawItems, _, err := jf.FetchAllPages(ctx, client, "/Shows/NextUp", params, maxItems)
				if err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := jf.MapExtract(rawItems, jf.ExtractMediaItem)
				return jf.TextResult(fmt.Sprintf("Next Up (%d episodes):\n\n%s", len(items), jf.FormatJSON(items))), &jf.TVShowsOutput{NextUp: jf.ToMediaItems(items)}, nil

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: seasons, episodes, next_up", args.Action), nil, nil
			}
		})
	}

	// --- jellyfin_music ---
	if enabled("jellyfin_music", AnnotReadOnly) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_music",
			Title: "Music",
			InputSchema: jf.WithEnums[jf.MusicInput](map[string][]any{
				"action": {"artists", "album_artists", "genres", "instant_mix"},
			}),
			Description: "Browse music content: list artists, album artists, music genres, or generate an instant mix playlist from a seed item. " +
				"Use 'artists' or 'album_artists' to browse the music library with optional name filtering. " +
				"Use 'genres' to list available music genres. " +
				"Use 'instant_mix' with an item_id (album, artist, song, or playlist) to generate an auto-playlist of similar tracks. " +
				"The query parameter filters results by name for artists and genres.",
			Annotations: AnnotReadOnly,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.MusicInput) (*mcp.CallToolResult, any, error) {
			limit := jf.ClampInt(args.Limit, 25, jf.MaxLimitCap)
			userID, err := client.GetUserID(ctx)
			if err != nil {
				return jf.ErrResult("Jellyfin error: %v", err), nil, nil
			}

			switch args.Action {
			case "artists", "album_artists":
				maxItems := jf.ClampInt(args.Limit, 200, jf.MaxLimitCap)
				endpoint := "/Artists"
				if args.Action == "album_artists" {
					endpoint = "/Artists/AlbumArtists"
				}
				params := url.Values{
					"UserId": {userID},
					"Fields": {"Overview,Genres"},
				}
				if args.Query != "" {
					params.Set("SearchTerm", args.Query)
				}
				rawItems, total, err := jf.FetchAllPages(ctx, client, endpoint, params, maxItems)
				if err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := jf.MapExtract(rawItems, jf.ExtractMediaItem)
				msg := fmt.Sprintf("Found %d artists", len(items))
				if total > len(items) {
					msg += fmt.Sprintf(" (of %d total)", total)
				}
				return jf.TextResult(fmt.Sprintf("%s:\n\n%s", msg, jf.FormatJSON(items))), nil, nil

			case "genres":
				maxItems := jf.ClampInt(args.Limit, 200, jf.MaxLimitCap)
				params := url.Values{
					"UserId": {userID},
				}
				if args.Query != "" {
					params.Set("SearchTerm", args.Query)
				}
				rawItems, _, err := jf.FetchAllPages(ctx, client, "/MusicGenres", params, maxItems)
				if err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := jf.MapExtract(rawItems, jf.ExtractMediaItem)
				return jf.TextResult(fmt.Sprintf("Music genres (%d):\n\n%s", len(items), jf.FormatJSON(items))), nil, nil

			case "instant_mix":
				if args.ItemID == "" {
					return jf.ErrResult("item_id is required for instant_mix. Provide an album, artist, song, or playlist ID."), nil, nil
				}
				params := url.Values{
					"Limit":  {fmt.Sprintf("%d", limit)},
					"UserId": {userID},
				}
				endpoint := fmt.Sprintf("/Items/%s/InstantMix", jf.SanitizeID(args.ItemID))
				var result map[string]any
				if err := client.Get(ctx, endpoint, params, &result); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := jf.ExtractItemList(result)
				return jf.TextResult(fmt.Sprintf("Instant mix (%d tracks):\n\n%s", len(items), jf.FormatJSON(items))), nil, nil

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: artists, album_artists, genres, instant_mix", args.Action), nil, nil
			}
		})
	}

	// --- jellyfin_people ---
	if enabled("jellyfin_people", AnnotReadOnly) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_people",
			Title: "People & Studios",
			InputSchema: jf.WithEnums[jf.PeopleInput](map[string][]any{
				"action": {"persons", "studios"},
			}),
			Description: "Browse persons (actors, directors, writers) and studios in the Jellyfin library. " +
				"Use action 'persons' to list or search for people who appear in your media. " +
				"Use action 'studios' to list production studios. " +
				"The query parameter filters results by name. Person and studio names can be used with jellyfin_browse to find their media.",
			Annotations: AnnotReadOnly,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.PeopleInput) (*mcp.CallToolResult, any, error) {
			userID, err := client.GetUserID(ctx)
			if err != nil {
				return jf.ErrResult("Jellyfin error: %v", err), nil, nil
			}

			switch args.Action {
			case "persons":
				maxItems := jf.ClampInt(args.Limit, 200, jf.MaxLimitCap)
				params := url.Values{
					"UserId": {userID},
				}
				if args.Query != "" {
					params.Set("SearchTerm", args.Query)
				}
				rawItems, total, err := jf.FetchAllPages(ctx, client, "/Persons", params, maxItems)
				if err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := jf.MapExtract(rawItems, jf.ExtractMediaItem)
				msg := fmt.Sprintf("Found %d persons", len(items))
				if total > len(items) {
					msg += fmt.Sprintf(" (of %d total)", total)
				}
				return jf.TextResult(fmt.Sprintf("%s:\n\n%s", msg, jf.FormatJSON(items))), nil, nil

			case "studios":
				maxItems := jf.ClampInt(args.Limit, 200, jf.MaxLimitCap)
				params := url.Values{
					"UserId": {userID},
				}
				if args.Query != "" {
					params.Set("SearchTerm", args.Query)
				}
				rawItems, total, err := jf.FetchAllPages(ctx, client, "/Studios", params, maxItems)
				if err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := jf.MapExtract(rawItems, jf.ExtractMediaItem)
				msg := fmt.Sprintf("Found %d studios", len(items))
				if total > len(items) {
					msg += fmt.Sprintf(" (of %d total)", total)
				}
				return jf.TextResult(fmt.Sprintf("%s:\n\n%s", msg, jf.FormatJSON(items))), nil, nil

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: persons, studios", args.Action), nil, nil
			}
		})
	}
}
