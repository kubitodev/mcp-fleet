# upsnap-mcp

First-party MCP over [UpSnap](https://github.com/seriousm4x/UpSnap)'s PocketBase
API — device visibility + Wake-on-LAN / power control. Built and published by
this repo as `ghcr.io/kubitodev/upsnap-mcp` (no upstream MCP exists).

Transport: StreamableHTTP at `/mcp` on `:8080`. Liveness at `/healthz`.

## Tools

Read (`readOnlyHint`):
- `upsnap_list_devices` — all devices with live status (online/offline/pending), ip, mac, groups
- `upsnap_get_device` — one device by id or name
- `upsnap_list_groups` — device groups

Power:
- `upsnap_wake` / `upsnap_shutdown` / `upsnap_reboot` / `upsnap_sleep` — by device id or name
- `upsnap_wake_group` / `upsnap_shutdown_group` — by group id or name
- `upsnap_switch_boot` — reboot a machine into an alternate boot target (dual-boot).
  Pass the UpSnap **helper** device whose shutdown command performs the boot switch
  (e.g. an SSH bootloader one-shot + reboot); it shares the real machine's MAC/IP.
  Online → switches now; offline → wakes it (boots the default OS first) and
  switches automatically once it's back online (completed in the background, so
  the call returns immediately).

`shutdown`/`reboot`/`sleep` (and `shutdown_group`, `switch_boot`) carry `destructiveHint`.

## Config (env)

| Var | Required | Default | Notes |
| --- | --- | --- | --- |
| `UPSNAP_URL` | yes | | UpSnap base URL, e.g. `http://upsnap:8090` |
| `UPSNAP_MCP_USER` | yes | | PocketBase identity (email/username) |
| `UPSNAP_MCP_PASSWORD` | yes | | its password |
| `UPSNAP_MCP_AUTH_COLLECTION` | no | (auto) | pin one collection; unset mirrors UpSnap's login (tries `_superusers`, then `users`) |
| `UPSNAP_MCP_AUTH_TOKEN` | no | | shared bearer guarding `/mcp` (strongly recommended) |
| `UPSNAP_MCP_READ_ONLY` | no | `false` | `true` drops all power tools |
| `UPSNAP_MCP_HTTP_ADDR` | no | `:8080` | listen address |

PocketBase has no static API token: the server logs in with user/password to get
a short-lived JWT and re-auths automatically on a 401.
