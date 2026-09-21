package tools

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	jf "github.com/jaredtrent/jellyfin-mcp/internal/jellyfin"
)

func RegisterAdminSystemTools(server *mcp.Server, client jf.Client, enabled func(string, *mcp.ToolAnnotations) bool) {

	// --- jellyfin_system_info (read-only system queries) ---
	if enabled("jellyfin_system_info", AnnotReadOnly) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_system_info",
			Title: "System Info",
			InputSchema: jf.WithEnums[jf.SystemInfoInput](map[string][]any{
				"action": {"whoami", "info", "storage", "activity_log", "ping", "logs", "log_file", "playback_history", "health_check"},
			}),
			Description: "Get server information and status. Actions cover user identity, server info, storage, activity logs, ping, log files, and playback history." +
				"\n\nAction notes:" +
				"\n- activity_log: Shows user-level events (logins, playback start/stop, session activity) — NOT server errors. For server errors and warnings, use the log_file action instead." +
				"\n- playback_history: Returns items with play counts, last-played dates, and an actual_playback flag (true = verified playback session in activity log, false = manually marked as watched or playback event expired from log). When the user asks 'what did I watch/last watch', use default verified_only=true. When they ask 'what's marked as watched', set verified_only=false to include manually marked items." +
				"\n- logs: Lists available log files. Tip: most server logs are named log_*.log — FFmpeg transcode logs are named FFmpeg.*.log." +
				"\n- log_file: Reads the content of a specific log file. Look for [WRN] and [ERR] tags for warnings and errors.",
			Annotations: AnnotReadOnly,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.SystemInfoInput) (*mcp.CallToolResult, any, error) {
			switch args.Action {
			case "whoami":
				userID, err := client.GetUserID(ctx)
				if err != nil {
					return jf.ErrResult("Jellyfin error: %v", err), nil, nil
				}
				var user map[string]any
				endpoint := fmt.Sprintf("/Users/%s", jf.SanitizeID(userID))
				if err := client.Get(ctx, endpoint, nil, &user); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				result := map[string]any{
					"user_id":  userID,
					"username": jf.GetString(user, "Name"),
				}
				if policy := jf.ToMap(user["Policy"]); policy != nil {
					result["is_admin"] = jf.GetBool(policy, "IsAdministrator")
					result["enable_all_folders"] = jf.GetBool(policy, "EnableAllFolders")
					if ids := jf.ToStringSlice(policy["EnabledFolders"]); len(ids) > 0 {
						// Resolve folder IDs to names
						var libs []map[string]any
						if err := client.Get(ctx, "/Library/VirtualFolders", nil, &libs); err == nil {
							nameMap := make(map[string]string, len(libs))
							for _, lib := range libs {
								nameMap[jf.GetString(lib, "ItemId")] = jf.GetString(lib, "Name")
							}
							resolved := make([]map[string]any, 0, len(ids))
							for _, id := range ids {
								entry := map[string]any{"id": id}
								if name, ok := nameMap[id]; ok {
									entry["name"] = name
								}
								resolved = append(resolved, entry)
							}
							result["enabled_folders"] = resolved
						} else {
							result["enabled_folder_ids"] = ids
						}
					}
					result["max_parental_rating"] = jf.GetInt(policy, "MaxParentalRating")
				}
				if la := jf.GetString(user, "LastActivityDate"); la != "" {
					result["last_activity"] = jf.Truncate(la, jf.DateTimeLen)
				}
				if ll := jf.GetString(user, "LastLoginDate"); ll != "" {
					result["last_login"] = jf.Truncate(ll, jf.DateTimeLen)
				}
				return jf.TextResult(fmt.Sprintf("Current user:\n\n%s", jf.FormatJSON(result))), nil, nil

			case "info":
				var info map[string]any
				if err := client.Get(ctx, "/System/Info", nil, &info); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				os := jf.GetString(info, "OperatingSystem")
				if os == "" {
					os = jf.GetString(info, "OperatingSystemDisplayName")
				}
				result := map[string]any{
					"server_name":              jf.GetString(info, "ServerName"),
					"version":                  jf.GetString(info, "Version"),
					"os":                       os,
					"id":                       jf.GetString(info, "Id"),
					"startup_wizard_completed": jf.GetBool(info, "StartupWizardCompleted"),
					"has_pending_restart":      jf.GetBool(info, "HasPendingRestart"),
					"has_update_available":     jf.GetBool(info, "HasUpdateAvailable"),
				}
				if lo := jf.GetString(info, "LocalAddress"); lo != "" {
					result["local_address"] = lo
				}
				if pkg := jf.GetString(info, "PackageName"); pkg != "" {
					result["package_name"] = pkg
				}
				if jf.GetBool(info, "CanSelfRestart") {
					result["can_self_restart"] = true
				}
				return jf.TextResult(jf.FormatJSON(result)), nil, nil

			case "storage":
				return handleStorage(ctx, client)

			case "activity_log":
				maxItems := jf.ClampInt(args.Limit, 200, jf.MaxLimitCap)
				params := url.Values{}
				if args.MinDate != "" {
					params.Set("MinDate", args.MinDate)
				}
				rawItems, _, err := jf.FetchAllPages(ctx, client, "/System/ActivityLog/Entries", params, maxItems)
				if err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				entries := make([]map[string]any, 0, len(rawItems))
				typeFilter := args.Type
				userFilter := args.UserID
				for _, raw := range rawItems {
					m := jf.ToMap(raw)
					entryType := jf.GetString(m, "Type")
					// Client-side type filter
					if typeFilter != "" && entryType != typeFilter {
						continue
					}
					// Client-side user filter
					if userFilter != "" && jf.GetString(m, "UserId") != userFilter {
						continue
					}
					entry := map[string]any{
						"date":     jf.Truncate(jf.GetString(m, "Date"), jf.DateTimeLen),
						"name":     jf.GetString(m, "Name"),
						"type":     entryType,
						"severity": jf.GetString(m, "Severity"),
					}
					if overview := jf.GetString(m, "Overview"); overview != "" {
						entry["overview"] = jf.Truncate(overview, jf.OverviewMaxLen)
					}
					if uid := jf.GetString(m, "UserId"); uid != "" {
						entry["user_id"] = uid
					}
					if itemID := jf.GetString(m, "ItemId"); itemID != "" {
						entry["item_id"] = itemID
					}
					entries = append(entries, entry)
				}
				header := fmt.Sprintf("Activity log (%d entries", len(entries))
				if typeFilter != "" {
					header += fmt.Sprintf(", type=%s", typeFilter)
				}
				if args.MinDate != "" {
					header += fmt.Sprintf(", since %s", args.MinDate)
				}
				header += ")"
				return jf.TextResult(fmt.Sprintf("%s:\n\n%s", header, jf.FormatJSON(entries))), nil, nil

			case "ping":
				if err := client.PostNoContent(ctx, "/System/Ping", nil, nil); err != nil {
					return jf.ErrResult("Ping failed: %v", err), nil, nil
				}
				return jf.TextResult("Server is responsive."), nil, nil

			case "logs":
				var logs []map[string]any
				if err := client.Get(ctx, "/System/Logs", nil, &logs); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				// Categorize and filter log files
				logType := strings.ToLower(args.LogType)
				if logType == "" {
					logType = "main"
				}
				var mainCount, ffmpegCount int
				entries := make([]map[string]any, 0, len(logs))
				for _, l := range logs {
					name := jf.GetString(l, "Name")
					isMain := strings.HasPrefix(name, "log_") && strings.HasSuffix(name, ".log")
					isFFmpeg := strings.HasPrefix(name, "FFmpeg")
					if isMain {
						mainCount++
					}
					if isFFmpeg {
						ffmpegCount++
					}
					// Apply type filter
					switch logType {
					case "main":
						if !isMain {
							continue
						}
					case "ffmpeg":
						if !isFFmpeg {
							continue
						}
					}
					entries = append(entries, map[string]any{
						"name":          name,
						"date_modified": jf.Truncate(jf.GetString(l, "DateModified"), jf.DateTimeLen),
						"size":          jf.GetInt64(l, "Size"),
					})
				}
				// Sort by date_modified descending
				sort.Slice(entries, func(i, j int) bool {
					return jf.GetString(entries[i], "date_modified") > jf.GetString(entries[j], "date_modified")
				})
				limit := jf.ClampInt(args.Limit, 25, jf.MaxLimitCap)
				if len(entries) > limit {
					entries = entries[:limit]
				}
				header := fmt.Sprintf("Log files (%d main, %d ffmpeg", mainCount, ffmpegCount)
				otherCount := len(logs) - mainCount - ffmpegCount
				if otherCount > 0 {
					header += fmt.Sprintf(", %d other", otherCount)
				}
				header += fmt.Sprintf(", showing %d with type=%s)", len(entries), logType)
				return jf.TextResult(fmt.Sprintf("%s:\n\n%s", header, jf.FormatJSON(entries))), nil, nil

			case "log_file":
				if args.Name == "" {
					return jf.ErrResult("name is required for log_file. Use 'logs' action to list available log files."), nil, nil
				}
				params := url.Values{"name": {args.Name}}
				logContent, err := client.GetRaw(ctx, "/System/Logs/Log", params)
				if err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				limit := jf.ClampInt(args.Limit, 200, jf.MaxLimitCap)
				lines := strings.Split(logContent, "\n")
				// Apply severity filter before tail limit
				severity := strings.ToLower(args.Severity)
				if severity != "" && severity != "all" {
					filtered := make([]string, 0, len(lines))
					for _, line := range lines {
						switch severity {
						case "warn":
							if strings.Contains(line, "[WRN]") {
								filtered = append(filtered, line)
							}
						case "error":
							if strings.Contains(line, "[ERR]") {
								filtered = append(filtered, line)
							}
						case "warn+error":
							if strings.Contains(line, "[WRN]") || strings.Contains(line, "[ERR]") {
								filtered = append(filtered, line)
							}
						}
					}
					totalMatches := len(filtered)
					if len(filtered) > limit {
						filtered = filtered[len(filtered)-limit:]
					}
					return jf.TextResult(fmt.Sprintf("Log file '%s' (severity=%s, %d matches, showing last %d):\n\n%s", args.Name, severity, totalMatches, len(filtered), strings.Join(filtered, "\n"))), nil, nil
				}
				// No severity filter — tail the raw output
				if len(lines) > limit {
					lines = lines[len(lines)-limit:]
				}
				return jf.TextResult(fmt.Sprintf("Log file '%s' (last %d lines):\n\n%s", args.Name, len(lines), strings.Join(lines, "\n"))), nil, nil

			case "playback_history":
				return handlePlaybackHistory(ctx, client, args)

			case "health_check":
				return handleHealthCheck(ctx, client)

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: whoami, info, storage, activity_log, ping, logs, log_file, playback_history, health_check", args.Action), nil, nil
			}
		})
	}

	// --- jellyfin_system_control (destructive server operations) ---
	if enabled("jellyfin_system_control", AnnotDestructive) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_system_control",
			Title: "Server Control",
			InputSchema: jf.WithEnums[jf.SystemControlInput](map[string][]any{
				"action": {"restart", "shutdown"},
			}),
			Description: "Restart or shut down the Jellyfin server. These are destructive operations that cannot be undone. " +
				"Use 'restart' to restart the server (it will be temporarily unavailable). " +
				"Use 'shutdown' to stop the server completely (must be manually restarted).",
			Annotations: AnnotDestructive,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.SystemControlInput) (*mcp.CallToolResult, any, error) {
			switch args.Action {
			case "restart":
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, "This will RESTART the Jellyfin server. It will be temporarily unavailable during the restart."); result != nil {
					return result, nil, nil
				}
				if err := client.PostNoContent(ctx, "/System/Restart", nil, nil); err != nil {
					return jf.ErrResult("Failed to restart: %v", err), nil, nil
				}
				return jf.TextResult("Server restart initiated. The server will be temporarily unavailable."), nil, nil

			case "shutdown":
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, "This will SHUT DOWN the Jellyfin server completely. It must be manually restarted afterward."); result != nil {
					return result, nil, nil
				}
				if err := client.PostNoContent(ctx, "/System/Shutdown", nil, nil); err != nil {
					return jf.ErrResult("Failed to shutdown: %v", err), nil, nil
				}
				return jf.TextResult("Server shutdown initiated. The server will stop and must be manually restarted."), nil, nil

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: restart, shutdown", args.Action), nil, nil
			}
		})
	}
}

