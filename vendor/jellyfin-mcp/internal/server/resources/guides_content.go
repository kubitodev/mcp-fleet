package resources

const transcodingGuide = `# Hardware Transcoding Setup

## Choosing a Hardware Acceleration Method

| Method | Best For | GPU Required |
|--------|----------|-------------|
| Intel QSV | Most users, excellent quality/performance | Intel CPU with iGPU (6th gen+) |
| VAAPI | Linux with Intel/AMD GPUs | Intel or AMD GPU |
| NVENC | NVIDIA GPU owners | NVIDIA GTX 1050+ |
| AMF | AMD GPU on Windows | AMD Radeon RX 400+ |

## Intel QSV (Recommended for most setups)

QSV is available on Intel CPUs with integrated graphics (HD Graphics 500+):

- **6th-7th Gen (Skylake/Kaby Lake):** H.264 encode/decode, partial HEVC
- **8th Gen+ (Coffee Lake+):** Full HEVC 10-bit, HDR tone mapping
- **11th Gen+ (Rocket Lake+):** AV1 decode
- **Arc GPUs / 12th Gen+:** AV1 encode and decode

### Docker GPU Passthrough (Intel)

~~~yaml
devices:
  - /dev/dri/renderD128:/dev/dri/renderD128
~~~

## NVIDIA NVENC

### Docker GPU Passthrough (NVIDIA)

~~~yaml
runtime: nvidia
deploy:
  resources:
    reservations:
      devices:
        - capabilities: [gpu]
~~~

Or use ` + "`--gpus all`" + ` with ` + "`docker run`" + `.

Requires the NVIDIA Container Toolkit installed on the host.

## Recommended Settings

- **Bitrate limit for remote streaming:** 8-20 Mbps for 1080p, 40-80 Mbps for 4K
- **Throttle transcodes:** Enable to reduce CPU usage when client buffer is full
- **Hardware decoding:** Enable all codecs your GPU supports
- **Tonemapping:** Enable for HDR-to-SDR conversion (requires 8th gen Intel+ or NVIDIA 1000+)

## Common Pitfalls

- Wrong hardware acceleration selected for your GPU
- Missing GPU drivers or firmware on the host OS
- Docker container not passed the GPU device
- Transcoding cache on slow storage (use SSD for /cache)
- VAAPI requires the ` + "`render`" + ` group — add the container user to it

## Sources

- https://jellyfin.org/docs/general/post-install/transcoding/
- https://jellyfin.org/docs/general/post-install/transcoding/hardware-acceleration/`

const fileNamingGuide = `# File Naming Conventions

Proper file naming is critical for Jellyfin to match your media with metadata providers.

## Movies

~~~
Movies/
  Movie Name (Year)/
    Movie Name (Year).mkv
~~~

Examples:
- ` + "`Movies/Neon Cascade (1999)/Neon Cascade (1999).mkv`" + `
- ` + "`Movies/Lucid Horizon (2010)/Lucid Horizon (2010).mp4`" + `

### Extras and Specials

~~~
Movies/
  Movie Name (Year)/
    Movie Name (Year).mkv
    behind the scenes/
      Behind The Scenes.mkv
    featurettes/
      Making Of.mkv
~~~

## TV Shows

~~~
TV Shows/
  Series Name (Year)/
    Season 01/
      Series Name S01E01.mkv
      Series Name S01E02.mkv
    Season 02/
      Series Name S02E01.mkv
~~~

### Multi-Episode Files

~~~
Series Name S01E01-E02.mkv
Series Name S01E01E02.mkv
~~~

### Specials

Place in ` + "`Season 00`" + `:

~~~
Series Name (Year)/
  Season 00/
    Series Name S00E01.mkv
~~~

## Music

~~~
Music/
  Artist Name/
    Album Name (Year)/
      01 - Track Name.flac
      02 - Track Name.flac
~~~

## Anime

For anime, consider using the **Shokofin** plugin with AniDB numbering. Without it, use standard TV naming with absolute episode numbers or TVDB numbering.

## Common Mistakes

- **Missing year:** ` + "`Movie Name.mkv`" + ` instead of ` + "`Movie Name (2024).mkv`" + ` — causes mismatches
- **Wrong delimiters:** Use spaces, not dots or underscores, in folder names
- **Mixed content:** Never put movies and TV shows in the same library folder
- **No folder per item:** Always wrap each movie in its own folder for extras and metadata images

## Sources

- https://jellyfin.org/docs/general/server/media/movies/
- https://jellyfin.org/docs/general/server/media/shows/
- https://jellyfin.org/docs/general/server/media/music/`

