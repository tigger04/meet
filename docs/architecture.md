<!-- Version: 0.1 | Last updated: 2026-04-29 -->

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
2. Parse JSON payload, log event metadata
3. For download events (`RECORDING_UPLOADED`, `TRANSCRIPTION_UPLOADED`,
   `CHAT_UPLOADED`):
   - Check idempotency key against dedup map
   - Respond HTTP 200 immediately
   - Spawn goroutine: download from `preAuthenticatedLink`, upload to
     Nextcloud via WebDAV PUT
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

The server is stateless. The only mutable state is the in-memory dedup map
(bounded at 1000 entries, evicts oldest). A restart clears it - acceptable
because webhook redelivery after a restart is harmless (the file already
exists on Nextcloud, and a duplicate PUT overwrites with identical content).

## Security

- Webhook endpoint validates a bearer token configured in secrets
- Moderator JWT is RS256-signed with a private key stored in secrets
- No secrets in committed config files - all in age-encrypted YAML
- Caddy handles TLS termination; the app binds to loopback only
- systemd runs the service with `DynamicUser=yes` and aggressive sandboxing
