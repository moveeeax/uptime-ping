# uptime-ping

> A tiny self-hosted status page and monitor — one binary, no dependencies.

**Status:** 🚧 In development

## Overview

A self-hosted status page and uptime pinger shipped as a single Go binary.

## Features

- Poll HTTP/TCP/ICMP targets on a per-check interval
- Serve a clean status page with per-service uptime and latency history
- Config via a single YAML file
- Alert webhook (Slack/Discord/generic) on state change
- Single static binary with an embedded UI

## Stack

Go 1.22, `net/http`, embedded static assets, SQLite/in-memory history.

## Usage

```bash
uptime-ping --config checks.yaml
# then open http://localhost:8080
```

## License

MIT