const remoteAccessGuide = `# Remote Access Setup

## Option 1: Reverse Proxy (Recommended)

A reverse proxy sits in front of Jellyfin and handles SSL/TLS termination.

### Caddy (Simplest)

~~~
jellyfin.example.com {
    reverse_proxy localhost:8096
}
~~~

Caddy automatically provisions HTTPS certificates via Let's Encrypt.

### Nginx

~~~nginx
server {
    listen 443 ssl http2;
    server_name jellyfin.example.com;

    ssl_certificate /etc/letsencrypt/live/jellyfin.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/jellyfin.example.com/privkey.pem;

    location / {
        proxy_pass http://localhost:8096;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
~~~

### Apache

~~~apache
<VirtualHost *:443>
    ServerName jellyfin.example.com
    SSLEngine on

    ProxyPreserveHost On
    ProxyPass / http://localhost:8096/
    ProxyPassReverse / http://localhost:8096/

    # WebSocket
    RewriteEngine on
    RewriteCond %{HTTP:Upgrade} websocket [NC]
    RewriteRule /(.*) ws://localhost:8096/$1 [P,L]
</VirtualHost>
~~~

## Option 2: VPN / Tailscale (Most Secure)

- **Tailscale:** Zero-config mesh VPN. Install on server and clients. Access via Tailscale IP. No port forwarding needed.
- **WireGuard:** Lightweight VPN. Requires manual key exchange.

This is the most secure option as Jellyfin is never exposed to the public internet.

## Option 3: Port Forwarding (Least Recommended)

Forward port 8096 (HTTP) or 8920 (HTTPS) on your router to the Jellyfin server.

**Security concerns:**
- Exposes Jellyfin directly to the internet
- No SSL unless configured in Jellyfin itself
- Brute-force attacks possible without rate limiting

## Base URL Configuration

If Jellyfin runs behind a subpath (e.g., ` + "`https://example.com/jellyfin`" + `):

1. Set Base URL to ` + "`/jellyfin`" + ` in Dashboard > Networking
2. Adjust your reverse proxy accordingly

**When to use Base URL:** Only when sharing a domain with other services on different paths. Not needed for a dedicated subdomain.

## SSL/TLS Certificates

- **Let's Encrypt (free):** Use Certbot or Caddy for automatic provisioning
- **Self-signed:** Works but causes client warnings
- Jellyfin can serve HTTPS directly (port 8920) but a reverse proxy is preferred

## Sources

- https://jellyfin.org/docs/general/post-install/networking/
- https://jellyfin.org/docs/general/post-install/networking/reverse-proxy/`

