package jellyfin

import "fmt"

func ExtractMediaItem(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	item := map[string]any{
		"id":   GetString(m, "Id"),
		"name": GetString(m, "Name"),
		"type": GetString(m, "Type"),
	}
	if year := GetIntPtr(m, "ProductionYear"); year != nil {
		item["year"] = *year
	}
	if overview := GetString(m, "Overview"); overview != "" {
		item["overview"] = Truncate(overview, OverviewMaxLen)
	}
	if rating := GetFloat(m, "CommunityRating"); rating > 0 {
		item["community_rating"] = rating
	}
	if official := GetString(m, "OfficialRating"); official != "" {
		item["official_rating"] = official
	}
	if rt := GetInt64(m, "RunTimeTicks"); rt > 0 {
		item["runtime_minutes"] = rt / TicksPerMinute
	}
	// User data (played/favorite/progress)
	if ud := ToMap(m["UserData"]); ud != nil {
		if GetBool(ud, "Played") {
			item["played"] = true
		}
		if pct := GetFloat(ud, "PlayedPercentage"); pct > 0 {
			item["progress"] = fmt.Sprintf("%.0f%%", pct)
		}
		if GetBool(ud, "IsFavorite") {
			item["favorite"] = true
		}
	}
	// Series context for episodes
	if sn := GetString(m, "SeriesName"); sn != "" {
		item["series_name"] = sn
	}
	if idx := GetInt(m, "IndexNumber"); idx > 0 {
		item["index_number"] = idx
	}
	if pidx := GetInt(m, "ParentIndexNumber"); pidx > 0 {
		item["parent_index_number"] = pidx
	}
	return item
}

