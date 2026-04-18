# meet

Branded 8x8 JaaS (Jitsi as a Service) meeting page. A lightweight Go web app
that serves video meeting rooms with a branded banner and moderator JWT support.

Visitors go to `meet.lobb.ie/workshop-april` and join a room. The moderator
generates a signed JWT URL via the CLI to get admin privileges.

## Quickstart

```bash
make init          # creates config/localhost.yaml from example
# edit config/localhost.yaml (addr, base_url)
# create secrets/localhost.yaml with 8x8-keys (app-id, key-id, private-key)
make serve         # start the server
make token ROOM=my-room   # generate a moderator URL
```

## Dependencies

- Go 1.24+
- 8x8 JaaS account with API key

## CLI

```
meet                        # start the web server (default: serve)
meet serve --config ...     # start with explicit config files
meet token --room <name>    # generate a moderator JWT URL
meet --help                 # show usage
meet --version              # print version
```

### remote-token (installed as meet-token)

```
meet-token <host> <room>    # SSH to host and generate a moderator URL
```

## Config

Three-layer config merged left-to-right via `--config`:

1. `config/defaults.yaml` - universal baseline (committed)
2. `config/<host>.yaml` - host-specific overrides (committed, no secrets)
3. `secrets/<host>.yaml` - secrets (gitignored locally, age-encrypted for deploy)

### Config fields

| Field | Location | Description |
|-------|----------|-------------|
| `addr` | config | Bind address (default `127.0.0.1:18085`) |
| `base_url` | config | Public URL, used for banner and token URLs |
| `default_room` | config | Room name when visiting `/` (default `lobby`) |
| `8x8-keys.app-id` | secrets | 8x8 JaaS application ID |
| `8x8-keys.key-id` | secrets | 8x8 API key ID (used as JWT `kid` header) |
| `8x8-keys.private-key` | secrets | RSA private key PEM for JWT signing |
| `8x8-keys.public-key` | secrets | RSA public key PEM (not used at runtime) |

## Important files

| Path | Purpose |
|------|---------|
| `cmd/meet/main.go` | Entrypoint: serve and token subcommands |
| `cmd/remote-token/main.go` | SSH wrapper for remote token generation |
| `internal/server/server.go` | HTTP server, routing, domain parsing |
| `internal/server/static/index.html` | Meeting page template (embedded) |
| `internal/server/static/SpecialElite-Regular.woff2` | Banner font (embedded) |
| `config/defaults.yaml` | Default config |
| `config/<host>.yaml` | Per-host config overrides |
| `secrets/<host>.yaml` | Per-host secrets (gitignored/age-encrypted) |
| `docs/8x8-embed.md` | 8x8 JaaS embed API reference |

## Deployment

Deployed via `deploy-app` from the hetzner repo. The Makefile contains local
dev targets only - no deploy, SSH, or systemd targets.

## Licence

MIT - Copyright Tadg Paul
