# Lightweight Uptime Monitoring

> [!IMPORTANT]
> This project was built with the help of _AI_. It has implemented major parts of the _frontend-design_, aswell as created _skeletons_ for the various `.yaml` configuration files. 

A lightweight, self-hosted, single-binary Uptime Monitoring Platform inspired by Uptime Kuma and Gatus. Configured strictly via declarative YAML files in `./config/` with zero web-based administration endpoints, a dark-mode-only dashboard, deduplicated webhook alerting, scheduled maintenance windows, and a token-secured [Homepage](https://gethomepage.dev) Custom API widget.

---

## Quickstart

### 1. Build Single Executable

```bash
cd web && \
npm install && \
npm run build && \
cd ..
```

```bash
# Compile single binary
go build -o uptime-mon ./cmd/uptime-mon
```

### 2. Validate Configuration

```bash
./uptime-mon --config --validate
```

```text
Validating configuration files in ./config...
✓ Global config: OK
✓ 2 zones defined: OK
✓ 4 endpoints defined: OK
✓ 2 one-off maintenance, 2 scheduled: OK
✓ 2 webhooks configured: OK
Configuration valid. 0 errors found.
```

### 3. Generate API-Token for Homepage Dashboard Widget

```bash
./uptime-mon --api --token --new --name "Homelab Production"
```

This command outputs a ready-to-paste `services.yaml` snippet for [gethomepage.dev](https://gethomepage.dev):

```yaml
- Uptime Monitor:
    icon: uptime-kuma.png
    href: http://localhost:8080/
    description: System Availability
    widget:
      type: customapi
      url: http://localhost:8080/api/v1/status/summary
      refreshInterval: 10000
      headers:
        X-API-Token-Id: <TOKEN_ID>
        X-API-Token-Secret: <TOKEN_SECRET>
      mappings:
        - field: "Hosts Up"
          label: "Up"
          format: number
        - field: "Hosts Down"
          label: "Down"
          format: number
        - field: "Uptime"
          label: "Uptime"
          format: percent
```

### 4. Start Monitoring Daemon

```bash
./uptime-mon serve --port 8080
```

Open your browser to `http://localhost:8080` to access the dashboard.

---

## CLI Reference

```bash
# Start monitoring daemon (default if no flags passed)
uptime-mon serve [--port 8080] [--bind 0.0.0.0] [--config-dir ./config] [--data-dir ./data]

# Generate a new token
uptime-mon --api --token --new [--name <label>]

# List registered tokens
uptime-mon --api --token --list

# Revoke a token immediately
uptime-mon --api --token --revoke <token_id>

# Validate configuration files
uptime-mon --config --validate [--config-dir ./config]
```

---

## Configuration Hierarchy (`./config/`)

### Global Defaults: `./config/config.yml`
```yaml
interval: "60s"
timeout: "5s"
retries: 3
timezone: "Europe/Berlin"
history_retention_days: 30
selfhst_icon_cdn: "https://cdn.jsdelivr.net/gh/selfhst/icons/svg/{icon}.svg"
```

### Zones: `./config/zones/<name>.yml`
```yaml
id: "networking"
display_name: "Networking & DNS"
description: "Core switches, routers, and firewalls"
order: 1
icon: "opnsense"
```

### Endpoints: `./config/endpoints/<name>.yml`
```yaml
- id: "gateway-ping"
  display_name: "OPNsense Gateway"
  zone: "networking"
  icon: "opnsense"
  type: "ping"
  target: "1.1.1.1"
  interval: "30s"
  timeout: "3s"
  retries: 3
  show_target: false

- id: "dns-tcp"
  display_name: "Cloudflare DNS TCP"
  zone: "networking"
  icon: "cloudflare"
  type: "tcp"
  target: "1.1.1.1"
  port: 53

- id: "adguard-doh"
  display_name: "AdGuard DoH"
  zone: "networking"
  icon: "adguardhome"
  type: "dns"
  hostname: "cloudflare.com"
  server: "https://dns.adguard-dns.com/dns-query"
  protocol: "doh"

- id: "web-portal"
  display_name: "Public Portal"
  zone: "webserver"
  icon: "traefik"
  type: "https"
  target: "https://portal.example.com"
  accepted_status_codes: ["20X", "30X"]
  tls_expiry_warn_days: [14, 7, 3]
  show_target: true
```

### Maintenance: `./config/maintenance/`
- **One-off**: `./config/maintenance/<name>.yml`
  ```yaml
  id: "firmware-update"
  message: "Switch upgrade"
  start_date: "2026-10-25"
  start_time: "02:00"
  end_date: "2026-10-25"
  end_time: "04:00"
  remindme: 2
  affected_zones: ["networking"]
  affected_hosts: ["all"]
  ```
- **Recurring**: `./config/maintenance/scheduled/<name>.yml`
  ```yaml
  id: "sunday-backup"
  message: "Weekly VM snapshot"
  schedule: "0 3 * * 0"
  duration: "2h"
  remindme: 1
  affected_zones: ["webserver"]
  affected_hosts: ["web-portal"]
  ```

---

## Docker Deployment

```bash
docker compose up -d
```

With `docker-compose.yml`:
```yaml
services:
  uptime-mon:
    build: .
    image: uptime-mon:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./config:/app/config:ro
      - ./data:/app/data
    environment:
      - TZ=Europe/Berlin
    cap_add:
      - NET_RAW
```
