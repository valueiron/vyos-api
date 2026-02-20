# vyos-api

A Go REST API proxy for one or more VyOS router devices. Translates simple CRUD HTTP calls into VyOS configuration operations, so callers never need to speak the VyOS HTTP API directly or manage API keys themselves.

## Stack

| Component | Choice |
|-----------|--------|
| Language | Go 1.24 |
| Router | gorilla/mux |
| VyOS SDK | github.com/ganawaj/go-vyos v0.1.0 |
| Runtime image | gcr.io/distroless/static-debian12:nonroot |
| Port | 8082 |

## Project structure

```
vyos-api/
├── main.go                   # Entry point, VYOS_HOSTS parsing, router, graceful shutdown
├── handlers/
│   ├── handler.go            # Handler struct, Device type, getClient(), writeJSON(), writeError()
│   ├── health.go             # GET /health
│   ├── devices.go            # GET /devices
│   ├── networks.go           # /devices/{id}/networks CRUD + toStringSlice helper
│   ├── vrfs.go               # /devices/{id}/vrfs CRUD
│   ├── vlans.go              # /devices/{id}/vlans CRUD
│   ├── firewall.go           # /devices/{id}/firewall/policies CRUD + /rules sub-resource
│   └── addressgroups.go      # /devices/{id}/firewall/address-groups CRUD
├── openapi.json              # OpenAPI 3.0 specification
├── go.mod
├── Dockerfile                # Multi-stage: golang:1.24-alpine → distroless/static
├── docker-compose.yml        # Standalone dev compose
└── .env.example              # Documents VYOS_HOSTS and PORT
```

## Configuration

| Variable | Required | Description |
|----------|----------|-------------|
| `PORT` | No | Listen port. Defaults to `8082`. |
| `VYOS_HOSTS` | No | Comma-separated list of devices (see format below). An empty value starts the service with no devices registered. |

### VYOS_HOSTS format

```
name:scheme://host:port:apikey
```

Multiple devices are comma-separated:

```bash
VYOS_HOSTS=router1:https://192.168.1.1:443:key1,router2:https://10.0.0.1:8443:key2
```

The `name` field becomes the `{device_id}` segment in all URL paths. TLS certificate verification is disabled automatically (required for VyOS self-signed certs).

Copy `.env.example` to `.env` and fill in your values before running.

## Running

### Locally (requires Go 1.24)

```bash
cp .env.example .env
# edit .env
source .env && go run .
```

### Docker (standalone)

```bash
docker build -t vyos-api .
docker run --rm -e VYOS_HOSTS="router1:https://192.168.1.1:443:mykey" -p 8082:8082 vyos-api
```

Or with docker compose:

```bash
docker compose up
```

### Portal stack

From the portal root:

```bash
docker compose up vyos-api
```

The `VYOS_HOSTS` variable is read from the shell environment or a root-level `.env` file.

## Endpoints

### Service

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Liveness probe — returns `{"status":"ok"}` |
| `GET` | `/devices` | List registered devices with a live connectivity probe |

### Networks (interfaces)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/devices/{device_id}/networks` | List all interfaces (all types) with their IPv4 addresses |
| `POST` | `/devices/{device_id}/networks` | Set an IPv4 address on an interface |
| `GET` | `/devices/{device_id}/networks/{interface}?type=ethernet` | Get a single interface |
| `PUT` | `/devices/{device_id}/networks/{interface}` | Replace the interface address (body must include `type`) |
| `DELETE` | `/devices/{device_id}/networks/{interface}?type=ethernet` | Delete an interface config node |

### VRFs

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/devices/{device_id}/vrfs` | List all VRFs |
| `POST` | `/devices/{device_id}/vrfs` | Create a VRF |
| `GET` | `/devices/{device_id}/vrfs/{vrf}` | Get a VRF |
| `PUT` | `/devices/{device_id}/vrfs/{vrf}` | Update a VRF (partial — omit unchanged fields) |
| `DELETE` | `/devices/{device_id}/vrfs/{vrf}` | Delete a VRF |

### VLANs (802.1Q vif subinterfaces)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/devices/{device_id}/vlans` | List all vif subinterfaces across all parent interfaces |
| `POST` | `/devices/{device_id}/vlans` | Create a vif subinterface |
| `GET` | `/devices/{device_id}/vlans/{interface}/{vlan_id}?type=ethernet` | Get a vif subinterface |
| `PUT` | `/devices/{device_id}/vlans/{interface}/{vlan_id}` | Update a vif subinterface |
| `DELETE` | `/devices/{device_id}/vlans/{interface}/{vlan_id}?type=ethernet` | Delete a vif subinterface |

### Firewall policies

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/devices/{device_id}/firewall/policies` | List all IPv4 named policies |
| `POST` | `/devices/{device_id}/firewall/policies` | Create a policy |
| `GET` | `/devices/{device_id}/firewall/policies/{policy}` | Get a policy including all its rules |
| `PUT` | `/devices/{device_id}/firewall/policies/{policy}` | Update `default_action` and/or `description` |
| `DELETE` | `/devices/{device_id}/firewall/policies/{policy}` | Delete a policy and all its rules |
| `POST` | `/devices/{device_id}/firewall/policies/{policy}/rules` | Add a rule to a policy |
| `DELETE` | `/devices/{device_id}/firewall/policies/{policy}/rules/{rule_id}` | Delete a rule |

### Firewall address groups

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/devices/{device_id}/firewall/address-groups` | List all address groups |
| `POST` | `/devices/{device_id}/firewall/address-groups` | Create an address group |
| `GET` | `/devices/{device_id}/firewall/address-groups/{group}` | Get an address group |
| `PUT` | `/devices/{device_id}/firewall/address-groups/{group}` | Full replacement of the address list |
| `DELETE` | `/devices/{device_id}/firewall/address-groups/{group}` | Delete an address group |

## Error responses

All errors return JSON with an `error` field:

```json
{ "error": "device not found: router99" }
```

| Status | Meaning |
|--------|---------|
| `400` | Missing or invalid request fields |
| `404` | Device ID not registered, or resource not found on device |
| `422` | Device rejected the operation (invalid config, constraint violation) |
| `502` | Could not reach the device (network error, timeout, TLS failure) |

## Notes

- **Descriptions**: VyOS description paths are split on whitespace, so descriptions must not contain spaces. Use hyphens or underscores (`my-vrf`, `lan_uplink`).
- **VLAN IDs**: VyOS stores 802.1Q subinterfaces under the `vif` key, not `vlan`. The API uses the `vlan_id` field but maps it to `vif` internally.
- **TLS**: All device connections use `InsecureSkipVerify` to accommodate VyOS self-signed certificates.
- **No persistence**: This service is stateless. All state lives on the VyOS device.

# Unit tests (with Docker if go not installed)
docker run --rm -v "$(pwd):/app" -w /app golang:1.24-alpine go test ./...

# Build and run
docker compose build && docker compose up -d

# Hit endpoints (optional; needs VYOS_HOSTS in .env for device-scoped routes)
./test_endpoints.sh