const troubleshootingGuide = `# Troubleshooting Common Issues

## Library Scan Failures

**Symptoms:** Library shows 0 items, scan never completes, items missing after scan.

**Solutions:**
1. Check file naming conventions (see jellyfin://guides/file-naming)
2. Verify file permissions — Jellyfin user must have read access
3. Check Dashboard > Scheduled Tasks for failed scan tasks
4. Review logs: ` + "`jellyfin_system_info action=log_file`" + `
5. Try scanning a single folder: ` + "`jellyfin_library_manage action=refresh_item`" + `

## Metadata Mismatches

**Symptoms:** Wrong poster, wrong description, movie matched to wrong title.

**Solutions:**
1. **Identify tool:** Use ` + "`jellyfin_metadata action=search`" + ` to find the correct match, then ` + "`action=apply`" + `
2. **NFO files:** Place a .nfo file next to the media file with the correct metadata provider ID
3. **Provider priority:** In Dashboard > Libraries > edit library, adjust metadata provider order
4. **Lock fields:** After correcting metadata, use ` + "`jellyfin_metadata action=update locked_fields=[\"Name\",\"Overview\"]`" + ` to prevent auto-overwrite

## Playback Issues

### Web Client
- Check browser console for errors (F12)
- Try disabling hardware acceleration in browser settings
- Clear browser cache

### Mobile/TV Apps
- Force close and reopen the app
- Check transcoding settings — lower bitrate limit for mobile
- Verify the server URL is correct and accessible

### Transcoding Failures
- Check ` + "`jellyfin_system_info action=log_file`" + ` for ffmpeg errors
- Verify GPU passthrough (see jellyfin://guides/transcoding)
- Test with a direct-play compatible client first
- Ensure transcoding cache directory has free space

## Database Issues

### Locked Out (Admin Password Lost)
1. Stop Jellyfin
2. Delete or rename ` + "`data/jellyfin.db`" + ` (this resets the database — back up first)
3. Restart Jellyfin and re-run setup wizard
4. Alternative: Use SQLite to edit the database directly

### Corrupted Database
1. Stop Jellyfin
2. Check for ` + "`jellyfin.db-shm`" + ` and ` + "`jellyfin.db-wal`" + ` files — delete them
3. Try ` + "`sqlite3 jellyfin.db \"PRAGMA integrity_check;\"`" + `
4. Restore from backup if integrity check fails

## Log File Interpretation

Use ` + "`jellyfin_system_info action=logs`" + ` to list log files, then ` + "`action=log_file name=<filename>`" + ` to read.

- **[ERR]** — Errors that need attention
- **[WRN]** — Warnings that may indicate issues
- **[INF]** — Informational, usually normal operation
- Common error patterns:
  - ` + "`IOException`" + ` — File access problems
  - ` + "`HttpRequestException`" + ` — Network/provider connectivity issues
  - ` + "`UnauthorizedAccessException`" + ` — Permission problems

## Sources

- https://jellyfin.org/docs/general/administration/troubleshooting/`

const librarySetupGuide = `# Library Organization Best Practices

## Content Type Separation

**Never mix movies and TV shows in the same library.** Jellyfin uses different metadata providers and naming parsers for each type.

Recommended structure:
~~~
/media/
  movies/          -> Library: Movies (type: movies)
  tv/              -> Library: TV Shows (type: tvshows)
  music/           -> Library: Music (type: music)
  audiobooks/      -> Library: Audiobooks (type: books)
  music-videos/    -> Library: Music Videos (type: musicvideos)
~~~

## Multiple Libraries for Same Content Type

Common use cases:
- **4K vs 1080p:** Separate libraries so 4K content can have different transcoding rules
- **Kids vs Adults:** Per-library access control for parental filtering
- **Anime vs Western TV:** Different metadata providers

## Storage Considerations

### Filesystem
- **EXT4/XFS (Linux):** Best performance and permission handling
- **NTFS:** Works but can cause permission issues, especially in Docker
- **Btrfs/ZFS:** Great for checksumming and snapshots

### Network Storage
- **NFS:** Preferred for Linux-to-Linux. Lower overhead, better performance.
- **SMB/CIFS:** Required for Windows shares. Use ` + "`vers=3.0`" + ` minimum.
- Mount network shares before starting Jellyfin to avoid scan issues

### Docker Volume Mounts

~~~yaml
volumes:
  - /path/to/config:/config
  - /path/to/cache:/cache      # Use SSD for transcoding cache
  - /media/movies:/movies:ro   # Read-only if Jellyfin shouldn't modify files
  - /media/tv:/tv:ro
  - /media/music:/music:ro
~~~

Use consistent mount paths and avoid symlinks inside containers.

## Sources

- https://jellyfin.org/docs/general/server/media/
- https://jellyfin.org/docs/general/administration/storage/`

