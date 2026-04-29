<!-- Version: 0.1 | Last updated: 2026-04-29 -->

# Vision

meet is a self-hosted, branded wrapper around 8x8 JaaS (Jitsi as a Service)
for small-scale video meetings. The goal is a simple, scriptable meeting
tool with full control over branding, authentication, and data retention.

## Goals

- **Minimal surface area.** A single Go binary serves the meeting page,
  handles webhooks, and generates moderator tokens. No database, no
  background workers, no external dependencies beyond 8x8 and Nextcloud.
- **Own your data.** Recordings, transcriptions, and chat logs are
  automatically archived to a self-hosted Nextcloud instance before the
  24-hour 8x8 download link expires.
- **CLI-first administration.** Moderator access is generated via CLI
  (`meet token`), not a web admin panel. No authentication system to
  build or maintain for a single-user deployment.
- **Scriptable and composable.** Room names are URL paths. Token generation
  is a single command. The webhook endpoint is standard HTTP. Everything
  integrates with shell scripts and automation.

## Non-goals

- Multi-tenant or multi-user admin (single operator assumed)
- Custom video infrastructure (delegates to 8x8 JaaS)
- Mobile apps (the web UI is responsive via JaaS)
- User accounts or persistent sessions
