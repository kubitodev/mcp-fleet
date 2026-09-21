# Resources

13 MCP resources providing live data from the Jellyfin server.

## Fixed resources

| URI | Description |
|-----|-------------|
| `jellyfin://server/info` | Server name, version, OS, and network details |
| `jellyfin://libraries` | All media libraries with IDs, collection types, and paths |
| `jellyfin://sessions` | All connected client sessions with playback status |
| `jellyfin://sessions/now-playing` | Sessions with active playback only |
| `jellyfin://users` | All user accounts with admin status and last activity |
| `jellyfin://resume` | Items with in-progress playback |
| `jellyfin://next-up` | Next episodes in series that are in progress |
| `jellyfin://favorites` | Items marked as favorite by the current user |
| `jellyfin://latest` | Recently added items across all libraries |
| `jellyfin://recently-played` | Recently watched or listened items (verified playback only) |

## Template resources

| URI | Description |
|-----|-------------|
| `jellyfin://items/{itemId}` | Detailed metadata for any media item |
| `jellyfin://users/{userId}` | User account details and policy settings |
| `jellyfin://libraries/{libraryId}/latest` | Recently added items in a specific library |

## Reference guides

| URI | Description |
|-----|-------------|
| `jellyfin://guides/transcoding` | Hardware transcoding: Intel QSV, VAAPI, NVENC, Docker GPU passthrough |
| `jellyfin://guides/file-naming` | Movie, TV, and music file naming for proper metadata matching |
| `jellyfin://guides/remote-access` | Reverse proxy, VPN, and port forwarding setup |
| `jellyfin://guides/troubleshooting` | Common issues and solutions for scans, metadata, playback, and database |
| `jellyfin://guides/library-setup` | Library organization, content separation, storage, and Docker volumes |
| `jellyfin://guides/docker` | Docker volume mounts, GPU passthrough, permissions, compose files |
| `jellyfin://guides/users-and-access` | User management, parental controls, per-library access |
| `jellyfin://guides/plugins` | Plugin repositories, recommended plugins, and configuration |
| `jellyfin://guides/migration` | Migrating from Plex or Emby: library setup, metadata, watch history |
| `jellyfin://guides/performance` | Transcoding optimization, database maintenance, cache, and networking |