const dockerGuide = `# Docker Deployment Guide

## Basic Docker Compose

~~~yaml
services:
  jellyfin:
    image: jellyfin/jellyfin:latest
    container_name: jellyfin
    user: "1000:1000"   # Match your host user UID:GID
    volumes:
      - ./config:/config
      - ./cache:/cache
      - /media:/media:ro
    ports:
      - "8096:8096"
    restart: unless-stopped
~~~

## GPU Passthrough

### Intel QSV / VAAPI

~~~yaml
services:
  jellyfin:
    image: jellyfin/jellyfin:latest
    devices:
      - /dev/dri/renderD128:/dev/dri/renderD128
    # If needed, also add:
    # group_add:
    #   - "render"   # or the GID of the render group
~~~

### NVIDIA

~~~yaml
services:
  jellyfin:
    image: jellyfin/jellyfin:latest
    runtime: nvidia
    deploy:
      resources:
        reservations:
          devices:
            - capabilities: [gpu]
    environment:
      - NVIDIA_VISIBLE_DEVICES=all
~~~

Requires ` + "`nvidia-container-toolkit`" + ` on the host.

## File Permissions

### PUID/PGID Method (linuxserver.io image)

~~~yaml
environment:
  - PUID=1000
  - PGID=1000
~~~

### Official Image

The official image uses ` + "`user:`" + ` directive:

~~~yaml
user: "1000:1000"
~~~

Run ` + "`id`" + ` on the host to find your UID/GID. Ensure media files are readable by this user.

## Volume Mounts

| Container Path | Purpose |
|---------------|---------|
| /config | Server configuration, database, metadata |
| /cache | Transcoding cache (use SSD) |
| /media | Media files (read-only recommended) |

## Networking

- **Bridge mode (default):** Use ` + "`ports:`" + ` mapping. Works with reverse proxies.
- **Host mode:** ` + "`network_mode: host`" + ` — required for DLNA/client discovery on local network. Not recommended with reverse proxy.

### Reverse Proxy Integration

When using a reverse proxy (Nginx, Caddy, Traefik), keep Jellyfin in bridge mode and proxy to the mapped port.

## Sources

- https://jellyfin.org/docs/general/installation/container/`

const usersAccessGuide = `# Users and Access Control

## User Creation and Permissions

Create users with ` + "`jellyfin_users action=create`" + ` and configure their permissions with ` + "`action=update_policy`" + `.

### Permission Model
- **Admin:** Full server access including settings, user management, and library configuration
- **Regular user:** Media access only, limited by library permissions
- **Disabled:** Account exists but cannot log in

## Per-Library Access Control

Restrict users to specific libraries:

~~~
jellyfin_users action=update_policy user_id=<id>
  enable_all_folders=false
  enabled_folder_ids=["library-id-1", "library-id-2"]
~~~

Use ` + "`jellyfin_libraries`" + ` to get library IDs.

## Parental Controls

### Content Rating Limits

In Dashboard > Users > select user > Access:
- Set maximum allowed content rating (e.g., PG-13)
- Items rated above this limit are hidden from the user

### Tag-Based Blocking

- Add custom tags to items (e.g., "violence", "mature-themes")
- In user settings, block specific tags
- Blocked items are hidden from that user

### Access Schedules

Restrict when a user can access the server:
- Set allowed days and time ranges in user policy
- Useful for children's accounts

## Known Limitations

- **Next Up widget:** May show restricted content titles (not playable, but visible in some clients)
- **Search:** May return titles from restricted libraries in some API responses
- Parental controls work best with properly rated content

## Third-Party Tools

### jfa-go (Jellyfin Account Manager)

An invitation-based user onboarding tool:
- Generate invite links with expiration
- Pre-configure user permissions per invite
- User self-registration with approval workflow
- Discord/Telegram bot integration

## Sources

- https://jellyfin.org/docs/general/server/users/adding-managing-users/`