func handleStorage(ctx context.Context, client jf.Client) (*mcp.CallToolResult, any, error) {
	var info map[string]any
	if err := client.Get(ctx, "/System/Info/Storage", nil, &info); err != nil {
		if err2 := client.Get(ctx, "/System/Info", nil, &info); err2 != nil {
			return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
		}
	}
	type mountEntry struct {
		FreeSpace int64
		Paths     []string
		Libraries []string
	}
	mounts := make(map[string]*mountEntry)
	addMount := func(freeSpace int64, path, label string) {
		m, ok := mounts[path]
		if !ok {
			m = &mountEntry{FreeSpace: freeSpace}
			mounts[path] = m
			m.Paths = append(m.Paths, path)
		}
		if label != "" {
			m.Libraries = append(m.Libraries, label)
		}
	}
	for _, key := range []string{"ProgramDataFolder", "CacheFolder", "LogFolder", "TranscodingTempFolder"} {
		if folder := jf.ToMap(info[key]); folder != nil {
			addMount(jf.GetInt64(folder, "FreeSpace"), jf.GetString(folder, "Path"), "")
		}
	}
	if libs := jf.ToSlice(info["Libraries"]); len(libs) > 0 {
		for _, lib := range libs {
			lm := jf.ToMap(lib)
			libName := jf.GetString(lm, "Name")
			if folders := jf.ToSlice(lm["Folders"]); len(folders) > 0 {
				for _, f := range folders {
					fm := jf.ToMap(f)
					addMount(jf.GetInt64(fm, "FreeSpace"), jf.GetString(fm, "Path"), libName)
				}
			}
		}
	}
	mountList := make([]map[string]any, 0, len(mounts))
	for _, m := range mounts {
		entry := map[string]any{
			"free":       jf.FormatGB(m.FreeSpace),
			"free_bytes": m.FreeSpace,
			"paths":      m.Paths,
		}
		if m.FreeSpace < jf.LowStorageThreshold {
			entry["low_space"] = true
		}
		if len(m.Libraries) > 0 {
			entry["libraries"] = m.Libraries
		}
		mountList = append(mountList, entry)
	}
	if len(mountList) == 0 {
		return jf.TextResult(jf.FormatJSON(info)), nil, nil
	}
	return jf.TextResult(fmt.Sprintf("Storage (%d mounts):\n\n%s", len(mountList), jf.FormatJSON(mountList))), nil, nil
}