func ExtractDetailedItem(m map[string]any) map[string]any {
	item := ExtractMediaItem(m)

	// Full overview (not truncated)
	if overview := GetString(m, "Overview"); overview != "" {
		item["overview"] = overview
	}

	// Dates
	if pd := GetString(m, "PremiereDate"); pd != "" {
		item["premiere_date"] = Truncate(pd, DateOnlyLen)
	}
	if ed := GetString(m, "EndDate"); ed != "" {
		item["end_date"] = Truncate(ed, DateOnlyLen)
	}

	// Ratings
	if cr := GetFloat(m, "CriticRating"); cr > 0 {
		item["critic_rating"] = cr
	}

	if tl := ToStringSlice(m["Taglines"]); len(tl) > 0 {
		item["taglines"] = tl
	}
	if gl := ToStringSlice(m["Genres"]); len(gl) > 0 {
		item["genres"] = gl
	}

	// Studios
	if studios := ToSlice(m["Studios"]); len(studios) > 0 {
		sl := make([]string, 0, len(studios))
		for _, s := range studios {
			sm := ToMap(s)
			if sm != nil {
				sl = append(sl, GetString(sm, "Name"))
			}
		}
		if len(sl) > 0 {
			item["studios"] = sl
		}
	}

	// People (top 15)
	if people := ToSlice(m["People"]); len(people) > 0 {
		pl := make([]map[string]any, 0, MaxPeopleInDetail)
		for i, p := range people {
			if i >= MaxPeopleInDetail {
				break
			}
			pm := ToMap(p)
			if pm == nil {
				continue
			}
			person := map[string]any{
				"name": GetString(pm, "Name"),
				"type": GetString(pm, "Type"),
			}
			if role := GetString(pm, "Role"); role != "" {
				person["role"] = role
			}
			pl = append(pl, person)
		}
		if len(pl) > 0 {
			item["people"] = pl
		}
	}

	// Provider IDs (IMDb, TMDB, TVDB) with direct URLs
	if pids := ToMap(m["ProviderIds"]); pids != nil {
		providerIDs := make(map[string]string)
		for k, v := range pids {
			if s, ok := v.(string); ok && s != "" {
				providerIDs[k] = s
			}
		}
		if len(providerIDs) > 0 {
			item["provider_ids"] = providerIDs
		}
		// Build clickable URLs from known providers
		links := BuildProviderLinks(providerIDs, GetString(m, "Type"))
		if len(links) > 0 {
			item["external_urls"] = links
		}
	}

	// User data
	if ud := ToMap(m["UserData"]); ud != nil {
		userData := map[string]any{
			"played":   GetBool(ud, "Played"),
			"favorite": GetBool(ud, "IsFavorite"),
		}
		if pc := GetInt(ud, "PlayCount"); pc > 0 {
			userData["play_count"] = pc
		}
		if pct := GetFloat(ud, "PlayedPercentage"); pct > 0 {
			userData["played_percentage"] = pct
		}
		if lp := GetString(ud, "LastPlayedDate"); lp != "" {
			userData["last_played"] = Truncate(lp, DateOnlyLen)
		}
		item["user_data"] = userData
	}

	// File path
	if path := GetString(m, "Path"); path != "" {
		item["file_path"] = path
	}
	if fn := GetString(m, "FileName"); fn != "" {
		item["file_name"] = fn
	}

	// Media sources (codec info, file details, all streams)
	if sources := ToSlice(m["MediaSources"]); len(sources) > 0 {
		ms := make([]map[string]any, 0, len(sources))
		for _, src := range sources {
			sm := ToMap(src)
			if sm == nil {
				continue
			}
			source := map[string]any{
				"container": GetString(sm, "Container"),
			}
			if path := GetString(sm, "Path"); path != "" {
				source["path"] = path
			}
			if br := GetInt64(sm, "Bitrate"); br > 0 {
				source["bitrate_kbps"] = br / UnitsPerKilo
			}
			if sz := GetInt64(sm, "Size"); sz > 0 {
				source["size_mb"] = sz / BytesPerMB
			}

			// Media streams — capture all audio and subtitle tracks
			var audioStreams []map[string]any
			var subtitleStreams []map[string]any
			if streams := ToSlice(sm["MediaStreams"]); len(streams) > 0 {
				for _, st := range streams {
					stm := ToMap(st)
					if stm == nil {
						continue
					}
					switch GetString(stm, "Type") {
					case "Video":
						source["video_codec"] = GetString(stm, "Codec")
						if w := GetInt(stm, "Width"); w > 0 {
							source["resolution"] = fmt.Sprintf("%dx%d", w, GetInt(stm, "Height"))
						}
						if profile := GetString(stm, "Profile"); profile != "" {
							source["video_profile"] = profile
						}
						if bitDepth := GetInt(stm, "BitDepth"); bitDepth > 0 {
							source["video_bit_depth"] = bitDepth
						}
						if videoRange := GetString(stm, "VideoRange"); videoRange != "" {
							source["video_range"] = videoRange
						}
					case "Audio":
						audio := map[string]any{
							"index": GetInt(stm, "Index"),
							"codec": GetString(stm, "Codec"),
						}
						if ch := GetInt(stm, "Channels"); ch > 0 {
							audio["channels"] = ch
						}
						if lang := GetString(stm, "Language"); lang != "" {
							audio["language"] = lang
						}
						if title := GetString(stm, "DisplayTitle"); title != "" {
							audio["display_title"] = title
						}
						if GetBool(stm, "IsDefault") {
							audio["is_default"] = true
						}
						audioStreams = append(audioStreams, audio)
					case "Subtitle":
						sub := map[string]any{
							"index": GetInt(stm, "Index"),
							"codec": GetString(stm, "Codec"),
						}
						if lang := GetString(stm, "Language"); lang != "" {
							sub["language"] = lang
						}
						if title := GetString(stm, "DisplayTitle"); title != "" {
							sub["display_title"] = title
						}
						if GetBool(stm, "IsExternal") {
							sub["is_external"] = true
						}
						if GetBool(stm, "IsDefault") {
							sub["is_default"] = true
						}
						if GetBool(stm, "IsForced") {
							sub["is_forced"] = true
						}
						subtitleStreams = append(subtitleStreams, sub)
					}
				}
			}
			if len(audioStreams) > 0 {
				source["audio_streams"] = audioStreams
			}
			if len(subtitleStreams) > 0 {
				source["subtitle_streams"] = subtitleStreams
			}
			ms = append(ms, source)
		}
		if len(ms) > 0 {
			item["media_sources"] = ms
		}
	}

	if al := ToStringSlice(m["Artists"]); len(al) > 0 {
		item["artists"] = al
	}
	if album := GetString(m, "Album"); album != "" {
		item["album"] = album
	}

	// TV-specific (series_name already set by ExtractMediaItem)
	if status := GetString(m, "Status"); status != "" {
		item["status"] = status
	}

	// Subtitle/lyric availability
	if GetBool(m, "HasSubtitles") {
		item["has_subtitles"] = true
	}
	if GetBool(m, "HasLyrics") {
		item["has_lyrics"] = true
	}

	// Child count for container items
	if cc := GetInt(m, "ChildCount"); cc > 0 {
		item["child_count"] = cc
	}
	if ric := GetInt(m, "RecursiveItemCount"); ric > 0 {
		item["recursive_item_count"] = ric
	}

	return item
}

