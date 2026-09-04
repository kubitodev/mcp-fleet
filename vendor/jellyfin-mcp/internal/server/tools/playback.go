package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	jf "github.com/jaredtrent/jellyfin-mcp/internal/jellyfin"
)

func RegisterPlaybackTools(server *mcp.Server, client jf.Client, enabled func(string, *mcp.ToolAnnotations) bool) {

	// --- jellyfin_sessions ---
	if enabled("jellyfin_sessions", AnnotReadOnly) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_sessions",
			Title: "Sessions",
			InputSchema: jf.WithEnums[jf.SessionsInput](map[string][]any{
				"action": {"list", "resume"},
			}),
			Description: "View active playback sessions and resumable items. Use action 'list' to see all connected clients, what they're playing, " +
				"and their session IDs (needed for jellyfin_playback_control and jellyfin_play). " +
				"Use action 'resume' to see items with in-progress playback that can be continued. " +
				"Session IDs change when clients reconnect, so always list sessions before sending playback commands.",
			Annotations: AnnotReadOnly,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.SessionsInput) (*mcp.CallToolResult, *jf.SessionsOutput, error) {
			switch args.Action {
			case "list":
				var sessions []map[string]any
				if err := client.Get(ctx, "/Sessions", nil, &sessions); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				playing := make([]map[string]any, 0)
				connected := make([]map[string]any, 0)
				for _, s := range sessions {
					info := jf.ExtractSessionInfo(s)
					if jf.ToMap(s["NowPlayingItem"]) != nil {
						info["status"] = "playing"
						playing = append(playing, info)
					} else {
						info["status"] = "connected"
						connected = append(connected, info)
					}
				}
				var sb strings.Builder
				fmt.Fprintf(&sb, "Playing (%d):\n\n%s", len(playing), jf.FormatJSON(playing))
				if len(connected) > 0 {
					fmt.Fprintf(&sb, "\n\nConnected/idle (%d):\n\n%s", len(connected), jf.FormatJSON(connected))
				}
				allSessions := make([]map[string]any, 0, len(playing)+len(connected))
				allSessions = append(allSessions, playing...)
				allSessions = append(allSessions, connected...)
				return jf.TextResult(sb.String()), &jf.SessionsOutput{Sessions: jf.ToSessionInfos(allSessions)}, nil

			case "resume":
				userID, err := client.GetUserID(ctx)
				if err != nil {
					return jf.ErrResult("Jellyfin error: %v", err), nil, nil
				}
				maxItems := jf.ClampInt(args.Limit, 100, jf.MaxLimitCap)
				params := url.Values{
					"Filters":   {"IsResumable"},
					"Recursive": {"true"},
					"SortBy":    {"DatePlayed"},
					"SortOrder": {"Descending"},
					"Fields":    {"Overview,RunTimeTicks"},
				}
				endpoint := fmt.Sprintf("/Users/%s/Items", jf.SanitizeID(userID))
				rawItems, _, err := jf.FetchAllPages(ctx, client, endpoint, params, maxItems)
				if err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := make([]map[string]any, 0, len(rawItems))
				for _, raw := range rawItems {
					m := jf.ToMap(raw)
					item := jf.ExtractMediaItem(m)
					if ud := jf.ToMap(m["UserData"]); ud != nil {
						if pct := jf.GetFloat(ud, "PlayedPercentage"); pct > 0 {
							item["progress"] = fmt.Sprintf("%.0f%%", pct)
						}
					}
					items = append(items, item)
				}
				return jf.TextResult(fmt.Sprintf("Resume watching (%d items):\n\n%s", len(items), jf.FormatJSON(items))), &jf.SessionsOutput{Resume: jf.ToMediaItems(items)}, nil

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: list, resume", args.Action), nil, nil
			}
		})
	}

	// --- jellyfin_playback_control ---
	if enabled("jellyfin_playback_control", AnnotWriteOp) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_playback_control",
			Title: "Playback Control",
			InputSchema: jf.WithEnums[jf.PlaybackControlInput](map[string][]any{
				"command": {"Pause", "Unpause", "Stop", "NextTrack", "PreviousTrack", "Seek", "Mute", "Unmute", "ToggleMute", "SetVolume", "SendMessage", "GoHome", "GoToSettings", "ChannelUp", "ChannelDown", "DisplayContent"},
			}),
			Description: "Send playback commands to an active client session. Supports Pause, Unpause, Stop, NextTrack, PreviousTrack, Seek, " +
				"Mute, Unmute, ToggleMute, SetVolume, and SendMessage. " +
				"The session_id must come from jellyfin_sessions (list action). For Seek, provide seek_position_ticks (10,000,000 ticks = 1 second). " +
				"To convert: multiply seconds by 10,000,000 to get ticks. The position_ticks field from jellyfin_sessions provides the raw value directly for seek operations. " +
				"For SetVolume, provide volume (0-100). For SendMessage, provide message text. " +
				"Also supports GoHome, GoToSettings, ChannelUp, ChannelDown (client navigation), and DisplayContent (navigate client to show an item — requires item_id). " +
				"The session must be actively playing for transport commands to work.",
			Annotations: AnnotWriteOp,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.PlaybackControlInput) (*mcp.CallToolResult, any, error) {
			sessionID := jf.SanitizeID(args.SessionID)

			switch args.Command {
			case "Pause", "Unpause", "Stop", "NextTrack", "PreviousTrack":
				endpoint := fmt.Sprintf("/Sessions/%s/Playing/%s", sessionID, args.Command)
				if err := client.PostNoContent(ctx, endpoint, nil, nil); err != nil {
					return jf.ErrResult("Failed to send command: %v. Verify the session_id is valid using jellyfin_sessions.", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Command '%s' sent to session.", args.Command)), nil, nil

			case "Seek":
				if args.SeekTicks == nil {
					return jf.ErrResult("seek_position_ticks is required for Seek command (10,000,000 ticks = 1 second)."), nil, nil
				}
				endpoint := fmt.Sprintf("/Sessions/%s/Playing/Seek", sessionID)
				params := url.Values{"seekPositionTicks": {fmt.Sprintf("%d", *args.SeekTicks)}}
				if err := client.PostNoContent(ctx, endpoint, params, nil); err != nil {
					return jf.ErrResult("Failed to seek: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Seeked to position %d ticks.", *args.SeekTicks)), nil, nil

			case "Mute", "Unmute", "ToggleMute":
				endpoint := fmt.Sprintf("/Sessions/%s/System/%s", sessionID, args.Command)
				if err := client.PostNoContent(ctx, endpoint, nil, nil); err != nil {
					return jf.ErrResult("Failed to send command: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Command '%s' sent to session.", args.Command)), nil, nil

			case "SetVolume":
				if args.Volume == nil {
					return jf.ErrResult("volume (0-100) is required for SetVolume command."), nil, nil
				}
				endpoint := fmt.Sprintf("/Sessions/%s/System/SetVolume", sessionID)
				params := url.Values{"volume": {fmt.Sprintf("%d", *args.Volume)}}
				if err := client.PostNoContent(ctx, endpoint, params, nil); err != nil {
					return jf.ErrResult("Failed to set volume: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Volume set to %d.", *args.Volume)), nil, nil

			case "SendMessage":
				if args.Message == "" {
					return jf.ErrResult("message text is required for SendMessage command."), nil, nil
				}
				endpoint := fmt.Sprintf("/Sessions/%s/Message", sessionID)
				body := map[string]any{"Text": args.Message}
				if err := client.PostNoContent(ctx, endpoint, nil, body); err != nil {
					return jf.ErrResult("Failed to send message: %v", err), nil, nil
				}
				return jf.TextResult("Message sent to session."), nil, nil

			case "GoHome", "GoToSettings", "ChannelUp", "ChannelDown":
				endpoint := fmt.Sprintf("/Sessions/%s/System/%s", sessionID, args.Command)
				if err := client.PostNoContent(ctx, endpoint, nil, nil); err != nil {
					return jf.ErrResult("Failed to send command: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Command '%s' sent to session.", args.Command)), nil, nil

			case "DisplayContent":
				if args.ItemID == "" {
					return jf.ErrResult("item_id is required for DisplayContent. Use jellyfin_search to find item IDs."), nil, nil
				}
				// Fetch item to get required type and name for the Viewing endpoint
				userID, err := client.GetUserID(ctx)
				if err != nil {
					return jf.ErrResult("Jellyfin error: %v", err), nil, nil
				}
				var item map[string]any
				itemEndpoint := fmt.Sprintf("/Users/%s/Items/%s", jf.SanitizeID(userID), jf.SanitizeID(args.ItemID))
				if err := client.Get(ctx, itemEndpoint, nil, &item); err != nil {
					return jf.ErrResultWithHint("Use jellyfin_search to find valid item IDs.", "Failed to look up item: %v", err), nil, nil
				}
				endpoint := fmt.Sprintf("/Sessions/%s/Viewing", sessionID)
				params := url.Values{
					"itemType": {jf.GetString(item, "Type")},
					"itemId":   {args.ItemID},
					"itemName": {jf.GetString(item, "Name")},
				}
				if err := client.PostNoContent(ctx, endpoint, params, nil); err != nil {
					return jf.ErrResult("Failed to display content: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Client navigated to '%s'.", jf.GetString(item, "Name"))), nil, nil

			default:
				return jf.ErrResult("Invalid command '%s'. Valid commands: Pause, Unpause, Stop, NextTrack, PreviousTrack, Seek, Mute, Unmute, ToggleMute, SetVolume, SendMessage, GoHome, GoToSettings, ChannelUp, ChannelDown, DisplayContent", args.Command), nil, nil
			}
		})
	}

	// --- jellyfin_play ---
	if enabled("jellyfin_play", AnnotWriteOp) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_play",
			Title: "Play Media",
			InputSchema: jf.WithEnums[jf.PlayInput](map[string][]any{
				"play_command": {"PlayNow", "PlayNext", "PlayLast"},
			}),
			Description: "Start playback of items on a client session. Sends a list of item IDs to play on the specified session. " +
				"Use play_command 'PlayNow' (default) to replace the current queue and start playing, 'PlayNext' to insert after the current item, " +
				"or 'PlayLast' to append to the end of the queue. Use start_index to begin from a specific item in the list. " +
				"The session_id must come from jellyfin_sessions (list action). Item IDs come from search, browse, or playlist results.",
			Annotations: AnnotWriteOp,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.PlayInput) (*mcp.CallToolResult, any, error) {
			if len(args.ItemIDs) == 0 {
				return jf.ErrResult("item_ids is required. Provide at least one item ID to play."), nil, nil
			}

			playCmd := args.PlayCommand
			if playCmd == "" {
				playCmd = "PlayNow"
			}

			params := url.Values{
				"PlayCommand": {playCmd},
			}
			for _, itemID := range args.ItemIDs {
				params.Add("ItemIds", itemID)
			}
			if args.StartIndex != nil {
				params.Set("StartIndex", fmt.Sprintf("%d", *args.StartIndex))
			}
			endpoint := fmt.Sprintf("/Sessions/%s/Playing", jf.SanitizeID(args.SessionID))

			if err := client.PostNoContent(ctx, endpoint, params, nil); err != nil {
				return jf.ErrResult("Failed to start playback: %v. Verify the session_id is valid using jellyfin_sessions.", err), nil, nil
			}
			return jf.TextResult(fmt.Sprintf("Playback started: %d items queued with command '%s'.", len(args.ItemIDs), playCmd)), nil, nil
		})
	}

	// --- jellyfin_syncplay ---
	if enabled("jellyfin_syncplay", AnnotWriteCreate) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_syncplay",
			Title: "SyncPlay",
			InputSchema: jf.WithEnums[jf.SyncPlayInput](map[string][]any{
				"action": {"list", "new", "join", "leave", "play", "pause", "seek", "stop", "queue", "next", "previous", "set_repeat", "set_shuffle"},
			}),
			Description: "Manage SyncPlay watch-together sessions for synchronized playback across multiple users. " +
				"Use 'list' to see active groups, 'new' to create a group, 'join' to join an existing group (requires group_id), and 'leave' to exit. " +
				"Transport controls: 'play', 'pause', 'seek' (requires seek_position_ticks), 'stop', 'next', 'previous'. " +
				"Queue management: 'queue' to add items (requires item_ids). Modes: 'set_repeat' and 'set_shuffle' (requires mode parameter).",
			Annotations: AnnotWriteCreate,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.SyncPlayInput) (*mcp.CallToolResult, any, error) {
			switch args.Action {
			case "list":
				var groups []map[string]any
				if err := client.Get(ctx, "/SyncPlay/List", nil, &groups); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("SyncPlay groups (%d):\n\n%s", len(groups), jf.FormatJSON(groups))), nil, nil

			case "new":
				body := map[string]any{"GroupName": "SyncPlay Group"}
				if err := client.PostNoContent(ctx, "/SyncPlay/New", nil, body); err != nil {
					return jf.ErrResult("Failed to create SyncPlay group: %v", err), nil, nil
				}
				return jf.TextResult("SyncPlay group created."), nil, nil

			case "join":
				if args.GroupID == "" {
					return jf.ErrResult("group_id is required. Use 'list' to find active SyncPlay groups."), nil, nil
				}
				body := map[string]any{"GroupId": args.GroupID}
				if err := client.PostNoContent(ctx, "/SyncPlay/Join", nil, body); err != nil {
					return jf.ErrResult("Failed to join group: %v", err), nil, nil
				}
				return jf.TextResult("Joined SyncPlay group."), nil, nil

			case "leave":
				if err := client.PostNoContent(ctx, "/SyncPlay/Leave", nil, nil); err != nil {
					return jf.ErrResult("Failed to leave group: %v", err), nil, nil
				}
				return jf.TextResult("Left SyncPlay group."), nil, nil

			case "play":
				if err := client.PostNoContent(ctx, "/SyncPlay/Unpause", nil, nil); err != nil {
					return jf.ErrResult("Failed: %v", err), nil, nil
				}
				return jf.TextResult("SyncPlay: resumed playback."), nil, nil

			case "pause":
				if err := client.PostNoContent(ctx, "/SyncPlay/Pause", nil, nil); err != nil {
					return jf.ErrResult("Failed: %v", err), nil, nil
				}
				return jf.TextResult("SyncPlay: paused."), nil, nil

			case "seek":
				if args.SeekTicks == nil {
					return jf.ErrResult("seek_position_ticks is required for seek."), nil, nil
				}
				body := map[string]any{"PositionTicks": *args.SeekTicks}
				if err := client.PostNoContent(ctx, "/SyncPlay/Seek", nil, body); err != nil {
					return jf.ErrResult("Failed to seek: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("SyncPlay: seeked to %d ticks.", *args.SeekTicks)), nil, nil

			case "stop":
				if err := client.PostNoContent(ctx, "/SyncPlay/Stop", nil, nil); err != nil {
					return jf.ErrResult("Failed: %v", err), nil, nil
				}
				return jf.TextResult("SyncPlay: stopped."), nil, nil

			case "queue":
				if len(args.ItemIDs) == 0 {
					return jf.ErrResult("item_ids is required for queue action."), nil, nil
				}
				body := map[string]any{"ItemIds": args.ItemIDs, "Mode": "Queue"}
				if err := client.PostNoContent(ctx, "/SyncPlay/Queue", nil, body); err != nil {
					return jf.ErrResult("Failed to queue items: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Queued %d items in SyncPlay.", len(args.ItemIDs))), nil, nil

			case "next":
				if err := client.PostNoContent(ctx, "/SyncPlay/NextItem", nil, map[string]any{"PlaylistItemId": ""}); err != nil {
					return jf.ErrResult("Failed: %v", err), nil, nil
				}
				return jf.TextResult("SyncPlay: skipped to next."), nil, nil

			case "previous":
				if err := client.PostNoContent(ctx, "/SyncPlay/PreviousItem", nil, map[string]any{"PlaylistItemId": ""}); err != nil {
					return jf.ErrResult("Failed: %v", err), nil, nil
				}
				return jf.TextResult("SyncPlay: went to previous."), nil, nil

			case "set_repeat":
				if args.Mode == "" {
					return jf.ErrResult("mode is required: RepeatNone, RepeatAll, or RepeatOne"), nil, nil
				}
				body := map[string]any{"Mode": args.Mode}
				if err := client.PostNoContent(ctx, "/SyncPlay/SetRepeatMode", nil, body); err != nil {
					return jf.ErrResult("Failed: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("SyncPlay repeat mode set to: %s", args.Mode)), nil, nil

			case "set_shuffle":
				if args.Mode == "" {
					return jf.ErrResult("mode is required: Sorted or Shuffle"), nil, nil
				}
				body := map[string]any{"Mode": args.Mode}
				if err := client.PostNoContent(ctx, "/SyncPlay/SetShuffleMode", nil, body); err != nil {
					return jf.ErrResult("Failed: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("SyncPlay shuffle mode set to: %s", args.Mode)), nil, nil

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: list, new, join, leave, play, pause, seek, stop, queue, next, previous, set_repeat, set_shuffle", args.Action), nil, nil
			}
		})
	}
}
