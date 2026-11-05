# uptime-ping

[![ci](https://github.com/moveeeax/uptime-ping/actions/workflows/ci.yml/badge.svg)](https://github.com/moveeeax/uptime-ping/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.26%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

A tiny self-hosted status page and uptime monitor — **one static Go binary, no dependencies**.

Point it at a YAML file listing your HTTP and TCP services, and it polls them on
their own schedules, keeps a rolling uptime/latency history, serves a live status
page with an embedded UI, exposes Prometheus metrics, and posts a webhook whenever
a service flips up or down.

## Features

- **HTTP + TCP checks** on a per-service interval, timeout and retry budget.
- **Debounced state** via consecutive `failure_threshold` / `success_threshold`
  counts, so a single blip does not page you.
- **Live status page** with per-service state, uptime %, latency and a sparkline —
  served from an embedded asset, so the binary is fully self-contained.
- **JSON API** at `/api/status` and **Prometheus metrics** at `/metrics`.
- **Webhook alerts** on state change, compatible with Slack, Discord or any JSON receiver.
- **One-shot mode** (`--once`) that prints a report and exits non-zero if anything is
  down — handy for cron and CI gates.

## Install

```sh
go install github.com/moveeeax/uptime-ping@latest
# or build from source
git clone https://github.com/moveeeax/uptime-ping && cd uptime-ping && make build
```

## Usage

Write a config (see [`examples/checks.yaml`](examples/checks.yaml)):

```yaml
listen: ":8080"
history: 100
webhook:
  url: ""            # optional Slack/Discord/generic webhook
targets:
  - name: example-site
    type: http
    url: https://example.com
    interval: 30s
    timeout: 5s
    retries: 2
    expect_status: 200
    failure_threshold: 2
    success_threshold: 1
  - name: local-postgres
    type: tcp
    address: 127.0.0.1:5432
    interval: 15s
    timeout: 3s
```

Run the server and open the status page:

```sh
uptime-ping --config checks.yaml
# then open http://localhost:8080
```

Run every check once and gate on the result (exit code `1` if any service is down):

```sh
$ uptime-ping --config checks.yaml --once
SERVICE         STATE  LATENCY  DETAIL
example-site    up     84ms     ok
local-postgres  down   1ms      dial tcp 127.0.0.1:5432: connect: connection refused
$ echo $?
1
```

### Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to the YAML config (default `checks.yaml`). |
| `--listen` | Override the listen address, e.g. `:9000`. |
| `--once` | Probe every target once, print a report, exit non-zero if any is down. |
| `--validate` | Parse and validate the config, then exit. |
| `--version` | Print the version and exit. |

### Endpoints

| Path | Description |
|------|-------------|
| `/` | Embedded status page (auto-refreshes every 5s). |
| `/api/status` | JSON snapshot of every service. |
| `/metrics` | Prometheus exposition (`uptime_ping_up`, `_latency_ms`, `_uptime_ratio`). |
| `/healthz` | Liveness probe. |

## How it works

All real network I/O sits behind a small `Prober` interface, so the scheduling,
threshold and reporting logic is unit-tested without touching the network. Each
target runs on its own ticker; a probe cycle consumes the retry budget before it
counts as a failure, and a debounced state machine only fires the webhook on an
actual up/down transition.

## Development

```sh
make            # fmtcheck + vet + test + build
make race       # go test -race ./...
make smoke      # build, then validate the example config
```

## License

[MIT](LICENSE)

---

**Archived on 2026-11-05.** HTTP/TCP/DNS/ICMP/SSH checks, condition expressions, many alert providers, status page, badges, maintenance windows and persistent storage. Uptime Kuma covers the UI-driven case. uptime-ping's only edge is size and no dependencies, which is not enough of an angle to justify building any of the features below. If it is kept as a minimal or reference tool, build the first two features only.
