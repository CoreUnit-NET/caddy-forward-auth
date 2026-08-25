<div align="center">

# 🔐 caddy-forward-auth

![MIT](https://img.shields.io/badge/license-MIT-blue.svg)
![CI/CD](https://github.com/CoreUnit-NET/caddy-forward-auth/actions/workflows/go-bin-release.yml/badge.svg)
![CI/CD](https://github.com/CoreUnit-NET/caddy-forward-auth/actions/workflows/go-test-build.yml/badge.svg)  
![](https://img.shields.io/badge/dynamic/json?color=green&label=watchers&query=watchers&suffix=x&url=https%3A%2F%2Fapi.github.com%2Frepos%2FCoreUnit-NET%2Fcaddy-forward-auth)
![](https://img.shields.io/badge/dynamic/json?color=yellow&label=stars&query=stargazers_count&suffix=x&url=https%3A%2F%2Fapi.github.com%2Frepos%2FCoreUnit-NET%2Fcaddy-forward-auth)
![](https://img.shields.io/badge/dynamic/json?color=navy&label=forks&query=forks&suffix=x&url=https%3A%2F%2Fapi.github.com%2Frepos%2FCoreUnit-NET%2Fcaddy-forward-auth)

</div>

## About

**caddy-forward-auth** is a small companion process for [Caddy](https://caddyserver.com/) `forward_auth`.
Caddy keeps TLS, routing, and reverse-proxying; this service only answers the auth probes (`/` and `/auth`) and tells Caddy whether the client may reach a protected host.

Configure one or more `SERVICE_*` entries (host glob + username + bcrypt password hash).
On each probe the service resolves the target host (`X-Forwarded-Host` / `Host`), matches a service entry, and checks HTTP Basic credentials.
A successful login returns `200` (and `Remote-User`); failures return `401` with a Basic challenge.
After a success the client IP can stay temporarily whitelisted (default 48h, no cookies/sessions) so browsers are not prompted again on every request.
Repeated failed attempts are flood-tracked and can escalate into temporary or permanent IP bans.

Run it on a private network (or localhost) reachable only by Caddy—not on the public internet.
State for whitelist, flood events, and bans is persisted under `./data/` by default.

<details><summary><strong>Features</strong></summary>

### Features

- **Caddy `forward_auth` endpoint**: Exposes exact paths `/` and `/auth` for Caddy `forward_auth` probes. Successful Basic checks return `200` and set `Remote-User`; failures return `401` with a Basic challenge (or `403` for blocked origins). Other paths return `404`.
- **HTTP Basic authentication**: Credentials are verified against bcrypt password hashes configured per service.
- **Per-service auth routing**: Each `SERVICE_*` entry maps a host pattern to its own username and password hash, so different upstream hosts can require different credentials.
- **Host glob matching**: Case-insensitive. `*` matches one DNS label (for example `*.intern.example.com` matches `api.intern.example.com`, but not `a.b.intern.example.com`). Bare `*` matches any non-empty host. Ports in the request host are ignored.
- **Origin allowlist**: Optional `ALLOWED_ORIGINS` CSV restricts browser `Origin` hostnames. Requests without an `Origin` header (typical for Caddy probes) are allowed.
- **Target host resolution**: The protected host is taken from `X-Forwarded-Host` (first value if CSV), falling back to the request `Host`.
- **Startup checks**: Boot fails when no `SERVICE_*` entries are configured or when a password hash is not valid bcrypt.
- **Auth event logs**: Every probe logs a short line with `status`, `path`, `host`, chosen `service`, `user`, and `reason` (no passwords).
- **Temporary IP whitelist**: After successful Basic auth, the client IP is remembered for a limited time (default 48h) so follow-up probes succeed without a new password prompt (`reason=whitelisted`). Whitelist hits return `200` but do **not** set `Remote-User` (only a full Basic success does). State is persisted under `./data/ipwhitelist.json` (not cookies or sessions).
- **Flood prevention**: Failed Basic attempts (`no_credentials`, `auth_failed`) are tracked per client IP. Escalating thresholds create temporary or permanent IP bans (`403`, `reason=banned` / `temp_banned`). State lives in `./data/flood.json` and `./data/ban.json`.

</details>

<details><summary><strong>Out of scope</strong></summary>

### Out of scope

- **TLS / HTTPS**: Terminate TLS in front of this service (for example with Caddy). The process itself listens on plain HTTP.
- **Cookie / session login**: No browser cookies or server sessions. Temporary IP whitelisting is used instead of a session store.
- **Non-Basic auth**: OAuth, OIDC, API keys, mTLS, and similar methods are not supported.
- **Reverse-proxy duties**: This process only answers auth probes. Upstream proxying, routing, and TLS remain Caddy’s responsibility.
- **Non-Caddy gatekeeping**: The handler is built for Caddy `forward_auth`. Other proxy auth protocols are not a goal.
- **Additional rate limiting beyond built-in flood bans**: Network placement and Caddy remain the first line of defence; this process only applies the fixed flood thresholds above.

</details>

<details><summary><strong>Security notes</strong></summary>

### Security notes

- Keep this process on a **private network** (or localhost) reachable by Caddy only. Do not publish the auth port to the internet.
- Trust `X-Forwarded-Host` / `Host` only in that trusted path. Direct public exposure lets clients pick which service glob they authenticate against.
- When `ALLOWED_ORIGINS` is set, missing `Origin` is still allowed (needed for typical `forward_auth` probes).
- Short auth event logs always include hostnames and usernames (not passwords).
- `--verbose` / `VERBOSE` additionally dumps every registered service on startup **including password hashes**, plus allowed origins. Use only while debugging on a private network.
- Put secrets in `.env` (loaded automatically if present) or your secret manager; never commit real password hashes.
- The temporary IP whitelist file (`./data/ipwhitelist.json` by default) and flood/ban files (`./data/flood.json`, `./data/ban.json`) trust client IPs as seen via `X-Forwarded-For` / `X-Real-IP` / `RemoteAddr`—keep the service on a private network behind Caddy so those addresses are meaningful.
- Whitelist `200` responses omit `Remote-User`; only a successful Basic login sets that header for Caddy `copy_headers`.
- Flood thresholds (per IP): 10 failures / 2m → 3m ban; 60 / 30m → 2h ban; 90 / 60m, 120 / 6h, or 240 / 168h → permanent ban. Temp-banned clients still accumulate flood events.

</details>

<details><summary><strong>Usage with Caddy</strong></summary>

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

1. Caddy sends an internal auth probe to this service (`/` or `/auth`).
2. On `200`, Caddy allows the client request and may forward `Remote-User` when the probe set it (Basic success only; whitelist hits omit it).
3. On `401`/`403`, Caddy denies access.

</details>

<details><summary><strong>Configuration</strong></summary>

## Configuration

CLI flags and environment variables can both be used. **Flags override env values.** Boolean flags default to `false`.

A `.env` file in the working directory is loaded at startup when present (missing file is ignored).

Running the binary with no subcommand starts the HTTP server (same as `serve`).  
Additional commands: `serve`, `version` (also `-v` / `--version`).

### Flags and environment

| Flag                | Env Var           | Type | Default   | Description                                                                                                                                                                               |
| ------------------- | ----------------- | ---- | --------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `--verbose` / `-b`  | `VERBOSE`         | bool | `false`   | Verbose mode: dump services (with password hashes) and allowed origins at startup                                                                                                         |
| `--host`            | `HOST`            | str  | `0.0.0.0` | Listen address                                                                                                                                                                            |
| `--port`            | `PORT`            | int  | `8080`    | Listen port                                                                                                                                                                               |
| `--allowed-origins` | `ALLOWED_ORIGINS` | CSV  | _(empty)_ | Allowed `Origin` hostnames/globs (same rules as `SERVICE_*` hostGlob: exact, `*.example.com`, or `*`). When set, other origins are rejected with `403`. Empty disables origin enforcement |

Quick help:

```sh
go run github.com/CoreUnit-NET/caddy-forward-auth@latest -h
```

### Services

Services are configured **only through environment variables** with the `SERVICE_` prefix.  
At least one valid `SERVICE_*` entry is required or startup fails.

| Piece             | Rule                                                                                            |
| ----------------- | ----------------------------------------------------------------------------------------------- |
| Env key           | `SERVICE_<name>` (name must be non-empty)                                                       |
| Value             | `hostGlob/username/passwordHash`                                                                |
| Parsing           | Split on `/` with at most **two** separators (`SplitN`); the bcrypt hash may contain `/`        |
| `hostGlob`        | Exact hostname, single-label `*` (for example `*.intern.example.com`), or bare `*` for any host |
| `username`        | Must be **unique** across all `SERVICE_*` entries (startup fails on duplicates)                 |
| `passwordHash`    | Valid bcrypt hash (startup fails on invalid hashes)                                             |
| Overlapping globs | Allowed; startup logs a warning when two `SERVICE_*` host globs can match the same host         |

Example:

```sh
PORT=8080
HOST=0.0.0.0
ALLOWED_ORIGINS="intern-auth.example.com, localhost, *.intern.example.com"
SERVICE_test="test.example.com/tester/$2a$14$AnhQELX1cqeO3YaLPOTWtOuPsKZgweRHrYLcqzQUcvokbVZmzNWrO"
SERVICE_intern="*.intern.example.com/intern-user/$2a$14$54tdWftb4iOouKyfDyURPuI6rOIwcbjqKYfzOqYE0PyOcmVFnU1mM"
```

See `.env.sample` for a copy-paste template.

When using Docker Compose (or any tool that interpolates `$…` in env files), escape each `$` in bcrypt hashes as `$$` so the hash is not truncated or altered.

</details>

<details><summary><strong>User Guide</strong></summary>

# User Guide

## Requirements

Linux- or macos-like systems with `go` or `wget & tar` installed.

## Getting Started

Start the latest repo version directly without leaving stuff in the current working dir:

```sh
go run github.com/CoreUnit-NET/caddy-forward-auth@latest
```

## Quick help

```sh
go run github.com/CoreUnit-NET/caddy-forward-auth@latest -h
```

## Install via go

###### _For this section go is required, check out the [install go guide](#install-go)._

```sh
go install github.com/CoreUnit-NET/caddy-forward-auth@latest
```

## Install via wget

```sh
export CUSTOM_BIN_DIR="/usr/local/bin" # <- change if needed
export CUSTOM_VERSION="" # <- set latest version here

rm -rf $CUSTOM_BIN_DIR/caddy-forward-auth
wget https://github.com/CoreUnit-NET/caddy-forward-auth/releases/download/v$CUSTOM_VERSION/caddy-forward-auth-v$CUSTOM_VERSION-linux-amd64.tar.gz -O /tmp/caddy-forward-auth.tar.gz
tar -xzvf /tmp/caddy-forward-auth.tar.gz -C $CUSTOM_BIN_DIR/ caddy-forward-auth
rm /tmp/caddy-forward-auth.tar.gz
```

# Build

## Build requirements

To build, you need to install go.
The required go version is in the `go.mod` file.

## Build Instructions

###### _For this section go is required, check out the [install go guide](#install-go)._

Clone the repo:

```sh
git clone https://github.com/CoreUnit-NET/caddy-forward-auth.git
cd caddy-forward-auth
```

Build the caddy-forward-auth binary from source code:

```sh
make build
./caddy-forward-auth
```

</details>

<details><summary><strong>Development</strong></summary>

# Development

###### _For this section go is required, check out the [install go guide](#install-go)._

This part is work in progress, I want to use 'AIR' as auto-reload tool:

```sh
make dev #WIP
```

## Install go

The required go version for this project is in the `go.mod` file.

To install and update go, I can recommend the following repo:

```sh
git clone git@github.com:udhos/update-golang.git golang-updater
cd golang-updater
sudo ./update-golang.sh
```

</details>

<div align="center">

# 🤝 Contributing

Contributions to this project are welcome!  
Follow the [CONTRIBUTING.md](CONTRIBUTING.md) for more infos.

# ⚠️ Disclaimer

This project is provided without warranties.

# 📜 License

Licensed under the [MIT license](LICENSE).

<a href="https://discord.coreunit.net">
    <img alt="CoreUnit.NET Discord Banner" src="https://discord.com/api/guilds/422136748294930443/widget.png?style=banner2">
</a>

</div>