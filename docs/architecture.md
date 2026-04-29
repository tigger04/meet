<!-- Version: 0.2 | Last updated: 2026-04-30 -->

# Architecture

## Overview

meet is a single Go binary with two subcommands (`serve` and `token`) and
a companion SSH wrapper (`remote-token`). The server embeds all static
assets (HTML template, font) and requires no external runtime dependencies.

## Components

```
                     ┌─────────────┐
                     │   Caddy     │ TLS termination, reverse proxy
                     │   :443      │
                     └──────┬──────┘
                            │
                     ┌──────▼──────┐
                     │  meet serve │ Go HTTP server
                     │  :18085     │
                     └──────┬──────┘
                            │
              ┌─────────────┼─────────────┐
              │             │             │
      ┌───────▼──────┐ ┌───▼───┐  ┌──────▼──────┐
      │ GET /{room}  │ │/health│  │POST /webhook│
      │ Meeting page │ │       │  │  /recording │
      └──────────────┘ └───────┘  └──────┬──────┘
                                         │
                                   ┌─────▼─────┐
                                   │  Download  │ async goroutine
                                   │  + Upload  │
                                   └─────┬─────┘
                                         │
                                   ┌─────▼─────┐
                                   │ Nextcloud  │ WebDAV PUT
                                   │  (remote)  │
                                   └───────────┘
```

## Request flow

### Meeting page (`GET /{room}`)

1. Extract room name from URL path (or use `default_room` for `/`)
2. Parse domain from `base_url` for banner rendering
3. Render embedded HTML template with room name, app-id, domain parts
4. Client-side JS loads JitsiMeetExternalAPI from 8x8 CDN
5. If `?jwt=` query param present, pass it to the API (moderator mode)

### Webhook (`POST /webhook/recording`)

1. Validate `Authorization` header against configured token
2. Parse JSON payload, log event metadata (all authenticated events are logged)
3. For download events (`RECORDING_UPLOADED`, `TRANSCRIPTION_UPLOADED`,
   `CHAT_UPLOADED`):
   - Check idempotency key against dedup map
   - Respond HTTP 200 immediately
   - Spawn goroutine for the download/upload pipeline:
     a. Download file to `{STATE_DIRECTORY}/download/{filename}`
     b. Upload to Nextcloud via WebDAV PUT
     c. On success, move file to `{STATE_DIRECTORY}/uploaded/`
     d. On failure, retry with exponential backoff (1m, 2m, 4m... up to 24h)
     e. If all retries fail, file remains in `download/` for recovery on restart
4. For all other events: log and respond HTTP 200

### Token generation (`meet token`)

1. Load config (defaults + host + secrets)
2. Parse RSA private key from PEM
3. Build JWT with moderator claims and recording feature flag
4. Print `{base_url}/{room}?jwt={signed_token}` to stdout

## Config layers

Config is merged left-to-right via `--config`:

1. `config/defaults.yaml` - committed, universal baseline
2. `config/<host>.yaml` - committed, host-specific (addr, base_url)
3. `secrets/<host>.yaml` - age-encrypted, host-specific (keys, credentials)

Later files override earlier ones. YAML unmarshalling is additive - fields
not present in a later file retain their earlier values.

## State

The server uses two forms of state:

**In-memory:** the dedup map (bounded at 1000 entries, evicts oldest) prevents
reprocessing of duplicate webhook deliveries within a single run. A restart
clears it, but this is safe - a redelivered webhook after restart will
re-download and re-upload, which is harmless (WebDAV PUT overwrites).

**On-disk:** the `STATE_DIRECTORY` (systemd's `/var/lib/meet/`) contains two
subdirectories that form a simple state machine:

- `download/` - files downloaded from 8x8 but not yet uploaded to Nextcloud.
  Presence of a file here means upload is pending or in retry.
- `uploaded/` - files successfully uploaded. Kept for 30 days as a local
  backup, then purged by a daily ticker.

On startup, the server scans `download/` and retries any pending uploads.
This recovers from crashes, restarts, and deployment-induced service restarts.

## Security

- Webhook endpoint validates a bearer token configured in secrets
- Moderator JWT is RS256-signed with a private key stored in secrets
- No secrets in committed config files - all in age-encrypted YAML
- Caddy handles TLS termination; the app binds to loopback only
- systemd runs the service with `DynamicUser=yes` and aggressive sandboxing