const migrationGuide = `# Migrating to Jellyfin

## From Plex

### Library Setup
- Jellyfin can use the **same media folders** as Plex — no need to move files
- Point Jellyfin libraries at the same paths (e.g. ` + "`/media/movies`" + `, ` + "`/media/tv`" + `)
- Jellyfin will re-scan and fetch metadata from its own providers (TMDb, TVDB)

### Metadata Differences
- Plex uses its own metadata agents; Jellyfin uses TMDb, TVDB, and OMDb
- NFO files are supported — if your Plex setup generated NFO sidecar files, Jellyfin can read them
- Poster and backdrop images in the media folder (` + "`poster.jpg`" + `, ` + "`fanart.jpg`" + `) are picked up automatically

### Watch History Migration
- **Plex2Jellyfin** (community tool): Exports Plex watch history and imports into Jellyfin
- Manual approach: Use ` + "`jellyfin_user_data action=mark_played`" + ` for individual items
- For bulk marking: Use ` + "`jellyfin_user_data action=set_user_data`" + ` with play count and played status

### Plugin Equivalents
| Plex Feature | Jellyfin Equivalent |
|-------------|-------------------|
| PlexPass HW transcoding | Free — built into Jellyfin |
| Plex Collections | Box Sets (` + "`jellyfin_collections`" + `) |
| Plex Playlists | Playlists (` + "`jellyfin_playlists`" + `) |
| Skip Intro | Intro Skipper plugin |
| Tautulli | Playback Reporting plugin |
| Sub-Zero | Open Subtitles plugin |

## From Emby

### Compatibility
- Jellyfin forked from Emby in 2018, so the database schema is partially compatible
- **Direct database migration is NOT recommended** for recent Emby versions — schemas have diverged
- Use the same media folder paths and let Jellyfin re-scan

### Configuration
- Emby plugins are NOT compatible with Jellyfin
- Emby client apps will NOT connect to Jellyfin — use Jellyfin-specific clients
- Server configuration format differs — reconfigure transcoding, networking, and user settings

### User Accounts
- Create new user accounts in Jellyfin (` + "`jellyfin_users action=create`" + `)
- Set up per-library access controls (` + "`jellyfin_users action=update_policy`" + `)
- Watch history must be migrated manually or via community tools

## Post-Migration Checklist

1. **Verify libraries**: ` + "`jellyfin_libraries`" + ` — confirm all content types appear
2. **Check metadata**: ` + "`jellyfin_analytics action=library_stats`" + ` — verify item counts match expectations
3. **Set up transcoding**: ` + "`jellyfin_server action=get_config_section key=encoding`" + ` — configure hardware acceleration
4. **Create users**: ` + "`jellyfin_users action=list`" + ` — verify all accounts exist
5. **Test playback**: Use multiple clients to confirm direct play and transcoding work
6. **Install plugins**: ` + "`jellyfin_plugins action=list_packages`" + ` — install subtitle providers, intro skipper, etc.
7. **Set up remote access**: See jellyfin://guides/remote-access

## Sources

- https://jellyfin.org/docs/general/administration/migrate/`

