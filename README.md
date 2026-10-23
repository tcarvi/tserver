# tserver

API-only HTTP gateway written in Go. Its only contract is
`POST /api/test-results` with `{"application":"…","version":"…"}`. The
request is forwarded to a Python API, which processes the test-result lookup
and returns the JSON response. The server has no static-file routes or other
API contracts.

The request must be exactly one JSON object with non-empty string fields
`application` and `version` (up to 128 bytes each). Unknown fields, additional
JSON values, or a body larger than the configured limit are rejected. The
Python response must be valid JSON and no larger than 4 MiB; its JSON body and
HTTP status below 500 are returned without reshaping. Upstream 5xx, timeouts,
and invalid responses become a generic JSON `502` response.

Every `POST /api/test-results` request requires a signed JWT with valid `iss`,
`sub`, and `exp` claims and the `api:read` permission. The permission may be
supplied in a `permissions` array or in the OAuth-style, space-separated
`scope` claim. The CORS preflight `OPTIONS` request is exempt from JWT
authentication.

JWTs must use HS256. `JWT_SECRET` must contain at least 32 bytes and
`JWT_ISSUER` must match the token issuer. The service fails at startup if
either setting is missing. OIDC discovery and JWKS validation for RS256 tokens
are not implemented.

`PYTHON_API_URL` is required and must be an absolute HTTP(S) URL. The server
listens on `:8080` by default; set `TSERVER_ADDR` to override the listen
address. Defaults are 10 requests per second per peer IP, burst 20, and a 1 MiB
request body limit. Forwarded-IP headers are intentionally ignored. Use a
trusted proxy or edge rate limiter when deploying behind a proxy.
The upstream timeout is 15 seconds and its JSON response is limited to 4 MiB.

CORS is disabled unless `CORS_ALLOWED_ORIGINS` contains exact,
comma-separated origins, such as `https://app.example.com`. Preflight is
handled after the IP rate limit and before JWT authentication. It permits only
`POST` and the `Authorization` and `Content-Type` headers.

Every request receives an `X-Request-ID`. Structured logs include the request
ID, method, path, response status, duration, application, version, and Python
status when available. Tokens and request/response bodies are not logged.

## Development

This module targets Go 1.20.13. In the local development environment, use the
`go120` command:

```sh
go120 version
go120 test ./...
go120 vet ./...
go120 build -o /tmp/tserver-api ./cmd/api
go120 run ./cmd/api
```

## Project layout

- `cmd/api`: process startup, dependency wiring, HTTP server and graceful shutdown.
- `internal/config`: environment loading and validation, including required secrets.
- `internal/dto/request` and `internal/dto/response`: inbound request and outbound response contracts. The test-result payload remains JSON defined by the Python service.
- `internal/handler`: HTTP contract binding, input validation and JSON responses.
- `internal/middleware`: JWT authentication/authorization, CORS, rate limiting, body limits, request logs and security headers.
- `internal/service`: orchestration of the test-results lookup.
- `internal/repository/python`: HTTP integration with the Python service. There is no local database in this contract.
- `internal/pkg/jwt` and `internal/pkg/requestid`: internal token parsing and request metadata utilities.
- `api/openapi.yaml`: API specification.
- `configs/example.env`: configuration template with a placeholder secret, not a real credential.

To run locally, configure a secret and issuer:

```sh
export JWT_SECRET="$(openssl rand -hex 32)"
export JWT_ISSUER="tserver-local"
export PYTHON_API_URL="http://127.0.0.1:8000/api/test-results"
go120 run ./cmd/api
```

Issue access tokens through a trusted identity service. A read token must
contain `iss`, `sub`, `exp`, and either `"permissions":["api:read"]` or
`"scope":"api:read"`. The request body accepts exactly the `application` and
`version` string fields; unknown fields and additional JSON values are rejected.

Example request (replace the token with one issued by the configured identity
provider):

```sh
curl -X POST http://localhost:8080/api/test-results \
	-H "Authorization: Bearer $ACCESS_TOKEN" \
	-H "Content-Type: application/json" \
	-d '{"application":"inventory","version":"1.2.3"}'
```

## Docker

The image uses Go 1.20 for the build and `bci-micro:15.5` at runtime. It is
statically linked and runs as an unprivileged user. Build from this repository
using the container project's Dockerfile:

```sh
docker build -f /path/to/server-go/Dockerfile -t tserver:local .
docker run --rm -p 8080:8080 \
	-e JWT_SECRET="$JWT_SECRET" \
	-e JWT_ISSUER="tserver-local" \
	-e PYTHON_API_URL="http://python-tests:8000/api/test-results" \
	-e CORS_ALLOWED_ORIGINS="https://app.example.com" \
	tserver:local
```

Do not commit or expose `JWT_SECRET`; use the deployment platform's secret
manager. The container and Python service must share a Docker network so
`PYTHON_API_URL` resolves. CORS can be left unset for non-browser clients. To
change the listen address, set `TSERVER_ADDR` and publish the matching port.