func handlePlaybackHistory(ctx context.Context, client jf.Client, args jf.SystemInfoInput) (*mcp.CallToolResult, any, error) {
	userID, err := client.GetUserID(ctx)
	if err != nil {
		return jf.ErrResult("Jellyfin error: %v", err), nil, nil
	}
	maxItems := jf.ClampInt(args.Limit, 500, jf.MaxLimitCap)
	targetUser := userID
	if args.UserID != "" {
		targetUser = jf.SanitizeID(args.UserID)
	}
	params := url.Values{
		"Recursive": {"true"},
		"Filters":   {"IsPlayed"},
		"SortBy":    {"DatePlayed"},
		"SortOrder": {"Descending"},
		"Fields":    {"Overview,ProductionYear,RunTimeTicks,UserData"},
	}
	endpoint := fmt.Sprintf("/Users/%s/Items", jf.SanitizeID(targetUser))
	rawItems, total, err := jf.FetchAllPages(ctx, client, endpoint, params, maxItems)
	if err != nil {
		return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
	}

	playedViaPlayback := map[string]bool{}
	logParams := url.Values{}
	logItems, _, logErr := jf.FetchAllPages(ctx, client, "/System/ActivityLog/Entries", logParams, jf.ActivityLogLookback)
	if logErr == nil {
		for _, raw := range logItems {
			m := jf.ToMap(raw)
			if jf.GetString(m, "Type") != "VideoPlaybackStopped" {
				continue
			}
			if jf.GetString(m, "UserId") != targetUser {
				continue
			}
			if itemID := jf.GetString(m, "ItemId"); itemID != "" {
				playedViaPlayback[itemID] = true
			}
		}
	}

	verifiedOnly := args.VerifiedOnly == nil || *args.VerifiedOnly

	entries := make([]map[string]any, 0, len(rawItems))
	for _, raw := range rawItems {
		m := jf.ToMap(raw)
		entry := jf.ExtractMediaItem(m)
		if ud := jf.ToMap(m["UserData"]); ud != nil {
			if lp := jf.GetString(ud, "LastPlayedDate"); lp != "" {
				entry["last_played"] = jf.Truncate(lp, jf.DateTimeLen)
			}
			if pc := jf.GetInt(ud, "PlayCount"); pc > 0 {
				entry["play_count"] = pc
			}
			if pct := jf.GetFloat(ud, "PlayedPercentage"); pct > 0 {
				entry["progress"] = fmt.Sprintf("%.0f%%", pct)
			}
			entry["completed"] = jf.GetBool(ud, "Played")
		}
		id, _ := entry["id"].(string)
		actual := playedViaPlayback[id]
		entry["actual_playback"] = actual
		if verifiedOnly && !actual {
			continue
		}
		entries = append(entries, entry)
	}
	header := fmt.Sprintf("Playback history (%d items", len(entries))
	if verifiedOnly {
		header += fmt.Sprintf(", verified_only=true, %d total marked played", total)
	} else {
		header += fmt.Sprintf(" of %d total", total)
	}
	header += ")"
	return jf.TextResult(fmt.Sprintf("%s:\n\n%s", header, jf.FormatJSON(entries))), nil, nil
}