const performanceGuide = `# Performance Tuning

## Hardware Transcoding Optimization

### GPU Selection
- **Intel QSV** is the most efficient for most users (low power, high quality)
- **NVIDIA NVENC** offers the highest throughput for concurrent streams
- **AMD AMF** works on Windows; VAAPI on Linux

### Key Settings
Use ` + "`jellyfin_server action=get_config_section key=encoding`" + ` to view current transcoding config.

- **EnableHardwareEncoding**: Must be ` + "`true`" + ` for GPU encoding
- **HardwareAccelerationType**: Match your GPU (` + "`qsv`" + `, ` + "`nvenc`" + `, ` + "`vaapi`" + `, ` + "`amf`" + `)
- **EnableDecodingColorDepth10Hevc/Vp9**: Enable for 10-bit HDR content
- **EnableTonemapping**: Required for HDR-to-SDR conversion
- **ThrottleDelaySeconds**: Set to 180 to reduce CPU when client buffer is full
- **TranscodingTempPath**: Point to an SSD for best performance

### Concurrent Stream Limits
| GPU | Approx. Concurrent 1080p Transcodes |
|-----|-------------------------------------|
| Intel i5 8th gen | 3-5 |
| Intel i7 10th gen | 5-8 |
| NVIDIA GTX 1050 | 2-3 |
| NVIDIA GTX 1660 | 5-8 |
| NVIDIA RTX 3060 | 8-15 |

## Database Maintenance

### SQLite Optimization
Jellyfin uses SQLite for its main database. Periodic maintenance helps:

1. **WAL mode**: Jellyfin uses WAL by default — verify with:
   ` + "```" + `
   sqlite3 /config/data/jellyfin.db "PRAGMA journal_mode;"
   ` + "```" + `
2. **VACUUM**: Reclaim space after bulk deletions:
   ` + "```" + `
   sqlite3 /config/data/jellyfin.db "VACUUM;"
   ` + "```" + `
3. **Integrity check**: Verify database health:
   ` + "```" + `
   sqlite3 /config/data/jellyfin.db "PRAGMA integrity_check;"
   ` + "```" + `

**Important**: Stop Jellyfin before running VACUUM or integrity_check.

## Cache Sizing

### Transcoding Cache
- Location: ` + "`/cache/transcodes`" + ` (Docker) or ` + "`/var/lib/jellyfin/transcodes`" + `
- **Use an SSD** — transcoding writes heavily to disk
- Size depends on concurrent streams: allocate ~5-10 GB per concurrent 1080p transcode
- Old segments are cleaned automatically

### Metadata Cache
- Posters, backdrops, and chapter images are cached in ` + "`/config/metadata`" + `
- A large library (10,000+ items) can use 5-20 GB of metadata storage
- Use ` + "`jellyfin_tasks action=list`" + ` to find the "Clean Cache Directory" task and run it periodically

## Storage I/O

### Disk Performance
- **Media storage**: HDD is fine for streaming (sequential reads)
- **Config/database**: SSD strongly recommended for snappy UI and fast library scans
- **Transcoding cache**: SSD required for smooth playback

### Network Storage
- **NFS**: Use ` + "`async`" + ` mount option for better read performance (safe for read-only media)
- **SMB**: Use ` + "`vers=3.0`" + ` minimum; avoid ` + "`vers=1.0`" + ` (slow, insecure)
- **iSCSI**: Best network performance, but more complex to set up

## Network Tuning

### Bandwidth Planning
| Content | Approx. Bandwidth |
|---------|-------------------|
| 1080p direct play | 5-20 Mbps |
| 4K HDR direct play | 40-80 Mbps |
| 1080p transcode (default) | 8-10 Mbps |
| Audio (FLAC) | 1-5 Mbps |

### Server-Side
- Set appropriate **remote bitrate limit** in Dashboard > Playback
- Enable **throttle transcoding** to reduce server load when client buffer is full
- Consider separate VLANs for media traffic on busy networks

### Client-Side
- Use clients that support direct play for your media codecs to avoid transcoding
- Set **maximum streaming bitrate** in client app settings for mobile/remote

## Monitoring

Use these tools to identify performance bottlenecks:
- ` + "`jellyfin_system_info action=info`" + ` — server hardware and version
- ` + "`jellyfin_analytics action=codec_report`" + ` — identify codecs that force transcoding
- ` + "`jellyfin_sessions action=list`" + ` — check what's being transcoded vs direct play
- ` + "`jellyfin_item_extras action=playback_info`" + ` — check if a specific item supports direct play
- ` + "`jellyfin_system_info action=log_file`" + ` — check for ffmpeg errors or performance warnings

## Sources

- https://jellyfin.org/docs/general/post-install/transcoding/
- https://jellyfin.org/docs/general/administration/troubleshooting/`

const pluginsGuide = `# Plugin Ecosystem Guide

## Plugin Repositories

### Default Repository
Jellyfin ships with its official plugin repository. Available in Dashboard > Plugins > Catalog.

### Adding Third-Party Repositories

In Dashboard > Plugins > Repositories, add a repository URL:

Use ` + "`jellyfin_plugins action=install package_name=<name> repository_url=<url>`" + ` to install from a specific repository.

## Recommended Plugins by Use Case

### Subtitles
- **Open Subtitles:** Most popular subtitle provider (requires free account)
- **SubBuzz:** Alternative subtitle search aggregator

### Metadata
- **TMDb Box Sets:** Auto-creates collections from TMDb
- **AniDB / AniList:** Anime metadata providers

### Stats and Monitoring
- **Playback Reporting:** Tracks play history, creates reports
- **Webhook:** Sends notifications on playback events (Discord, Gotify, etc.)

### Media Management
- **Shokofin:** Anime library management via Shoko Server
- **Bookshelf:** Enhanced book/comic metadata

### Client Enhancement
- **Intro Skipper:** Detects and allows skipping TV show intros
- **Chapter Segments Provider:** Marks intro/credits segments for skip

## Plugin Configuration Tips

1. After installing, check for plugin settings in Dashboard > Plugins > My Plugins
2. Some plugins require a server restart to activate
3. Use ` + "`jellyfin_plugins action=get_config plugin_id=<id>`" + ` to view configuration
4. Plugin updates are shown in the catalog — update regularly for bug fixes

## Sources

- https://jellyfin.org/docs/general/server/plugins/`
