package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	jf "github.com/jaredtrent/jellyfin-mcp/internal/jellyfin"
)

func RegisterLiveTVTools(server *mcp.Server, client jf.Client, enabled func(string, *mcp.ToolAnnotations) bool) {

	// --- jellyfin_live_tv ---
	if enabled("jellyfin_live_tv", AnnotReadOnly) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_live_tv",
			Title: "Live TV",
			InputSchema: jf.WithEnums[jf.LiveTVInput](map[string][]any{
				"action": {"channels", "channel", "programs", "recommended", "guide_info", "tuners"},
			}),
			Description: "Browse live TV channels and program guide. Use 'channels' to list available channels, 'channel' for details on a specific channel. " +
				"Use 'programs' to see current and upcoming programs (optionally filter by channel_id). " +
				"Use 'recommended' for recommended programs. Use 'guide_info' for TV guide metadata and date ranges. " +
				"Use 'tuners' to list configured tuner devices. Requires Live TV to be set up with a tuner and guide data source.",
			Annotations: AnnotReadOnly,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.LiveTVInput) (*mcp.CallToolResult, any, error) {
			userID, err := client.GetUserID(ctx)
			if err != nil {
				return jf.ErrResult("Jellyfin error: %v", err), nil, nil
			}

			switch args.Action {
			case "channels":
				maxItems := jf.ClampInt(args.Limit, 500, jf.MaxLimitCap)
				params := url.Values{
					"UserId": {userID},
				}
				rawItems, _, err := jf.FetchAllPages(ctx, client, "/LiveTv/Channels", params, maxItems)
				if err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := jf.MapExtract(rawItems, jf.ExtractMediaItem)
				return jf.TextResult(fmt.Sprintf("Live TV channels (%d):\n\n%s", len(items), jf.FormatJSON(items))), nil, nil

			case "channel":
				if args.ChannelID == "" {
					return jf.ErrResult("channel_id is required. Use 'channels' to find IDs."), nil, nil
				}
				var channel map[string]any
				endpoint := fmt.Sprintf("/LiveTv/Channels/%s", jf.SanitizeID(args.ChannelID))
				params := url.Values{"UserId": {userID}}
				if err := client.Get(ctx, endpoint, params, &channel); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				return jf.TextResult(jf.FormatJSON(channel)), nil, nil

			case "programs":
				maxItems := jf.ClampInt(args.Limit, 200, jf.MaxLimitCap)
				params := url.Values{
					"UserId": {userID},
				}
				if args.ChannelID != "" {
					params.Set("ChannelIds", args.ChannelID)
				}
				rawItems, _, err := jf.FetchAllPages(ctx, client, "/LiveTv/Programs", params, maxItems)
				if err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := jf.MapExtract(rawItems, jf.ExtractMediaItem)
				return jf.TextResult(fmt.Sprintf("Programs (%d):\n\n%s", len(items), jf.FormatJSON(items))), nil, nil

			case "recommended":
				maxItems := jf.ClampInt(args.Limit, 100, jf.MaxLimitCap)
				params := url.Values{
					"UserId": {userID},
				}
				rawItems, _, err := jf.FetchAllPages(ctx, client, "/LiveTv/Programs/Recommended", params, maxItems)
				if err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := jf.MapExtract(rawItems, jf.ExtractMediaItem)
				return jf.TextResult(fmt.Sprintf("Recommended programs (%d):\n\n%s", len(items), jf.FormatJSON(items))), nil, nil

			case "guide_info":
				var info map[string]any
				if err := client.Get(ctx, "/LiveTv/GuideInfo", nil, &info); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				return jf.TextResult(jf.FormatJSON(info)), nil, nil

			case "tuners":
				var tuners []map[string]any
				if err := client.Get(ctx, "/LiveTv/TunerHosts/Types", nil, &tuners); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Tuner types:\n\n%s", jf.FormatJSON(tuners))), nil, nil

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: channels, channel, programs, recommended, guide_info, tuners", args.Action), nil, nil
			}
		})
	}

	// --- jellyfin_recordings ---
	if enabled("jellyfin_recordings", AnnotDestructive) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_recordings",
			Title: "DVR Recordings",
			InputSchema: jf.WithEnums[jf.RecordingsInput](map[string][]any{
				"action": {"list", "delete", "timers", "create_timer", "cancel_timer", "series_timers", "create_series_timer", "cancel_series_timer"},
			}),
			Description: "Manage DVR recordings and timers. Use 'list' to see existing recordings, 'delete' to remove one (destructive). " +
				"Use 'timers' to see scheduled one-time recordings, 'create_timer' to schedule a recording from a program_id, 'cancel_timer' to cancel one. " +
				"Use 'series_timers' to see series recording rules, 'create_series_timer' to record all episodes of a series, 'cancel_series_timer' to cancel. " +
				"Requires Live TV with DVR capability configured.",
			Annotations: AnnotDestructive,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.RecordingsInput) (*mcp.CallToolResult, any, error) {
			switch args.Action {
			case "list":
				var result map[string]any
				if err := client.Get(ctx, "/LiveTv/Recordings", nil, &result); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := jf.ExtractItemList(result)
				return jf.TextResult(fmt.Sprintf("Recordings (%d):\n\n%s", len(items), jf.FormatJSON(items))), nil, nil

			case "delete":
				if args.RecordingID == "" {
					return jf.ErrResult("recording_id is required."), nil, nil
				}
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, fmt.Sprintf("This will PERMANENTLY DELETE recording '%s'.", args.RecordingID)); result != nil {
					return result, nil, nil
				}
				endpoint := fmt.Sprintf("/LiveTv/Recordings/%s", jf.SanitizeID(args.RecordingID))
				if err := client.Del(ctx, endpoint, nil); err != nil {
					return jf.ErrResult("Failed to delete recording: %v", err), nil, nil
				}
				return jf.TextResult("Recording deleted."), nil, nil

			case "timers":
				var result map[string]any
				if err := client.Get(ctx, "/LiveTv/Timers", nil, &result); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				return jf.TextResult(jf.FormatJSON(result)), nil, nil

			case "create_timer":
				if args.ProgramID == "" {
					return jf.ErrResult("program_id is required. Use jellyfin_live_tv 'programs' to find program IDs."), nil, nil
				}
				params := url.Values{"programId": {args.ProgramID}}
				var defaults map[string]any
				if err := client.Get(ctx, "/LiveTv/Timers/Defaults", params, &defaults); err != nil {
					return jf.ErrResult("Failed to get timer defaults: %v", err), nil, nil
				}
				if err := client.PostNoContent(ctx, "/LiveTv/Timers", nil, defaults); err != nil {
					return jf.ErrResult("Failed to create timer: %v", err), nil, nil
				}
				return jf.TextResult("Recording timer created."), nil, nil

			case "cancel_timer":
				if args.TimerID == "" {
					return jf.ErrResult("timer_id is required. Use 'timers' to find timer IDs."), nil, nil
				}
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, fmt.Sprintf("This will CANCEL recording timer '%s'.", args.TimerID)); result != nil {
					return result, nil, nil
				}
				endpoint := fmt.Sprintf("/LiveTv/Timers/%s", jf.SanitizeID(args.TimerID))
				if err := client.Del(ctx, endpoint, nil); err != nil {
					return jf.ErrResult("Failed to cancel timer: %v", err), nil, nil
				}
				return jf.TextResult("Timer cancelled."), nil, nil

			case "series_timers":
				var result map[string]any
				if err := client.Get(ctx, "/LiveTv/SeriesTimers", nil, &result); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				return jf.TextResult(jf.FormatJSON(result)), nil, nil

			case "create_series_timer":
				if args.ProgramID == "" {
					return jf.ErrResult("program_id is required."), nil, nil
				}
				params := url.Values{"programId": {args.ProgramID}}
				var defaults map[string]any
				if err := client.Get(ctx, "/LiveTv/SeriesTimers/Defaults", params, &defaults); err != nil {
					defaults = map[string]any{"ProgramId": args.ProgramID}
				}
				if err := client.PostNoContent(ctx, "/LiveTv/SeriesTimers", nil, defaults); err != nil {
					return jf.ErrResult("Failed to create series timer: %v", err), nil, nil
				}
				return jf.TextResult("Series recording rule created."), nil, nil

			case "cancel_series_timer":
				if args.TimerID == "" {
					return jf.ErrResult("timer_id is required. Use 'series_timers' to find IDs."), nil, nil
				}
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, fmt.Sprintf("This will CANCEL series recording rule '%s'. Future episodes will not be recorded.", args.TimerID)); result != nil {
					return result, nil, nil
				}
				endpoint := fmt.Sprintf("/LiveTv/SeriesTimers/%s", jf.SanitizeID(args.TimerID))
				if err := client.Del(ctx, endpoint, nil); err != nil {
					return jf.ErrResult("Failed to cancel series timer: %v", err), nil, nil
				}
				return jf.TextResult("Series recording rule cancelled."), nil, nil

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: list, delete, timers, create_timer, cancel_timer, series_timers, create_series_timer, cancel_series_timer", args.Action), nil, nil
			}
		})
	}
}