func handleHealthCheck(ctx context.Context, client jf.Client) (*mcp.CallToolResult, any, error) {
	var (
		sysInfo     map[string]any
		tasks       []map[string]any
		plugins     []map[string]any
		logs        []map[string]any
		backups     any
		storageInfo map[string]any
		sysInfoErr  error
		tasksErr    error
		pluginsErr  error
		logsErr     error
		backupsErr  error
		storageErr  error
	)
	var wg sync.WaitGroup
	wg.Add(6)
	go func() { defer wg.Done(); sysInfoErr = client.Get(ctx, "/System/Info", nil, &sysInfo) }()
	go func() { defer wg.Done(); tasksErr = client.Get(ctx, "/ScheduledTasks", nil, &tasks) }()
	go func() { defer wg.Done(); pluginsErr = client.Get(ctx, "/Plugins", nil, &plugins) }()
	go func() { defer wg.Done(); logsErr = client.Get(ctx, "/System/Logs", nil, &logs) }()
	go func() { defer wg.Done(); backupsErr = client.Get(ctx, "/Backup", nil, &backups) }()
	go func() { defer wg.Done(); storageErr = client.Get(ctx, "/System/Info/Storage", nil, &storageInfo) }()
	wg.Wait()

	overallStatus := "healthy"
	report := map[string]any{}

	if sysInfoErr == nil {
		osName := jf.GetString(sysInfo, "OperatingSystem")
		if osName == "" {
			osName = jf.GetString(sysInfo, "OperatingSystemDisplayName")
		}
		serverSection := map[string]any{
			"version":              jf.GetString(sysInfo, "Version"),
			"server_name":          jf.GetString(sysInfo, "ServerName"),
			"os":                   osName,
			"has_pending_restart":  jf.GetBool(sysInfo, "HasPendingRestart"),
			"has_update_available": jf.GetBool(sysInfo, "HasUpdateAvailable"),
		}
		if jf.GetBool(sysInfo, "HasPendingRestart") {
			overallStatus = "warnings"
		}
		report["server"] = serverSection
	}

	if storageErr == nil {
		storageHealthy := true
		mountSummary := make([]map[string]any, 0)
		seen := make(map[string]bool)
		checkMount := func(freeSpace int64, path string) {
			if seen[path] {
				return
			}
			seen[path] = true
			entry := map[string]any{
				"path": path,
				"free": jf.FormatGB(freeSpace),
			}
			if freeSpace < jf.LowStorageThreshold {
				entry["low_space"] = true
				storageHealthy = false
			}
			mountSummary = append(mountSummary, entry)
		}
		for _, key := range []string{"ProgramDataFolder", "CacheFolder", "TranscodingTempFolder"} {
			if folder := jf.ToMap(storageInfo[key]); folder != nil {
				checkMount(jf.GetInt64(folder, "FreeSpace"), jf.GetString(folder, "Path"))
			}
		}
		if libs := jf.ToSlice(storageInfo["Libraries"]); len(libs) > 0 {
			for _, lib := range libs {
				lm := jf.ToMap(lib)
				if folders := jf.ToSlice(lm["Folders"]); len(folders) > 0 {
					for _, f := range folders {
						fm := jf.ToMap(f)
						checkMount(jf.GetInt64(fm, "FreeSpace"), jf.GetString(fm, "Path"))
					}
				}
			}
		}
		if !storageHealthy && overallStatus == "healthy" {
			overallStatus = "warnings"
		}
		report["storage"] = map[string]any{
			"healthy": storageHealthy,
			"mounts":  mountSummary,
		}
	}

	if tasksErr == nil {
		failedTasks := make([]string, 0)
		runningTasks := make([]string, 0)
		for _, t := range tasks {
			name := jf.GetString(t, "Name")
			state := jf.GetString(t, "State")
			if state == "Running" {
				runningTasks = append(runningTasks, name)
			}
			if le := jf.ToMap(t["LastExecutionResult"]); le != nil {
				status := jf.GetString(le, "Status")
				if status != "" && status != "Completed" && status != "Aborted" {
					failedTasks = append(failedTasks, fmt.Sprintf("%s (%s)", name, status))
					if overallStatus == "healthy" {
						overallStatus = "warnings"
					}
				}
			}
		}
		report["tasks"] = map[string]any{
			"failed":  failedTasks,
			"running": runningTasks,
		}
	}

	if pluginsErr == nil {
		needsRestart := make([]string, 0)
		disabled := make([]string, 0)
		for _, p := range plugins {
			name := jf.GetString(p, "Name")
			status := jf.GetString(p, "Status")
			version := jf.GetString(p, "Version")
			if status == "Restart" || status == "Superseded" {
				needsRestart = append(needsRestart, fmt.Sprintf("%s (%s)", name, version))
				if overallStatus == "healthy" {
					overallStatus = "warnings"
				}
			}
			if status == "Disabled" {
				disabled = append(disabled, name)
			}
		}
		report["plugins"] = map[string]any{
			"needs_restart": needsRestart,
			"disabled":      disabled,
		}
	}

	if logsErr == nil {
		var latestLog string
		var latestDate string
		for _, l := range logs {
			name := jf.GetString(l, "Name")
			if strings.HasPrefix(name, "log_") && strings.HasSuffix(name, ".log") {
				date := jf.GetString(l, "DateModified")
				if date > latestDate {
					latestDate = date
					latestLog = name
				}
			}
		}
		if latestLog != "" {
			params := url.Values{"name": {latestLog}}
			if logContent, err := client.GetRaw(ctx, "/System/Logs/Log", params); err == nil {
				logLines := strings.Split(logContent, "\n")
				var warnCount, errCount int
				var allIssues []string
				for _, line := range logLines {
					if strings.Contains(line, "[WRN]") {
						warnCount++
						allIssues = append(allIssues, jf.Truncate(line, jf.OverviewMaxLen))
					} else if strings.Contains(line, "[ERR]") {
						errCount++
						if overallStatus != "errors" {
							overallStatus = "errors"
						}
						allIssues = append(allIssues, jf.Truncate(line, jf.OverviewMaxLen))
					}
				}
				lastIssues := allIssues
				if len(lastIssues) > jf.HealthCheckMaxIssues {
					lastIssues = lastIssues[len(lastIssues)-jf.HealthCheckMaxIssues:]
				}
				report["recent_log_issues"] = map[string]any{
					"log_file":    latestLog,
					"warnings":    warnCount,
					"errors":      errCount,
					"last_issues": lastIssues,
				}
			}
		}
	}

	if backupsErr == nil {
		backupList := jf.ToSlice(backups)
		if len(backupList) > 0 {
			var latestBackup map[string]any
			var latestBackupDate string
			for _, b := range backupList {
				bm := jf.ToMap(b)
				date := jf.GetString(bm, "DateCreated")
				if date > latestBackupDate {
					latestBackupDate = date
					latestBackup = bm
				}
			}
			if latestBackup != nil {
				backupHealthy := true
				backupDate := jf.Truncate(latestBackupDate, jf.DateOnlyLen)
				if parsed, err := time.Parse(jf.DateOnlyFormat, backupDate); err == nil {
					ageDays := int(time.Since(parsed).Hours() / 24)
					backupSection := map[string]any{
						"last_backup":          backupDate,
						"last_backup_age_days": ageDays,
					}
					if version := jf.GetString(latestBackup, "ServerVersion"); version != "" {
						backupSection["backup_server_version"] = version
					}
					if ageDays > jf.BackupStaleDays {
						backupHealthy = false
						if overallStatus == "healthy" {
							overallStatus = "warnings"
						}
					}
					backupSection["healthy"] = backupHealthy
					report["backups"] = backupSection
				}
			}
		} else {
			report["backups"] = map[string]any{"healthy": false, "note": "no backups found"}
			if overallStatus == "healthy" {
				overallStatus = "warnings"
			}
		}
	} else {
		report["backups"] = map[string]any{"note": "backup API not available (requires Jellyfin 10.11+)"}
	}

	report["status"] = overallStatus
	return jf.TextResult(fmt.Sprintf("Health check — %s:\n\n%s", overallStatus, jf.FormatJSON(report))), nil, nil
}
