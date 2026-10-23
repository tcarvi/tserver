# tserver

API-only HTTP server written in Go. All non-API routes return `404`; the
server does not serve HTML, JavaScript, CSS, or other web application assets.

The server exposes files under `github.com/tcarvi/api/` at `/api/...`, relative
to its working directory. Only `GET` and `HEAD` are supported. Every API
request requires a signed JWT with valid `iss`, `sub`, and `exp` claims and the
`api:read` permission. The permission may be supplied in a `permissions` array
or in the OAuth-style, space-separated `scope` claim.

JWTs must use HS256. `JWT_SECRET` must contain at least 32 bytes and
`JWT_ISSUER` must match the token issuer. The service fails at startup if
either setting is missing. OIDC discovery and JWKS validation for RS256 tokens
are not implemented.

The server listens on `:8080` by default; set `TSERVER_ADDR` to override the
listen address. Defaults are 10 requests per second per peer IP, burst 20, and
a 1 MiB request body limit. Forwarded-IP headers are intentionally ignored.
Use a trusted proxy or edge rate limiter when deploying behind a proxy.

CORS is disabled unless `CORS_ALLOWED_ORIGINS` contains exact,
comma-separated origins, such as `https://app.example.com`. Preflight is
handled before authentication and permits only `GET`/`HEAD` and the
`Authorization` and `Content-Type` headers.

## Development

This module targets Go 1.20.13. In the local development environment, use the
`go120` command:

```sh
go120 version
go120 test ./...
go120 vet ./...
go120 run .
```

To run locally, configure a secret and issuer:

```sh
export JWT_SECRET="$(openssl rand -hex 32)"
export JWT_ISSUER="tserver-local"
go120 run .
```

Issue access tokens through a trusted identity service. A read token must
contain `iss`, `sub`, `exp`, and either `"permissions":["api:read"]` or
`"scope":"api:read"`.

## Docker

The image uses Go 1.20 for the build and `bci-micro:15.5` at runtime. It is
statically linked and runs as an unprivileged user. Build from this repository
using the container project's Dockerfile:

```sh
docker build -f /path/to/server-go/Dockerfile -t tserver:local .
docker run --rm -p 8080:8080 \
	-e JWT_SECRET="$JWT_SECRET" \
	-e JWT_ISSUER="tserver-local" \
	-e CORS_ALLOWED_ORIGINS="https://app.example.com" \
	-v "$PWD/github.com/tcarvi/api:/app/webapp/github.com/tcarvi/api:ro" \
	tserver:local
```

Do not commit or expose `JWT_SECRET`; use the deployment platform's secret
manager. CORS can be left unset for non-browser clients. To change the listen
address, set `TSERVER_ADDR` and publish the matching port.