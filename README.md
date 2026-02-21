# vyos-api

A Go REST API proxy for one or more VyOS router devices. Translates simple CRUD HTTP calls into VyOS configuration operations, so callers never need to speak the VyOS HTTP API directly or manage API keys themselves.

## Stack

| Component | Choice |
|-----------|--------|
| Language | Go 1.24 |
| Router | gorilla/mux |
| VyOS client | Self-contained (no external VyOS SDK); see `vyos/` |
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
│   ├── addressgroups.go      # /devices/{id}/firewall/address-groups CRUD
│   └── nat.go                # /devices/{id}/nat/{source|destination}/rules CRUD
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
| `GET` | `/devices/{device_id}/firewall/policies` | List all IPv4 named policies and configured base chains (`forward`, `input`, `output`) |
| `POST` | `/devices/{device_id}/firewall/policies` | Create a policy |
| `GET` | `/devices/{device_id}/firewall/policies/{policy}` | Get a policy including all its rules |
| `PUT` | `/devices/{device_id}/firewall/policies/{policy}` | Update `default_action` and/or `description` |
| `DELETE` | `/devices/{device_id}/firewall/policies/{policy}` | Delete a policy and all its rules |
| `PUT` | `/devices/{device_id}/firewall/policies/{policy}/disable` | Set the VyOS `disable` flag on a policy (rules retained but skipped) |
| `PUT` | `/devices/{device_id}/firewall/policies/{policy}/enable` | Remove the `disable` flag from a policy |
| `POST` | `/devices/{device_id}/firewall/policies/{policy}/rules` | Add a rule to a policy |
| `DELETE` | `/devices/{device_id}/firewall/policies/{policy}/rules/{rule_id}` | Delete a rule |
| `PUT` | `/devices/{device_id}/firewall/policies/{policy}/rules/{rule_id}/disable` | Set the VyOS `disable` flag on a rule |
| `PUT` | `/devices/{device_id}/firewall/policies/{policy}/rules/{rule_id}/enable` | Remove the `disable` flag from a rule |

### Firewall address groups

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/devices/{device_id}/firewall/address-groups` | List all address groups |
| `POST` | `/devices/{device_id}/firewall/address-groups` | Create an address group |
| `GET` | `/devices/{device_id}/firewall/address-groups/{group}` | Get an address group |
| `PUT` | `/devices/{device_id}/firewall/address-groups/{group}` | Full replacement of the address list |
| `DELETE` | `/devices/{device_id}/firewall/address-groups/{group}` | Delete an address group |

### NAT rules

`{nat_type}` is either `source` (SNAT / masquerade) or `destination` (DNAT / port-forward).

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/devices/{device_id}/nat/{nat_type}/rules` | List all NAT rules of the given type. Returns `[]` when NAT is not yet configured. |
| `POST` | `/devices/{device_id}/nat/{nat_type}/rules` | Create a NAT rule (`translation_address` required) |
| `GET` | `/devices/{device_id}/nat/{nat_type}/rules/{rule_id}` | Get a single NAT rule |
| `PUT` | `/devices/{device_id}/nat/{nat_type}/rules/{rule_id}` | Update a NAT rule (partial — omit unchanged fields) |
| `DELETE` | `/devices/{device_id}/nat/{nat_type}/rules/{rule_id}` | Delete a NAT rule |

#### Source NAT fields

| Field | Required | Description |
|-------|----------|-------------|
| `rule_id` | Yes (create) | Rule number (positive integer, multiples of 10 by convention) |
| `translation_address` | Yes | Translated source address or `masquerade` for dynamic SNAT |
| `outbound_interface` | No | Outbound interface name (e.g. `eth0`) |
| `source_address` | No | Match source IP/CIDR |
| `source_port` | No | Match source port or range (e.g. `1024-65535`) |
| `destination_address` | No | Match destination IP/CIDR |
| `destination_port` | No | Match destination port |
| `translation_port` | No | Translated port |
| `protocol` | No | `tcp`, `udp`, `tcp_udp`, `icmp`, or `all` |
| `description` | No | Label (no spaces) |

#### Destination NAT fields

Same fields as source NAT, with `inbound_interface` instead of `outbound_interface`.

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
- **Address groups in rules**: Use `source_group` / `destination_group` instead of `source` / `destination` to match by address-group name. The two are mutually exclusive per direction.
- **Disabling**: Policies and individual rules can be disabled without deletion using the `/disable` and `/enable` sub-resource endpoints. The `disabled` boolean field is reflected in GET responses for both `PolicyInfo` and `RuleInfo`.
- **NAT not configured**: If no NAT rules of a given type exist on the device, VyOS returns HTTP 400 for the config path. The list endpoint silently converts this to an empty array `[]` rather than an error.
- **SNAT masquerade**: Set `translation_address` to the literal string `masquerade` to use VyOS masquerade (dynamic source NAT). Any non-masquerade value is treated as a fixed IP/CIDR.
- **NAT rule_id**: Like firewall rules, VyOS convention is multiples of 10 (`10`, `20`, …). Rules are evaluated in ascending order.

# Unit tests (with Docker if go not installed)
docker run --rm -v "$(pwd):/app" -w /app golang:1.24-alpine go test ./...

# Build and run
docker compose build && docker compose up -d

# Hit endpoints (optional; needs VYOS_HOSTS in .env for device-scoped routes)
./test_endpoints.sh