func ExtractSessionInfo(s map[string]any) map[string]any {
	session := map[string]any{
		"session_id":    GetString(s, "Id"),
		"user":          GetString(s, "UserName"),
		"client":        GetString(s, "Client"),
		"device_name":   GetString(s, "DeviceName"),
		"last_activity": Truncate(GetString(s, "LastActivityDate"), DateTimeLen),
	}

	np := ToMap(s["NowPlayingItem"])
	if np != nil {
		nowPlaying := map[string]any{
			"id":   GetString(np, "Id"),
			"name": GetString(np, "Name"),
			"type": GetString(np, "Type"),
		}
		if rt := GetInt64(np, "RunTimeTicks"); rt > 0 {
			nowPlaying["runtime_minutes"] = rt / 600000000
		}
		session["now_playing"] = nowPlaying

		ps := ToMap(s["PlayState"])
		if ps != nil {
			playState := map[string]any{
				"is_paused": GetBool(ps, "IsPaused"),
			}
			if pos := GetInt64(ps, "PositionTicks"); pos > 0 {
				playState["position_seconds"] = pos / TicksPerSecond
				playState["position_ticks"] = pos
			}
			if vol := GetInt(ps, "VolumeLevel"); vol > 0 {
				playState["volume"] = vol
			}
			session["play_state"] = playState
		}
	}

	return session
}

func ExtractLibraries(libs []map[string]any) []map[string]any {
	items := make([]map[string]any, 0, len(libs))
	for _, lib := range libs {
		ct := GetString(lib, "CollectionType")
		if ct == "" {
			ct = "mixed"
		}
		item := map[string]any{
			"name":            GetString(lib, "Name"),
			"collection_type": ct,
			"item_id":         GetString(lib, "ItemId"),
		}
		if paths := ToStringSlice(lib["Locations"]); len(paths) > 0 {
			item["paths"] = paths
		}
		items = append(items, item)
	}
	return items
}

func ExtractUserSummary(u map[string]any) map[string]any {
	user := map[string]any{
		"id":       GetString(u, "Id"),
		"name":     GetString(u, "Name"),
		"is_admin": false,
	}
	if policy := ToMap(u["Policy"]); policy != nil {
		user["is_admin"] = GetBool(policy, "IsAdministrator")
		if GetBool(policy, "IsDisabled") {
			user["is_disabled"] = true
		}
	}
	if la := GetString(u, "LastActivityDate"); la != "" {
		user["last_activity"] = Truncate(la, DateTimeLen)
	}
	if ll := GetString(u, "LastLoginDate"); ll != "" {
		user["last_login"] = Truncate(ll, DateTimeLen)
	}
	return user
}

func ExtractDetailedUser(u map[string]any) map[string]any {
	user := ExtractUserSummary(u)

	if GetBool(u, "HasPassword") {
		user["has_password"] = true
	}

	if policy := ToMap(u["Policy"]); policy != nil {
		// Library access
		if GetBool(policy, "EnableAllFolders") {
			user["enable_all_folders"] = true
		} else {
			user["enable_all_folders"] = false
			if ids := ToStringSlice(policy["EnabledFolders"]); len(ids) > 0 {
				user["enabled_folders"] = ids
			}
		}
		// Parental controls (only when set)
		if mr := GetIntPtr(policy, "MaxParentalRating"); mr != nil {
			user["max_parental_rating"] = *mr
		}
		if tl := ToStringSlice(policy["BlockedTags"]); len(tl) > 0 {
			user["blocked_tags"] = tl
		}
		// Notable non-default permissions
		if GetBool(policy, "IsHidden") {
			user["is_hidden"] = true
		}
		if !GetBool(policy, "EnableRemoteAccess") {
			user["remote_access"] = false
		}
		if GetBool(policy, "EnableContentDeletion") {
			user["can_delete_content"] = true
		}
		// Limits (only when non-zero)
		if ms := GetInt(policy, "MaxActiveSessions"); ms > 0 {
			user["max_active_sessions"] = ms
		}
		if bl := GetInt64(policy, "RemoteClientBitrateLimit"); bl > 0 {
			user["remote_bitrate_limit"] = bl
		}
		if la := GetInt(policy, "InvalidLoginAttemptCount"); la > 0 {
			user["invalid_login_attempt_count"] = la
		}
	}

	if config := ToMap(u["Configuration"]); config != nil {
		prefs := make(map[string]any)
		if sl := GetString(config, "SubtitleLanguagePreference"); sl != "" {
			prefs["subtitle_language"] = sl
		}
		if al := GetString(config, "AudioLanguagePreference"); al != "" {
			prefs["audio_language"] = al
		}
		if sm := GetString(config, "SubtitleMode"); sm != "" {
			prefs["subtitle_mode"] = sm
		}
		if !GetBool(config, "PlayDefaultAudioTrack") {
			prefs["play_default_audio"] = false
		}
		if !GetBool(config, "EnableNextEpisodeAutoPlay") {
			prefs["next_episode_auto_play"] = false
		}
		if GetBool(config, "DisplayMissingEpisodes") {
			prefs["display_missing_episodes"] = true
		}
		if len(prefs) > 0 {
			user["preferences"] = prefs
		}
	}

	return user
}

func ExtractItemList(result map[string]any) []map[string]any {
	return MapExtract(ToSlice(result["Items"]), ExtractMediaItem)
}
