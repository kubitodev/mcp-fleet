package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	jf "github.com/jaredtrent/jellyfin-mcp/internal/jellyfin"
)

func RegisterContentTools(server *mcp.Server, client jf.Client, enabled func(string, *mcp.ToolAnnotations) bool) {

	// --- jellyfin_videos ---
	if enabled("jellyfin_videos", AnnotDestructive) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_videos",
			Title: "Video Versions",
			InputSchema: jf.WithEnums[jf.VideosInput](map[string][]any{
				"action": {"merge_versions", "split_versions"},
			}),
			Description: "Manage video file versions. Use 'merge_versions' to combine multiple items as alternate versions of the same title " +
				"(e.g. 4K and 1080p copies). Use 'split_versions' to undo a merge and restore items as separate entries. " +
				"Both actions require confirm=true. Get item IDs from jellyfin_search or jellyfin_browse.",
			Annotations: AnnotDestructive,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.VideosInput) (*mcp.CallToolResult, any, error) {
			switch args.Action {
			case "merge_versions":
				if len(args.ItemIDs) < 2 {
					return jf.ErrResult("item_ids must contain at least 2 item IDs to merge."), nil, nil
				}
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, fmt.Sprintf("This will MERGE %d items into alternate versions of a single item. The first item becomes the primary.", len(args.ItemIDs))); result != nil {
					return result, nil, nil
				}
				params := url.Values{"ids": {jf.JoinIDs(args.ItemIDs)}}
				if err := client.PostNoContent(ctx, "/Videos/MergeVersions", params, nil); err != nil {
					return jf.ErrResult("Failed to merge versions: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Merged %d items as alternate versions.", len(args.ItemIDs))), nil, nil

			case "split_versions":
				if args.ItemID == "" {
					return jf.ErrResultWithHint("Use jellyfin_search to find the merged item ID.", "item_id is required for split_versions."), nil, nil
				}
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, fmt.Sprintf("This will SPLIT all alternate versions from item '%s' back into separate items.", args.ItemID)); result != nil {
					return result, nil, nil
				}
				endpoint := fmt.Sprintf("/Videos/%s/AlternateSources", jf.SanitizeID(args.ItemID))
				if err := client.Del(ctx, endpoint, nil); err != nil {
					return jf.ErrResult("Failed to split versions: %v", err), nil, nil
				}
				return jf.TextResult("Alternate versions split into separate items."), nil, nil

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: merge_versions, split_versions", args.Action), nil, nil
			}
		})
	}

	// --- jellyfin_metadata ---
	if enabled("jellyfin_metadata", AnnotWriteOp) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_metadata",
			Title: "Metadata",
			InputSchema: jf.WithEnums[jf.MetadataInput](map[string][]any{
				"action": {"search", "apply", "update", "batch_update", "editor_info", "external_ids"},
			}),
			Description: "Search for and manage item metadata from online providers. Use 'search' to find metadata matches, then 'apply' to apply a result to an item. " +
				"Also supports 'update' for manual field edits, 'batch_update' for bulk changes, 'editor_info', and 'external_ids'.",
			Annotations: AnnotWriteOp,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.MetadataInput) (*mcp.CallToolResult, any, error) {
			switch args.Action {
			case "search":
				if args.SearchType == "" || args.SearchQuery == "" {
					return jf.ErrResult("search_type and search_query are required. search_type options: Movie, Series, Person, Book, BoxSet, MusicAlbum, MusicArtist"), nil, nil
				}
				body := map[string]any{
					"SearchInfo": map[string]any{
						"Name": args.SearchQuery,
					},
				}
				if args.SearchYear != nil {
					body["SearchInfo"].(map[string]any)["Year"] = *args.SearchYear
				}
				endpoint := fmt.Sprintf("/Items/RemoteSearch/%s", jf.SanitizeID(args.SearchType))
				var results []map[string]any
				if err := client.Post(ctx, endpoint, nil, body, &results); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := make([]map[string]any, 0, len(results))
				for _, r := range results {
					item := map[string]any{
						"name": jf.GetString(r, "Name"),
					}
					if year := jf.GetIntPtr(r, "ProductionYear"); year != nil {
						item["year"] = *year
					}
					if pids := jf.ToMap(r["ProviderIds"]); pids != nil {
						item["provider_ids"] = pids
					}
					if ov := jf.GetString(r, "Overview"); ov != "" {
						item["overview"] = jf.Truncate(ov, jf.OverviewMaxLen)
					}
					items = append(items, item)
				}
				return jf.TextResult(fmt.Sprintf("Found %d metadata results:\n\n%s", len(items), jf.FormatJSON(items))), nil, nil

			case "apply":
				if args.ItemID == "" || args.ProviderName == "" || args.ProviderID == "" {
					return jf.ErrResult("item_id, provider_name, and provider_id are required. Use 'search' first to find provider IDs."), nil, nil
				}
				body := map[string]any{
					"SearchResult": map[string]any{
						"ProviderIds": map[string]any{
							args.ProviderName: args.ProviderID,
						},
					},
				}
				endpoint := fmt.Sprintf("/Items/RemoteSearch/Apply/%s", jf.SanitizeID(args.ItemID))
				if err := client.PostNoContent(ctx, endpoint, nil, body); err != nil {
					return jf.ErrResult("Failed to apply metadata: %v", err), nil, nil
				}
				return jf.TextResult("Metadata applied. The item will be updated with data from the provider."), nil, nil

			case "update":
				if args.ItemID == "" {
					return jf.ErrResult("item_id is required for update."), nil, nil
				}

				jf.ReportProgress(ctx, req, 0, 2, "Fetching current item data...")

				userID, err := client.GetUserID(ctx)
				if err != nil {
					return jf.ErrResult("Jellyfin error: %v", err), nil, nil
				}
				var current map[string]any
				if err := client.Get(ctx, fmt.Sprintf("/Users/%s/Items/%s", jf.SanitizeID(userID), jf.SanitizeID(args.ItemID)), nil, &current); err != nil {
					return jf.ErrResult("Failed to get item: %v", err), nil, nil
				}

				jf.ReportProgress(ctx, req, 1, 2, "Applying updates...")

				jf.ApplyMetadataFields(current, args)
				endpoint := fmt.Sprintf("/Items/%s", jf.SanitizeID(args.ItemID))
				if err := client.PostNoContent(ctx, endpoint, nil, current); err != nil {
					return jf.ErrResult("Failed to update item: %v", err), nil, nil
				}
				return jf.TextResult("Item metadata updated."), nil, nil

			case "batch_update":
				if len(args.ItemIDs) == 0 {
					return jf.ErrResult("item_ids is required for batch_update (max 50)."), nil, nil
				}
				if len(args.ItemIDs) > 50 {
					return jf.ErrResult("batch_update supports at most 50 items at a time."), nil, nil
				}
				// Build field description for confirmation message
				var fields []string
				if args.Name != "" {
					fields = append(fields, "name")
				}
				if args.Overview != "" {
					fields = append(fields, "overview")
				}
				if len(args.Genres) > 0 {
					fields = append(fields, "genres")
				}
				if len(args.Tags) > 0 {
					fields = append(fields, "tags")
				}
				if len(args.Studios) > 0 {
					fields = append(fields, "studios")
				}
				if args.Year != nil {
					fields = append(fields, "year")
				}
				if args.CommunityRating != nil {
					fields = append(fields, "community_rating")
				}
				if args.OfficialRating != "" {
					fields = append(fields, "official_rating")
				}
				if args.SortName != "" {
					fields = append(fields, "sort_name")
				}
				if len(args.LockedFields) > 0 {
					fields = append(fields, "locked_fields")
				}
				if len(fields) == 0 {
					return jf.ErrResult("No fields to update. Provide at least one field (name, overview, genres, tags, studios, production_year, community_rating, official_rating, sort_name, locked_fields)."), nil, nil
				}

				// Dry-run mode: default true — preview changes without applying
				dryRun := args.DryRun == nil || *args.DryRun
				if dryRun {
					return jf.TextResult(fmt.Sprintf("DRY RUN — would update %d items. Fields: %s\n\nSet dry_run=false and confirm=true to apply.", len(args.ItemIDs), strings.Join(fields, ", "))), nil, nil
				}

				if result := jf.ConfirmationGate(ctx, req, args.Confirm, fmt.Sprintf("This will update metadata for %d items. Fields: %s", len(args.ItemIDs), strings.Join(fields, ", "))); result != nil {
					return result, nil, nil
				}

				userID, err := client.GetUserID(ctx)
				if err != nil {
					return jf.ErrResult("Jellyfin error: %v", err), nil, nil
				}
				total := float64(len(args.ItemIDs))
				successes := 0
				failures := 0
				for i, id := range args.ItemIDs {
					jf.ReportProgress(ctx, req, float64(i), total, fmt.Sprintf("Updating item %d/%d", i+1, int(total)))
					var current map[string]any
					if err := client.Get(ctx, fmt.Sprintf("/Users/%s/Items/%s", jf.SanitizeID(userID), jf.SanitizeID(id)), nil, &current); err != nil {
						failures++
						continue
					}
					jf.ApplyMetadataFields(current, args)
					if err := client.PostNoContent(ctx, fmt.Sprintf("/Items/%s", jf.SanitizeID(id)), nil, current); err != nil {
						failures++
						continue
					}
					successes++
				}
				jf.ReportProgress(ctx, req, total, total, "Batch update complete")
				return jf.TextResult(fmt.Sprintf("Batch update complete. Updated %d of %d items. %d failures.", successes, len(args.ItemIDs), failures)), nil, nil

			case "editor_info":
				if args.ItemID == "" {
					return jf.ErrResult("item_id is required."), nil, nil
				}
				var info map[string]any
				endpoint := fmt.Sprintf("/Items/%s/MetadataEditor", jf.SanitizeID(args.ItemID))
				if err := client.Get(ctx, endpoint, nil, &info); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				return jf.TextResult(jf.FormatJSON(info)), nil, nil

			case "external_ids":
				if args.ItemID == "" {
					return jf.ErrResult("item_id is required."), nil, nil
				}
				var ids []map[string]any
				endpoint := fmt.Sprintf("/Items/%s/ExternalIdInfos", jf.SanitizeID(args.ItemID))
				if err := client.Get(ctx, endpoint, nil, &ids); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				return jf.TextResult(jf.FormatJSON(ids)), nil, nil

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: search, apply, update, batch_update, editor_info, external_ids", args.Action), nil, nil
			}
		})
	}

	// --- jellyfin_subtitles_lyrics ---
	if enabled("jellyfin_subtitles_lyrics", AnnotWriteOp) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_subtitles_lyrics",
			Title: "Subtitles & Lyrics",
			InputSchema: jf.WithEnums[jf.SubtitlesLyricsInput](map[string][]any{
				"action": {"search_subtitles", "download_subtitle", "delete_subtitle", "batch_download_subtitles", "upload_subtitle", "get_lyrics", "search_lyrics", "download_lyrics", "delete_lyrics"},
			}),
			Description: "Search for, download, and manage subtitles and lyrics. For subtitles: use 'search_subtitles' with item_id and language to find available subtitles online, " +
				"'download_subtitle' with subtitle_id to download one, 'delete_subtitle' with subtitle_index to remove a track (confirm required). " +
				"Use 'batch_download_subtitles' with item_ids (max 25) to search and download subtitles for multiple items (confirm required). " +
				"For lyrics: use 'get_lyrics' to view current lyrics, 'search_lyrics' to find lyrics online, " +
				"'download_lyrics' with lyric_id to download, 'delete_lyrics' to remove (confirm required). Language codes use ISO 639 format (e.g. en, es, fr, de, ja).",
			Annotations: AnnotWriteOp,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.SubtitlesLyricsInput) (*mcp.CallToolResult, any, error) {
			itemID := jf.SanitizeID(args.ItemID)

			switch args.Action {
			case "search_subtitles":
				lang := args.Language
				if lang == "" {
					lang = "en"
				}
				endpoint := fmt.Sprintf("/Items/%s/RemoteSearch/Subtitles/%s", itemID, jf.SanitizeID(lang))
				var results []map[string]any
				if err := client.Get(ctx, endpoint, nil, &results); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := make([]map[string]any, 0, len(results))
				for _, r := range results {
					items = append(items, map[string]any{
						"id":             jf.GetString(r, "Id"),
						"name":           jf.GetString(r, "Name"),
						"provider":       jf.GetString(r, "ProviderName"),
						"format":         jf.GetString(r, "Format"),
						"language":       jf.GetString(r, "Language"),
						"download_count": jf.GetInt(r, "DownloadCount"),
					})
				}
				return jf.TextResult(fmt.Sprintf("Found %d subtitles:\n\n%s", len(items), jf.FormatJSON(items))), nil, nil

			case "download_subtitle":
				if args.SubtitleID == "" {
					return jf.ErrResult("subtitle_id is required. Use 'search_subtitles' to find subtitle IDs."), nil, nil
				}
				endpoint := fmt.Sprintf("/Items/%s/RemoteSearch/Subtitles/%s", itemID, jf.SanitizeID(args.SubtitleID))
				if err := client.PostNoContent(ctx, endpoint, nil, nil); err != nil {
					return jf.ErrResult("Failed to download subtitle: %v", err), nil, nil
				}
				return jf.TextResult("Subtitle downloaded and added to item."), nil, nil

			case "delete_subtitle":
				if args.SubtitleIndex == nil {
					return jf.ErrResult("subtitle_index is required. Get the index from jellyfin_get_item media source info."), nil, nil
				}
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, fmt.Sprintf("This will DELETE subtitle track %d from item '%s'.", *args.SubtitleIndex, args.ItemID)); result != nil {
					return result, nil, nil
				}
				endpoint := fmt.Sprintf("/Videos/%s/Subtitles/%d", itemID, *args.SubtitleIndex)
				if err := client.Del(ctx, endpoint, nil); err != nil {
					return jf.ErrResult("Failed to delete subtitle: %v", err), nil, nil
				}
				return jf.TextResult("Subtitle track deleted."), nil, nil

			case "batch_download_subtitles":
				if len(args.ItemIDs) == 0 {
					return jf.ErrResult("item_ids is required for batch_download_subtitles (max 25)."), nil, nil
				}
				if len(args.ItemIDs) > 25 {
					return jf.ErrResult("batch_download_subtitles supports at most 25 items at a time."), nil, nil
				}
				lang := args.Language
				if lang == "" {
					lang = "en"
				}
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, fmt.Sprintf("This will search and download subtitles for %d items in language '%s'.", len(args.ItemIDs), lang)); result != nil {
					return result, nil, nil
				}
				total := float64(len(args.ItemIDs))
				var results []map[string]any
				for i, id := range args.ItemIDs {
					jf.ReportProgress(ctx, req, float64(i), total, fmt.Sprintf("Processing item %d/%d", i+1, int(total)))
					escapedID := jf.SanitizeID(id)
					endpoint := fmt.Sprintf("/Items/%s/RemoteSearch/Subtitles/%s", escapedID, jf.SanitizeID(lang))
					var searchResults []map[string]any
					if err := client.Get(ctx, endpoint, nil, &searchResults); err != nil {
						results = append(results, map[string]any{"item_id": id, "status": "search_failed", "error": err.Error()})
						continue
					}
					if len(searchResults) == 0 {
						results = append(results, map[string]any{"item_id": id, "status": "no_results"})
						continue
					}
					// Pick best result (first)
					best := searchResults[0]
					subID := jf.GetString(best, "Id")
					dlEndpoint := fmt.Sprintf("/Items/%s/RemoteSearch/Subtitles/%s", escapedID, jf.SanitizeID(subID))
					if err := client.PostNoContent(ctx, dlEndpoint, nil, nil); err != nil {
						results = append(results, map[string]any{"item_id": id, "status": "download_failed", "subtitle": jf.GetString(best, "Name"), "error": err.Error()})
						continue
					}
					results = append(results, map[string]any{"item_id": id, "status": "downloaded", "subtitle": jf.GetString(best, "Name"), "provider": jf.GetString(best, "ProviderName")})
				}
				jf.ReportProgress(ctx, req, total, total, "Batch subtitle download complete")
				downloaded := 0
				for _, r := range results {
					if jf.GetString(r, "status") == "downloaded" {
						downloaded++
					}
				}
				return jf.TextResult(fmt.Sprintf("Batch subtitle download complete. Downloaded %d of %d:\n\n%s", downloaded, len(args.ItemIDs), jf.FormatJSON(results))), nil, nil

			case "get_lyrics":
				endpoint := fmt.Sprintf("/Audio/%s/Lyrics", itemID)
				var result map[string]any
				if err := client.Get(ctx, endpoint, nil, &result); err != nil {
					return jf.ErrResult("Jellyfin API error: %v. This item may not have lyrics.", err), nil, nil
				}
				return jf.TextResult(jf.FormatJSON(result)), nil, nil

			case "search_lyrics":
				endpoint := fmt.Sprintf("/Audio/%s/RemoteSearch/Lyrics", itemID)
				var results []map[string]any
				if err := client.Get(ctx, endpoint, nil, &results); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Found %d lyric results:\n\n%s", len(results), jf.FormatJSON(results))), nil, nil

			case "download_lyrics":
				if args.LyricID == "" {
					return jf.ErrResult("lyric_id is required. Use 'search_lyrics' to find lyric IDs."), nil, nil
				}
				endpoint := fmt.Sprintf("/Audio/%s/RemoteSearch/Lyrics/%s", itemID, jf.SanitizeID(args.LyricID))
				if err := client.PostNoContent(ctx, endpoint, nil, nil); err != nil {
					return jf.ErrResult("Failed to download lyrics: %v", err), nil, nil
				}
				return jf.TextResult("Lyrics downloaded and added to item."), nil, nil

			case "upload_subtitle":
				if args.ItemID == "" {
					return jf.ErrResult("item_id is required for upload_subtitle."), nil, nil
				}
				if args.SubtitleData == "" || args.SubtitleFormat == "" {
					return jf.ErrResult("subtitle_data (base64-encoded) and subtitle_format (e.g. srt, ass, vtt) are required for upload_subtitle."), nil, nil
				}
				lang := args.SubtitleLanguage
				if lang == "" {
					lang = "eng"
				}
				body := map[string]any{
					"Language": lang,
					"Format":   args.SubtitleFormat,
					"Data":     args.SubtitleData,
				}
				if args.IsForced != nil {
					body["IsForced"] = *args.IsForced
				}
				if args.IsHearingImpaired != nil {
					body["IsHearingImpaired"] = *args.IsHearingImpaired
				}
				endpoint := fmt.Sprintf("/Videos/%s/Subtitles", itemID)
				if err := client.PostNoContent(ctx, endpoint, nil, body); err != nil {
					return jf.ErrResult("Failed to upload subtitle: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Subtitle uploaded (%s, %s).", args.SubtitleFormat, lang)), nil, nil

			case "delete_lyrics":
				if result := jf.ConfirmationGate(ctx, req, args.Confirm, fmt.Sprintf("This will DELETE lyrics from item '%s'.", args.ItemID)); result != nil {
					return result, nil, nil
				}
				endpoint := fmt.Sprintf("/Audio/%s/Lyrics", itemID)
				if err := client.Del(ctx, endpoint, nil); err != nil {
					return jf.ErrResult("Failed to delete lyrics: %v", err), nil, nil
				}
				return jf.TextResult("Lyrics deleted."), nil, nil

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: search_subtitles, download_subtitle, delete_subtitle, batch_download_subtitles, upload_subtitle, get_lyrics, search_lyrics, download_lyrics, delete_lyrics", args.Action), nil, nil
			}
		})
	}

	// --- jellyfin_images ---
	if enabled("jellyfin_images", AnnotWriteOp) {
		mcp.AddTool(server, &mcp.Tool{
			Name:  "jellyfin_images",
			Title: "Images",
			InputSchema: jf.WithEnums[jf.ImagesInput](map[string][]any{
				"action": {"list", "get_url", "remote_list", "remote_download", "upload"},
			}),
			Description: "Manage item images. Use 'list' to see all images for an item (types: Primary, Backdrop, Logo, Thumb, Banner). " +
				"Use 'get_url' to get the direct URL for an item's image (useful for displaying or linking). " +
				"Use 'remote_list' to browse available images from online providers like TheMovieDb. " +
				"Use 'remote_download' to download a specific remote image URL and set it for the item. " +
				"Use 'upload' to set an image from base64-encoded data (JPEG or PNG).",
			Annotations: AnnotWriteOp,
		}, func(ctx context.Context, req *mcp.CallToolRequest, args jf.ImagesInput) (*mcp.CallToolResult, any, error) {
			itemID := jf.SanitizeID(args.ItemID)
			imageType := args.ImageType
			if imageType == "" {
				imageType = "Primary"
			}

			switch args.Action {
			case "list":
				var images []map[string]any
				endpoint := fmt.Sprintf("/Items/%s/Images", itemID)
				if err := client.Get(ctx, endpoint, nil, &images); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				items := make([]map[string]any, 0, len(images))
				for _, img := range images {
					items = append(items, map[string]any{
						"image_type":  jf.GetString(img, "ImageType"),
						"image_index": jf.GetInt(img, "ImageIndex"),
						"width":       jf.GetInt(img, "Width"),
						"height":      jf.GetInt(img, "Height"),
					})
				}
				return jf.TextResult(fmt.Sprintf("Images (%d):\n\n%s", len(items), jf.FormatJSON(items))), nil, nil

			case "get_url":
				idx := 0
				if args.ImageIndex != nil {
					idx = *args.ImageIndex
				}
				imageURL := fmt.Sprintf("%s/Items/%s/Images/%s/%d", client.BaseURL(), args.ItemID, imageType, idx)
				return jf.TextResult(fmt.Sprintf("Image URL: %s", imageURL)), nil, nil

			case "remote_list":
				params := url.Values{}
				if args.Provider != "" {
					params.Set("ProviderName", args.Provider)
				}
				params.Set("Type", imageType)
				endpoint := fmt.Sprintf("/Items/%s/RemoteImages", itemID)
				var result map[string]any
				if err := client.Get(ctx, endpoint, params, &result); err != nil {
					return jf.ErrResult("Jellyfin API error: %v", err), nil, nil
				}
				rawImages := jf.ToSlice(result["Images"])
				items := make([]map[string]any, 0, len(rawImages))
				for _, raw := range rawImages {
					m := jf.ToMap(raw)
					items = append(items, map[string]any{
						"url":              jf.GetString(m, "Url"),
						"provider_name":    jf.GetString(m, "ProviderName"),
						"type":             jf.GetString(m, "Type"),
						"width":            jf.GetInt(m, "Width"),
						"height":           jf.GetInt(m, "Height"),
						"community_rating": jf.GetFloat(m, "CommunityRating"),
						"language":         jf.GetString(m, "Language"),
					})
				}
				return jf.TextResult(fmt.Sprintf("Remote images (%d):\n\n%s", len(items), jf.FormatJSON(items))), nil, nil

			case "remote_download":
				if args.ImageURL == "" {
					return jf.ErrResult("image_url is required. Use 'remote_list' to find image URLs."), nil, nil
				}
				body := map[string]any{
					"ImageUrl": args.ImageURL,
					"Type":     imageType,
				}
				endpoint := fmt.Sprintf("/Items/%s/RemoteImages/Download", itemID)
				if err := client.PostNoContent(ctx, endpoint, nil, body); err != nil {
					return jf.ErrResult("Failed to download image: %v", err), nil, nil
				}
				return jf.TextResult("Image downloaded and set for item."), nil, nil

			case "upload":
				if args.ImageData == "" {
					return jf.ErrResult("image_data (base64-encoded JPEG or PNG) is required for upload."), nil, nil
				}
				decoded, err := base64.StdEncoding.DecodeString(args.ImageData)
				if err != nil {
					return jf.ErrResult("Invalid base64 in image_data: %v", err), nil, nil
				}
				// Detect content type from magic bytes
				contentType := "image/png"
				if len(decoded) >= 2 && decoded[0] == 0xFF && decoded[1] == 0xD8 {
					contentType = "image/jpeg"
				}
				endpoint := fmt.Sprintf("/Items/%s/Images/%s", itemID, imageType)
				if err := client.PostRaw(ctx, endpoint, nil, decoded, contentType); err != nil {
					return jf.ErrResult("Failed to upload image: %v", err), nil, nil
				}
				return jf.TextResult(fmt.Sprintf("Image uploaded as %s for item.", imageType)), nil, nil

			default:
				return jf.ErrResult("Invalid action '%s'. Valid actions: list, get_url, remote_list, remote_download, upload", args.Action), nil, nil
			}
		})
	}
}
