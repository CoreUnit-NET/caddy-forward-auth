# intern-auth-gateway

![CI/CD](https://github.com/CoreUnit-NET/intern-auth-gateway/actions/workflows/go-bin-release.yml/badge.svg)
![CI/CD](https://github.com/CoreUnit-NET/intern-auth-gateway/actions/workflows/go-test-build.yml/badge.svg)  
![MIT](https://img.shields.io/badge/license-MIT-blue.svg)
![](https://img.shields.io/badge/dynamic/json?color=green&label=watchers&query=watchers&suffix=x&url=https%3A%2F%2Fapi.github.com%2Frepos%2FCoreUnit-NET%2Fintern-auth-gateway)
![](https://img.shields.io/badge/dynamic/json?color=yellow&label=stars&query=stargazers_count&suffix=x&url=https%3A%2F%2Fapi.github.com%2Frepos%2FCoreUnit-NET%2Fintern-auth-gateway)
![](https://img.shields.io/badge/dynamic/json?color=navy&label=forks&query=forks&suffix=x&url=https%3A%2F%2Fapi.github.com%2Frepos%2FCoreUnit-NET%2Fintern-auth-gateway)

intern-auth-gateway is a small HTTP auth service for Caddy `forward_auth`.
It verifies HTTP Basic credentials against per-service bcrypt hashes and only then lets Caddy allow access to the protected upstream hosts.
After a successful login it can temporarily whitelist the client IP so browsers do not need to resend Basic auth on every request.

# Table of Contents

- [Features](#features)
- [Out of scope](#out-of-scope)
- [Security notes](#security-notes)
- [Usage with Caddy](#usage-with-caddy)
- [Configuration](#configuration)
  - [Flags and environment](#flags-and-environment)
  - [Services](#services)
- [Getting Started](#getting-started)
  - [Requirements](#requirements)
  - [Use via Go](#use-via-go)
  - [Install via go](#install-via-go)
  - [Install via GitHub release](#install-via-github-release)
- [Build](#build)
  - [Build requirements](#build-requirements)
  - [Build Instructions](#build-instructions)
- [Development](#development)
- [Docker](#docker)
- [Install go](#install-go)
- [Contributing](#contributing)
- [License](#license)
- [Disclaimer](#disclaimer)

## Features

- **Caddy `forward_auth` endpoint**: Exposes exact paths `/` and `/auth` for Caddy `forward_auth` probes. Successful checks return `200` and set `Remote-User`; failures return `401` with a Basic challenge (or `403` for blocked origins). Other paths return `404`.
- **HTTP Basic authentication**: Credentials are verified against bcrypt password hashes configured per service.
- **Per-service auth routing**: Each `SERVICE_*` entry maps a host pattern to its own username and password hash, so different upstream hosts can require different credentials.
- **Host glob matching**: Case-insensitive. `*` matches one DNS label (for example `*.intern.example.com` matches `api.intern.example.com`, but not `a.b.intern.example.com`). Bare `*` matches any non-empty host. Ports in the request host are ignored.
- **Origin allowlist**: Optional `ALLOWED_ORIGINS` CSV restricts browser `Origin` hostnames. Requests without an `Origin` header (typical for Caddy probes) are allowed.
- **Target host resolution**: The protected host is taken from `X-Forwarded-Host` (first value if CSV), falling back to the request `Host`.
- **Startup checks**: Boot fails when no `SERVICE_*` entries are configured or when a password hash is not valid bcrypt.
- **Auth event logs**: Every probe logs a short line with `status`, `path`, `host`, chosen `service`, `user`, and `reason` (no passwords).
- **Temporary IP whitelist**: After successful Basic auth, the client IP is remembered for a limited time (default 48h) so follow-up probes succeed without a new password prompt (`reason=whitelisted`). State is persisted under `./data/ipwhitelist.json` (not cookies or sessions).
- **Flood prevention**: Failed Basic attempts (`no_credentials`, `auth_failed`) are tracked per client IP. Escalating thresholds create temporary or permanent IP bans (`403`, `reason=banned` / `temp_banned`). State lives in `./data/flood.json` and `./data/ban.json`.

## Out of scope

- **TLS / HTTPS**: Terminate TLS in front of this service (for example with Caddy). The gateway itself listens on plain HTTP.
- **Cookie / session login**: No browser cookies or server sessions. Temporary IP whitelisting is used instead of a session store.
- **Non-Basic auth**: OAuth, OIDC, API keys, mTLS, and similar methods are not supported.
- **Reverse-proxy duties**: This process only answers auth probes. Upstream proxying, routing, and TLS remain Caddy’s responsibility.
- **Non-Caddy gatekeeping**: The handler is built for Caddy `forward_auth`. Other proxy auth protocols are not a goal.
- **Additional rate limiting beyond built-in flood bans**: Network placement and Caddy remain the first line of defense; this process only applies the fixed flood thresholds above.

## Security notes

- Keep this process on a **private network** (or localhost) reachable by Caddy only. Do not publish the auth port to the internet.
- Trust `X-Forwarded-Host` / `Host` only in that trusted path. Direct public exposure lets clients pick which service glob they authenticate against.
- When `ALLOWED_ORIGINS` is set, missing `Origin` is still allowed (needed for typical `forward_auth` probes).
- Short auth event logs always include hostnames and usernames (not passwords).
- `--verbose` / `VERBOSE` additionally dumps every registered service on startup **including password hashes**, plus allowed origins. Use only while debugging on a private network.
- Put secrets in `.env` (loaded automatically if present) or your secret manager; never commit real password hashes.
- The temporary IP whitelist file (`./data/ipwhitelist.json` by default) and flood/ban files (`./data/flood.json`, `./data/ban.json`) trust client IPs as seen via `X-Forwarded-For` / `X-Real-IP` / `RemoteAddr`—keep the gateway on a private network behind Caddy so those addresses are meaningful.
- Flood thresholds (per IP): 10 failures / 2m → 3m ban; 60 / 30m → 2h ban; 90 / 60m, 120 / 6h, or 240 / 168h → permanent ban. Temp-banned clients still accumulate flood events.

## Usage with Caddy

Example snippet (adapt hostnames and upstreams):

```caddyfile
intern-auth.example.com {
	reverse_proxy 127.0.0.1:8080
}

*.intern.example.com {
	forward_auth 127.0.0.1:8080 {
		uri /auth
		copy_headers Remote-User
	}
	reverse_proxy 127.0.0.1:9000
}
```

Flow:

1. Caddy sends an internal auth probe to this gateway (`/` or `/auth`).
2. On `200`, Caddy allows the client request and may forward `Remote-User`.
3. On `401`/`403`, Caddy denies access.

## Configuration

CLI flags and environment variables can both be used. **Flags override env values.** Boolean flags default to `false`.

A `.env` file in the working directory is loaded at startup when present (missing file is ignored).

Running the binary with no subcommand starts the HTTP server (same as `serve`).  
Additional commands: `serve`, `version` (also `-v` / `--version`).

### Flags and environment

| Flag                | Env Var           | Type | Default   | Description |
| ------------------- | ----------------- | ---- | --------- | ----------- |
| `--verbose` / `-b`  | `VERBOSE`         | bool | `false`   | Verbose mode: dump services (with password hashes) and allowed origins at startup |
| `--host`            | `HOST`            | str  | `0.0.0.0` | Listen address |
| `--port`            | `PORT`            | int  | `8080`    | Listen port |
| `--allowed-origins` | `ALLOWED_ORIGINS` | CSV  | _(empty)_ | Allowed `Origin` hostnames. When set, other origins are rejected with `403`. Empty disables origin enforcement |

Quick help:

```sh
go run github.com/CoreUnit-NET/intern-auth-gateway@latest -h
```

### Services

Services are configured **only through environment variables** with the `SERVICE_` prefix.  
At least one valid `SERVICE_*` entry is required or startup fails.

| Piece | Rule |
| ----- | ---- |
| Env key | `SERVICE_<name>` (name must be non-empty) |
| Value | `hostGlob/username/passwordHash` |
| Parsing | Split on `/` with at most **two** separators (`SplitN`); the bcrypt hash may contain `/` |
| `hostGlob` | Exact hostname, single-label `*` (for example `*.intern.example.com`), or bare `*` for any host |
| `username` | Must be **unique** across all `SERVICE_*` entries (startup fails on duplicates) |
| `passwordHash` | Valid bcrypt hash (startup fails on invalid hashes) |
| Overlapping globs | Allowed; startup logs a warning when two `SERVICE_*` host globs can match the same host |

Example:

```sh
PORT=8080
HOST=0.0.0.0
ALLOWED_ORIGINS="intern-auth.example.com, localhost, auth-test.example.com"
SERVICE_test="test.example.com/tester/$2a$14$AnhQELX1cqeO3YaLPOTWtOuPsKZgweRHrYLcqzQUcvokbVZmzNWrO"
SERVICE_intern="*.intern.example.com/intern-user/$2a$14$54tdWftb4iOouKyfDyURPuI6rOIwcbjqKYfzOqYE0PyOcmVFnU1mM"
```

See `sample.env` for a copy-paste template.

## Getting Started

### Requirements

Linux- or macos-like systems with:

- `go` **or** `wget` & `tar` (to run/install the intern-auth-gateway binary)

### Use via Go

Help and configuration details:

```sh
go run github.com/CoreUnit-NET/intern-auth-gateway@latest -h
```

Start with defaults (same as `serve`):

```sh
go run github.com/CoreUnit-NET/intern-auth-gateway@latest
```

### Install via go

###### _For this section go is required, check out the [install go guide](#install-go)._

```sh
go install github.com/CoreUnit-NET/intern-auth-gateway@latest
```

### Install via GitHub release

1. Open the [GitHub Releases](https://github.com/CoreUnit-NET/intern-auth-gateway/releases) page.
2. Download the archive for your OS/arch (release assets are produced as `intern-auth-gateway_<os>_<arch>.tar.gz`, for example `intern-auth-gateway_linux_amd64.tar.gz`).
3. Extract the `intern-auth-gateway` binary into your preferred bin directory:

```sh
export CUSTOM_BIN_DIR="/usr/local/bin" # <- change if needed
tar -xzvf intern-auth-gateway_linux_amd64.tar.gz -C "$CUSTOM_BIN_DIR" intern-auth-gateway
chmod +x "$CUSTOM_BIN_DIR/intern-auth-gateway"
```

# Build

## Build requirements

To build, you need to install go.
The required go version is in the `go.mod` file.

## Build Instructions

###### _For this section go is required, check out the [install go guide](#install-go)._

Clone the repo:

```sh
git clone https://github.com/CoreUnit-NET/intern-auth-gateway.git
cd intern-auth-gateway
```

Build the binary from source (`make build` writes `./bin`):

```sh
make build
./bin
```

# Development

###### _For this section go is required, check out the [install go guide](#install-go)._

Auto-reload via [Air](https://github.com/air-verse/air) (installs the tool if needed):

```sh
make dev
```

Useful make targets: `make test`, `make build`, `make cover`.

# Docker

Docker Compose services:

- `local` — Air reload with the repo mounted
- `deploy` — built runtime image
- `lint` — golangci-lint

```sh
make docker      # shell in local image
make docker/run  # run with Air and published ports
```

Compose publishes `127.0.0.1:${PORT:-8080}` on the host to container port `8080`, and forces `PORT=8080` inside the container so the app listen port stays aligned.

## Install go

The required go version for this project is in the `go.mod` file.

To install and update go, I can recommend the following repo:

```sh
git clone git@github.com:udhos/update-golang.git golang-updater
cd golang-updater
sudo ./update-golang.sh
```

# Contributing

Contributions to this project are welcome!  
Interested users can refer to the guidelines provided in the [CONTRIBUTING.md](CONTRIBUTING.md) file to contribute to the project and help improve its functionality and features.

# License

This project is licensed under the [MIT license](LICENSE), providing users with flexibility and freedom to use and modify the software according to their needs.

# Disclaimer

This project is provided without warranties.  
Users are advised to review the accompanying license for more information on the terms of use and limitations of liability.