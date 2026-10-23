# tserver

`tserver` is an authenticated Go HTTP gateway for retrieving test results from a
Python API. It currently implements one synchronous endpoint:

- `POST /api/test-results` accepts `{"application":"…","version":"…"}` and
  returns the valid JSON response from Python.

This is not an asynchronous job API: `POST /api/test-runs` and
`GET /api/test-runs/{test_id}` are not implemented. See
[`api/openapi.yaml`](api/openapi.yaml) for the runtime contract and
[`CONTRIBUTING.md`](CONTRIBUTING.md) for architecture, security, configuration,
development, and deployment details.

## Requirements and development

The module targets Go 1.20.13. In the local development environment, use the
`go120` command from the module root:

```sh
go120 test ./...
go120 vet ./...
go120 build -o /tmp/tserver-api ./cmd/api
go120 run ./cmd/api
```

To run locally, configure the required settings:

```sh
export JWT_SECRET="$(openssl rand -hex 32)"
export JWT_ISSUER="tserver-local"
export PYTHON_API_URL="http://127.0.0.1:8000/api/test-results"
go120 run ./cmd/api
```

Requests need a trusted HS256 JWT with the configured issuer, a non-empty
subject, an expiration, and the `api:read` permission. The request body must be
one JSON object with non-empty `application` and `version` strings, each at
most 128 bytes.

Example (replace the placeholder with a token issued by a trusted identity
provider):

```sh
curl -i -X POST http://localhost:8080/api/test-results \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"application":"inventory","version":"1.2.3"}'
```

The Python service owns the result schema. Its valid JSON body and HTTP status
below 500 are forwarded by the gateway. Upstream 5xx responses, timeouts,
invalid JSON, non-JSON content types, and responses larger than 4 MiB become a
generic JSON `502` response.
