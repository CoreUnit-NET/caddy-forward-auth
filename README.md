# intern-auth-gateway

![CI/CD](https://github.com/noblemajo/intern-auth-gateway/actions/workflows/go-bin-release.yml/badge.svg)
![CI/CD](https://github.com/noblemajo/intern-auth-gateway/actions/workflows/go-test-build.yml/badge.svg)  
![MIT](https://img.shields.io/badge/license-MIT-blue.svg)
![](https://img.shields.io/badge/dynamic/json?color=green&label=watchers&query=watchers&suffix=x&url=https%3A%2F%2Fapi.github.com%2Frepos%2Fnoblemajo%2Fintern-auth-gateway)
![](https://img.shields.io/badge/dynamic/json?color=yellow&label=stars&query=stargazers_count&suffix=x&url=https%3A%2F%2Fapi.github.com%2Frepos%2Fnoblemajo%2Fintern-auth-gateway)
![](https://img.shields.io/badge/dynamic/json?color=navy&label=forks&query=forks&suffix=x&url=https%3A%2F%2Fapi.github.com%2Frepos%2Fnoblemajo%2Fintern-auth-gateway)

intern-auth-gateway is a small HTTP auth service for Caddy `forward_auth`.
It verifies HTTP Basic credentials against per-service bcrypt hashes and only then lets Caddy allow access to the protected upstream hosts.

# Table of Contents

- [Features](#features)
- [Out of scope](#out-of-scope)
- [Configuration](#configuration)
  - [Flags and environment](#flags-and-environment)
  - [Services](#services)
- [Getting Started](#getting-started)
  - [Requirements](#requirements)
  - [Use via Go](#use-via-go)
  - [Install via go](#install-via-go)
  - [Install via wget](#install-via-wget)
- [Build requirements](#build-requirements)
- [Build Instructions](#build-instructions)
- [Install go](#install-go)

## Features

- **Caddy `forward_auth` endpoint**: Exposes `/` and `/auth` for Caddy `forward_auth` probes. Successful checks return `200` and set `Remote-User`; failures return `401` with a Basic challenge (or `403` for blocked origins).
- **HTTP Basic authentication**: Credentials are verified against bcrypt password hashes configured per service.
- **Per-service auth routing**: Each `SERVICE_*` entry maps a host pattern to its own username and password hash, so different upstream hosts can require different credentials.
- **Host glob matching**: Service host patterns support single-label wildcards (for example `*.intern.example.com` matches `api.intern.example.com`, but not `a.b.intern.example.com`).
- **Origin allowlist**: Optional `ALLOWED_ORIGINS` CSV restricts browser `Origin` hostnames. Requests without an `Origin` header (typical for Caddy probes) are allowed.
- **Target host resolution**: The protected host is taken from `X-Forwarded-Host` (first value if CSV), falling back to the request `Host`.

## Out of scope

- **TLS / HTTPS**: Terminate TLS in front of this service (for example with Caddy). The gateway itself listens on plain HTTP.
- **Non-Basic auth**: OAuth, OIDC, cookies, API keys, mTLS, and similar methods are not supported.
- **Reverse-proxy duties**: This process only answers auth probes. Upstream proxying, routing, and TLS remain Caddy’s responsibility.
- **Non-Caddy gatekeeping**: The handler is built for Caddy `forward_auth`. Other proxy auth protocols are not a goal.

## Configuration

CLI flags and environment variables can both be used. **Flags override env values.** Boolean flags default to `false`.

Running the binary with no subcommand starts the HTTP server (same as `serve`).

### Flags and environment

| Flag                | Env Var           | Type | Default   | Description |
| ------------------- | ----------------- | ---- | --------- | ----------- |
| `--verbose` / `-b`  | `VERBOSE`         | bool | `false`   | Enable verbose request logging |
| `--host`            | `HOST`            | str  | `0.0.0.0` | Listen address |
| `--port`            | `PORT`            | int  | `8080`    | Listen port |
| `--allowed-origins` | `ALLOWED_ORIGINS` | CSV  | _(empty)_ | Allowed `Origin` hostnames. When set, other origins are rejected with `403`. Empty disables origin enforcement |

Quick help:

```sh
go run github.com/NobleMajo/intern-auth-gateway@latest -h
```

### Services

Services are configured **only through environment variables** with the `SERVICE_` prefix.

| Piece | Rule |
| ----- | ---- |
| Env key | `SERVICE_<name>` (name must be non-empty) |
| Value | `hostGlob/username/passwordHash` |
| Parsing | Split on `/` with at most **two** separators (`SplitN`); the bcrypt hash may contain `/` |
| `hostGlob` | Exact hostname, or `*` for a single DNS label (for example `*.intern.example.com`) |
| `username` | Must be **unique** across all `SERVICE_*` entries (startup fails on duplicates) |
| `passwordHash` | bcrypt hash (for example `$2a$…`) |

Example:

```sh
PORT=8080
HOST=0.0.0.0
ALLOWED_ORIGINS="intern-auth.example.com, localhost, auth-test.example.com"
SERVICE_test="test.example.com/tester/$2a$14$AnhQELX1cqeO3YaLPOTWtOuPsKZgweRHrYLcqzQUcvokbVZmzNWrO"
SERVICE_intern="*.intern.example.com/intern-user/$2a$14$54tdWftb4iOouKyfDyURPuI6rOIwcbjqKYfzOqYE0PyOcmVFnU1mM"
```

## Getting Started

### Requirements

Linux- or macos-like systems with:

- `go` **or** `wget` & `tar` (to run/install the intern-auth-gateway binary)

### Use via Go

Help and configuration details:

```sh
go run github.com/NobleMajo/intern-auth-gateway@latest -h
```

Start with defaults (same as `serve`):

```sh
go run github.com/NobleMajo/intern-auth-gateway@latest
```

### Install via go

###### _For this section go is required, check out the [install go guide](#install-go)._

```sh
go install github.com/NobleMajo/intern-auth-gateway@latest
```

### Install via wget

```sh
export CUSTOM_BIN_DIR="/usr/local/bin" # <- change if needed
export INTERN_AUTH_GATEWAY_VERSION="" # <- set latest version here

rm -rf $CUSTOM_BIN_DIR/intern-auth-gateway
wget https://github.com/NobleMajo/intern-auth-gateway/releases/download/v$INTERN_AUTH_GATEWAY_VERSION/intern-auth-gateway-v$INTERN_AUTH_GATEWAY_VERSION-linux-amd64.tar.gz -O /tmp/intern-auth-gateway.tar.gz
tar -xzvf /tmp/intern-auth-gateway.tar.gz -C $CUSTOM_BIN_DIR/ intern-auth-gateway
rm /tmp/intern-auth-gateway.tar.gz
```

# Build

## Build requirements

To build, you need to install go.
The required go version is in the `go.mod` file.

## Build Instructions

###### _For this section go is required, check out the [install go guide](#install-go)._

Clone the repo:

```sh
git clone https://github.com/NobleMajo/intern-auth-gateway.git
cd intern-auth-gateway
```

Build the intern-auth-gateway binary from source code:

```sh
make build
./intern-auth-gateway
```

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

# Contributing

Contributions to this project are welcome!  
Interested users can refer to the guidelines provided in the [CONTRIBUTING.md](CONTRIBUTING.md) file to contribute to the project and help improve its functionality and features.

# License

This project is licensed under the [MIT license](LICENSE), providing users with flexibility and freedom to use and modify the software according to their needs.

# Disclaimer

This project is provided without warranties.  
Users are advised to review the accompanying license for more information on the terms of use and limitations of liability.