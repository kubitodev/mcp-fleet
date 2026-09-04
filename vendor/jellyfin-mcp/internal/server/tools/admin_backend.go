package tools

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	jf "github.com/jaredtrent/jellyfin-mcp/internal/jellyfin"
)

func RegisterAdminBackendTools(server *mcp.Server, client jf.Client, enabled func(string, *mcp.ToolAnnotations) bool) {

	// --- jellyfin_tasks ---
	if enabled("jellyfin_tasks", AnnotWriteOp) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_tasks",
			Title: "Scheduled Tasks",
			InputSchema: jf.WithEnums[jf.TasksInput](map[string][]any{
				"action": {"list", "get", "start", "stop", "set_triggers"},
			}),
			Description: "View and manage scheduled tasks such as library scanning, metadata downloads, image extraction, and cache cleanup. " +
				"Use 'list' to see all tasks with their current state and last execution time. Use 'get' with a task_id for full task details. " +
				"Use 'start' to run a task immediately or 'stop' to cancel a running task. " +
				"Use 'set_triggers' to replace task triggers (requires task_id, triggers array, and confirm=true). " +
				"Trigger types: DailyTrigger, WeeklyTrigger, IntervalTrigger, StartupTrigger. Use 'get' first to see current trigger format. " +
				"Common tasks include 'Scan All Libraries', 'Download missing subtitles', and 'Clean Cache Directory'.",
			Annotations: AnnotWriteOp,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.TasksInput) (*mcp.CallToolResult, any, error) {
			switch args.Action {
			case "list":
				var tasks []map[string]any
				if err := client.Get(ctx, "/ScheduledTasks", nil, &tasks); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := make([]map[string]any, 0, len(tasks))
				for _, t := range tasks {
					task := map[string]any{
						"id":       jf.GetString(t, "Id"),
						"name":     jf.GetString(t, "Name"),
						"state":    jf.GetString(t, "State"),
						"category": jf.GetString(t, "Category"),
					}
					if le := jf.ToMap(t["LastExecutionResult"]); le != nil {
						task["last_status"] = jf.GetString(le, "Status")
						task["last_end"] = jf.Truncate(jf.GetString(le, "EndTimeUtc"), jf.DateTimeLen)
					}
					items = append(items, task)
				}
				return jf.TextResult(fmt.Sprintf("Scheduled tasks (%d):\n\n%s", len(items), jf.FormatJSON(items))), nil, nil

			case "get":
				if args.TaskID == "" {
					return jf.ErrResult("task_id is required. Use 'list' to find task IDs."), nil, nil
				}
				var task map[string]any
				endpoint := fmt.Sprintf("/ScheduledTasks/%s", jf.SanitizeID(args.TaskID))
				if err := client.Get(ctx, endpoint, nil, &task); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				return jf.TextResult(jf.FormatJSON(task)), nil, nil

			case "start":
				if args.TaskID == "" {
					return jf.ErrResult("task_id is required. Use 'list' to find task IDs."), nil, nil
				}
				endpoint := fmt.Sprintf("/ScheduledTasks/Running/%s", jf.SanitizeID(args.TaskID))
				if err := client.PostNoContent(ctx, endpoint, nil, nil); err != nil {
					return jf.ErrResult("Failed to start task: %v", err), nil, nil
				}
				return jf.TextResult("Task started."), nil, nil

			case "stop":
				if args.TaskID == "" {
					return jf.ErrResult("task_id is required."), nil, nil
				}
				endpoint := fmt.Sprintf("/ScheduledTasks/Running/%s", jf.SanitizeID(args.TaskID))
				if err := client.Del(ctx, endpoint, nil); err != nil {
					return jf.ErrResult("Failed to stop task: %v", err), nil, nil
				}
				return jf.TextResult("Task stopped."), nil, nil

			case "set_triggers":
				if args.TaskID == "" {
					return jf.ErrResult("task_id is required for set_triggers."), nil, nil
				}
				if args.Triggers == nil {
					return jf.ErrResult("triggers is required for set_triggers. Use 'get' first to see current trigger format. Types: DailyTrigger, WeeklyTrigger, IntervalTrigger, StartupTrigger."), nil, nil
				}
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, fmt.Sprintf("This will REPLACE all triggers for task '%s'.", args.TaskID)); result != nil {
					return result, nil, nil
				}
				endpoint := fmt.Sprintf("/ScheduledTasks/%s/Triggers", jf.SanitizeID(args.TaskID))
				if err := client.PostNoContent(ctx, endpoint, nil, args.Triggers); err != nil {
					return jf.ErrResult("Failed to set triggers: %v", err), nil, nil
				}
				return jf.TextResult("Task triggers updated."), nil, nil

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: list, get, start, stop, set_triggers", args.Action), nil, nil
			}
		})
	}

	// --- jellyfin_plugins ---
	if enabled("jellyfin_plugins", AnnotWriteCreate) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_plugins",
			Title: "Plugins",
			InputSchema: jf.WithEnums[jf.PluginsInput](map[string][]any{
				"action": {"list", "enable", "disable", "uninstall", "get_config", "update_config", "list_packages", "install", "list_repos", "set_repos"},
			}),
			Description: "Manage Jellyfin plugins. Use 'list' to see installed plugins with their versions and status. " +
				"Use 'enable' or 'disable' to toggle a plugin (requires plugin_id and version). Use 'uninstall' to remove a plugin. " +
				"Use 'get_config' to view a plugin's configuration. Use 'update_config' to set plugin configuration (requires plugin_id, config, and confirm=true). " +
				"Use 'list_packages' to browse available packages from repositories. " +
				"Use 'install' to install a package by name. " +
				"Use 'list_repos' to see configured plugin repositories. Use 'set_repos' to replace all repositories (requires repos array and confirm=true). " +
				"A server restart may be required after plugin changes.",
			Annotations: AnnotWriteCreate,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.PluginsInput) (*mcp.CallToolResult, any, error) {
			switch args.Action {
			case "list":
				var plugins []map[string]any
				if err := client.Get(ctx, "/Plugins", nil, &plugins); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := make([]map[string]any, 0, len(plugins))
				for _, p := range plugins {
					items = append(items, map[string]any{
						"id":      jf.GetString(p, "Id"),
						"name":    jf.GetString(p, "Name"),
						"version": jf.GetString(p, "Version"),
						"status":  jf.GetString(p, "Status"),
					})
				}
				return jf.TextResult(fmt.Sprintf("Installed plugins (%d):\n\n%s", len(items), jf.FormatJSON(items))), nil, nil

			case "enable":
				if args.PluginID == "" || args.Version == "" {
					return jf.ErrResult("plugin_id and version are required. Use 'list' to find them."), nil, nil
				}
				endpoint := fmt.Sprintf("/Plugins/%s/%s/Enable", jf.SanitizeID(args.PluginID), jf.SanitizeID(args.Version))
				if err := client.PostNoContent(ctx, endpoint, nil, nil); err != nil {
					return jf.ErrResult("Failed to enable plugin: %v", err), nil, nil
				}
				return jf.TextResult("Plugin enabled. A server restart may be required."), nil, nil

			case "disable":
				if args.PluginID == "" || args.Version == "" {
					return jf.ErrResult("plugin_id and version are required."), nil, nil
				}
				endpoint := fmt.Sprintf("/Plugins/%s/%s/Disable", jf.SanitizeID(args.PluginID), jf.SanitizeID(args.Version))
				if err := client.PostNoContent(ctx, endpoint, nil, nil); err != nil {
					return jf.ErrResult("Failed to disable plugin: %v", err), nil, nil
				}
				return jf.TextResult("Plugin disabled. A server restart may be required."), nil, nil

			case "uninstall":
				if args.PluginID == "" || args.Version == "" {
					return jf.ErrResult("plugin_id and version are required."), nil, nil
				}
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, fmt.Sprintf("This will UNINSTALL plugin '%s' (version %s). A server restart may be required.", args.PluginID, args.Version)); result != nil {
					return result, nil, nil
				}
				endpoint := fmt.Sprintf("/Plugins/%s/%s", jf.SanitizeID(args.PluginID), jf.SanitizeID(args.Version))
				if err := client.Del(ctx, endpoint, nil); err != nil {
					return jf.ErrResult("Failed to uninstall plugin: %v", err), nil, nil
				}
				return jf.TextResult("Plugin uninstalled. A server restart may be required."), nil, nil

			case "get_config":
				if args.PluginID == "" {
					return jf.ErrResult("plugin_id is required."), nil, nil
				}
				var config map[string]any
				endpoint := fmt.Sprintf("/Plugins/%s/Configuration", jf.SanitizeID(args.PluginID))
				if err := client.Get(ctx, endpoint, nil, &config); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				return jf.TextResult(jf.FormatJSON(config)), nil, nil

			case "list_packages":
				var packages []map[string]any
				if err := client.Get(ctx, "/Packages", nil, &packages); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := make([]map[string]any, 0, len(packages))
				for _, p := range packages {
					items = append(items, map[string]any{
						"name":        jf.GetString(p, "name"),
						"description": jf.Truncate(jf.GetString(p, "description"), jf.SummaryMaxLen),
						"category":    jf.GetString(p, "category"),
						"owner":       jf.GetString(p, "owner"),
					})
				}
				return jf.TextResult(fmt.Sprintf("Available packages (%d):\n\n%s", len(items), jf.FormatJSON(items))), nil, nil

			case "install":
				if args.PackageName == "" {
					return jf.ErrResult("package_name is required. Use 'list_packages' to find available packages."), nil, nil
				}
				params := url.Values{}
				if args.Version != "" {
					params.Set("version", args.Version)
				}
				if args.RepoURL != "" {
					params.Set("repositoryUrl", args.RepoURL)
				}
				endpoint := fmt.Sprintf("/Packages/Installed/%s", jf.SanitizeID(args.PackageName))
				if err := client.PostNoContent(ctx, endpoint, params, nil); err != nil {
					return jf.ErrResult("Failed to install package: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Package '%s' installation started. A server restart may be required.", args.PackageName)), nil, nil

			case "update_config":
				if args.PluginID == "" {
					return jf.ErrResult("plugin_id is required for update_config. Use 'list' to find plugin IDs."), nil, nil
				}
				if args.Config == nil {
					return jf.ErrResult("config is required for update_config. Use 'get_config' first to see the current configuration, modify it, then POST back."), nil, nil
				}
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, fmt.Sprintf("This will replace the configuration for plugin '%s'.", args.PluginID)); result != nil {
					return result, nil, nil
				}
				endpoint := fmt.Sprintf("/Plugins/%s/Configuration", jf.SanitizeID(args.PluginID))
				if err := client.PostNoContent(ctx, endpoint, nil, args.Config); err != nil {
					return jf.ErrResult("Failed to update plugin config: %v", err), nil, nil
				}
				return jf.TextResult("Plugin configuration updated."), nil, nil

			case "list_repos":
				var repos any
				if err := client.Get(ctx, "/Repositories", nil, &repos); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Plugin repositories:\n\n%s", jf.FormatJSON(repos))), nil, nil

			case "set_repos":
				if args.Repos == nil {
					return jf.ErrResult("repos is required for set_repos. Use 'list_repos' first, modify the array, then POST back. Format: [{\"Name\": \"...\", \"Url\": \"...\"}]."), nil, nil
				}
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, "This will REPLACE all configured plugin repositories."); result != nil {
					return result, nil, nil
				}
				if err := client.PostNoContent(ctx, "/Repositories", nil, args.Repos); err != nil {
					return jf.ErrResult("Failed to set repositories: %v", err), nil, nil
				}
				return jf.TextResult("Plugin repositories updated."), nil, nil

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: list, enable, disable, uninstall, get_config, update_config, list_packages, install, list_repos, set_repos", args.Action), nil, nil
			}
		})
	}

	// --- jellyfin_server ---
	if enabled("jellyfin_server", AnnotWriteOp) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_server",
			Title: "Server Configuration",
			InputSchema: jf.WithEnums[jf.ServerManageInput](map[string][]any{
				"action": {"get_config", "get_config_section", "update_config_section", "list_backups", "create_backup", "restore_backup"},
			}),
			Description: "Read and modify server configuration, and manage backups. " +
				"Use 'get_config' to read the full server configuration. " +
				"Use 'get_config_section' with key (e.g. 'encoding' for transcoding settings) to read a specific section. " +
				"Use 'update_config_section' to replace an entire config section (requires confirm=true). " +
				"Use 'list_backups' to see existing backups, 'create_backup' to create one, or 'restore_backup' to restore (requires confirm=true, triggers server restart). " +
				"Backup actions require Jellyfin 10.11+.",
			Annotations: AnnotWriteOp,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.ServerManageInput) (*mcp.CallToolResult, any, error) {
			switch args.Action {
			case "get_config":
				var config map[string]any
				if err := client.Get(ctx, "/System/Configuration", nil, &config); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Server configuration:\n\n%s", jf.FormatJSON(config))), nil, nil

			case "get_config_section":
				if args.Key == "" {
					return jf.ErrResult("key is required for get_config_section. Common keys: 'encoding' (transcoding), 'dlna', 'branding'."), nil, nil
				}
				var config any
				endpoint := fmt.Sprintf("/System/Configuration/%s", jf.SanitizeID(args.Key))
				if err := client.Get(ctx, endpoint, nil, &config); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Config section '%s':\n\n%s", args.Key, jf.FormatJSON(config))), nil, nil

			case "update_config_section":
				if args.Key == "" {
					return jf.ErrResult("key is required for update_config_section."), nil, nil
				}
				if args.Config == nil {
					return jf.ErrResult("config is required for update_config_section. This replaces the entire section — use get_config_section first to read the current values, modify, then POST back the full object."), nil, nil
				}
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, fmt.Sprintf("This will REPLACE the entire '%s' configuration section. Use get_config_section first to read current values.", args.Key)); result != nil {
					return result, nil, nil
				}
				endpoint := fmt.Sprintf("/System/Configuration/%s", jf.SanitizeID(args.Key))
				if err := client.PostNoContent(ctx, endpoint, nil, args.Config); err != nil {
					return jf.ErrResult("Failed to update config section: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Config section '%s' updated.", args.Key)), nil, nil

			case "list_backups":
				var backups any
				if err := client.Get(ctx, "/Backup", nil, &backups); err != nil {
					return jf.ErrResult("Jellyfin API error: %v. Backup support requires Jellyfin 10.11+.", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Backups:\n\n%s", jf.FormatJSON(backups))), nil, nil

			case "create_backup":
				body := map[string]any{
					"metadata":  true,
					"trickplay": true,
					"subtitles": true,
					"database":  true,
				}
				if err := client.PostNoContent(ctx, "/Backup/Create", nil, body); err != nil {
					return jf.ErrResult("Failed to create backup: %v. Backup support requires Jellyfin 10.11+.", err), nil, nil
				}
				return jf.TextResult("Backup creation started. Use 'list_backups' to check status."), nil, nil

			case "restore_backup":
				if args.FileName == "" {
					return jf.ErrResult("file_name is required for restore_backup. Use 'list_backups' to find backup filenames."), nil, nil
				}
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, fmt.Sprintf("This will RESTORE the server from backup '%s'. The server will restart and current data will be replaced.", args.FileName)); result != nil {
					return result, nil, nil
				}
				body := map[string]any{"ArchiveFileName": args.FileName}
				if err := client.PostNoContent(ctx, "/Backup/Restore", nil, body); err != nil {
					return jf.ErrResult("Failed to restore backup: %v", err), nil, nil
				}
				return jf.TextResult("Backup restore initiated. The server will restart."), nil, nil

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: get_config, get_config_section, update_config_section, list_backups, create_backup, restore_backup", args.Action), nil, nil
			}
		})
	}

	// --- jellyfin_devices ---
	if enabled("jellyfin_devices", AnnotDestructive) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_devices",
			Title: "Devices & API Keys",
			InputSchema: jf.WithEnums[jf.DevicesInput](map[string][]any{
				"action": {"list", "get", "delete", "api_keys", "create_api_key", "revoke_api_key"},
			}),
			Description: "Manage connected devices and API keys. Use 'list' to see all devices that have connected to the server. " +
				"Use 'get' for device details or 'delete' to revoke a device's access (device_id required). " +
				"Use 'api_keys' to list all API keys, 'create_api_key' to generate a new one (app_name required), " +
				"or 'revoke_api_key' to delete a key. Revoking a device or API key immediately blocks that client's access.",
			Annotations: AnnotDestructive,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.DevicesInput) (*mcp.CallToolResult, any, error) {
			switch args.Action {
			case "list":
				var result map[string]any
				if err := client.Get(ctx, "/Devices", nil, &result); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				rawItems := jf.ToSlice(result["Items"])
				totalDevices := len(rawItems)
				// Apply recency filter
				days := args.Days
				if days <= 0 {
					days = 30
				}
				cutoff := time.Now().AddDate(0, 0, -days).Format(jf.DateTimeFormat)
				items := make([]map[string]any, 0, len(rawItems))
				for _, raw := range rawItems {
					m := jf.ToMap(raw)
					lastActivity := jf.Truncate(jf.GetString(m, "DateLastActivity"), jf.DateTimeLen)
					if lastActivity < cutoff {
						continue
					}
					items = append(items, map[string]any{
						"id":            jf.GetString(m, "Id"),
						"name":          jf.GetString(m, "Name"),
						"app_name":      jf.GetString(m, "AppName"),
						"app_version":   jf.GetString(m, "AppVersion"),
						"last_user":     jf.GetString(m, "LastUserName"),
						"last_activity": lastActivity,
					})
				}
				// Sort by last_activity descending
				sort.Slice(items, func(i, j int) bool {
					return jf.GetString(items[i], "last_activity") > jf.GetString(items[j], "last_activity")
				})
				limit := jf.ClampInt(args.Limit, 50, jf.MaxLimitCap)
				if len(items) > limit {
					items = items[:limit]
				}
				return jf.TextResult(fmt.Sprintf("Devices (%d total, %d active in last %d days, showing %d):\n\n%s", totalDevices, len(items), days, len(items), jf.FormatJSON(items))), nil, nil

			case "get":
				if args.DeviceID == "" {
					return jf.ErrResult("device_id is required. Use 'list' to find device IDs."), nil, nil
				}
				params := url.Values{"id": {args.DeviceID}}
				var device map[string]any
				if err := client.Get(ctx, "/Devices/Info", params, &device); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				return jf.TextResult(jf.FormatJSON(device)), nil, nil

			case "delete":
				if args.DeviceID == "" {
					return jf.ErrResult("device_id is required."), nil, nil
				}
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, fmt.Sprintf("This will REMOVE device '%s'. It will need to re-authenticate to connect again.", args.DeviceID)); result != nil {
					return result, nil, nil
				}
				params := url.Values{"id": {args.DeviceID}}
				if err := client.Del(ctx, "/Devices", params); err != nil {
					return jf.ErrResult("Failed to delete device: %v", err), nil, nil
				}
				return jf.TextResult("Device removed. It will need to re-authenticate to connect again."), nil, nil

			case "api_keys":
				var result map[string]any
				if err := client.Get(ctx, "/Auth/Keys", nil, &result); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				rawItems := jf.ToSlice(result["Items"])
				items := make([]map[string]any, 0, len(rawItems))
				for _, raw := range rawItems {
					m := jf.ToMap(raw)
					items = append(items, map[string]any{
						"access_token": jf.MaskToken(jf.GetString(m, "AccessToken")),
						"app_name":     jf.GetString(m, "AppName"),
						"date_created": jf.Truncate(jf.GetString(m, "DateCreated"), jf.DateTimeLen),
					})
				}
				return jf.TextResult(fmt.Sprintf("API keys (%d):\n\n%s", len(items), jf.FormatJSON(items))), nil, nil

			case "create_api_key":
				if args.AppName == "" {
					return jf.ErrResult("app_name is required for create_api_key."), nil, nil
				}
				params := url.Values{"app": {args.AppName}}
				if err := client.PostNoContent(ctx, "/Auth/Keys", params, nil); err != nil {
					return jf.ErrResult("Failed to create API key: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("API key created for '%s'. Use 'api_keys' to see the key.", args.AppName)), nil, nil

			case "revoke_api_key":
				if args.Key == "" {
					return jf.ErrResult("key is required. Use 'api_keys' to find keys."), nil, nil
				}
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, "This will REVOKE the API key, immediately blocking any client using it."); result != nil {
					return result, nil, nil
				}
				endpoint := fmt.Sprintf("/Auth/Keys/%s", jf.SanitizeID(args.Key))
				if err := client.Del(ctx, endpoint, nil); err != nil {
					return jf.ErrResult("Failed to revoke API key: %v", err), nil, nil
				}
				return jf.TextResult("API key revoked."), nil, nil

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: list, get, delete, api_keys, create_api_key, revoke_api_key", args.Action), nil, nil
			}
		})
	}
}
