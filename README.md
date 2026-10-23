# tserver

API-only HTTP server written in Go. All non-API routes return `404`; the
server does not serve HTML, JavaScript, CSS, or other web application assets.

The server exposes `/api` from files under `github.com/tcarvi/api/`, relative
to its working directory. It listens on `:8080` by default; set `TSERVER_ADDR`
to override the listen address.

## Development

This module targets Go 1.20.13. In the local development environment, use the
`go120` command:

```sh
go120 version
go120 test ./...
go120 run .
```