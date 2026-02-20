#!/usr/bin/env bash
# Hit each API endpoint and report status. Uses DEVICE_ID=router1 by default.
# Usage: ./test_endpoints.sh [base_url]
set -e
BASE="${1:-http://localhost:8082}"
DEV="${2:-router1}"

test() {
  local method="$1"
  local path="$2"
  local body="$3"
  local want="${4:-2}"  # default expect 2xx
  local code
  code=$(curl -s -o /tmp/body -w "%{http_code}" -X "$method" -H "Content-Type: application/json" ${body:+-d "$body"} "$BASE$path")
  if [[ "$want" == "2" ]]; then
    if [[ "$code" != 2* ]]; then
      echo "FAIL $method $path -> $code (expected 2xx)"
      cat /tmp/body | head -3
      return 1
    fi
  else
    if [[ "$code" != "$want" ]]; then
      echo "FAIL $method $path -> $code (expected $want)"
      cat /tmp/body | head -3
      return 1
    fi
  fi
  echo "OK   $method $path -> $code"
  return 0
}

echo "Testing base: $BASE  device: $DEV"
echo "--- Service ---"
test GET "/health"
test GET "/devices"
echo "--- Device not found ---"
test GET "/devices/nonexistent/networks" "" 404
echo "--- Networks ---"
test GET "/devices/$DEV/networks"
test GET "/devices/$DEV/networks/eth0"
test GET "/devices/$DEV/networks/eth0?type=ethernet"
echo "--- VRFs ---"
test GET "/devices/$DEV/vrfs"
echo "--- VLANs ---"
test GET "/devices/$DEV/vlans"
echo "--- Firewall ---"
test GET "/devices/$DEV/firewall/policies"
test GET "/devices/$DEV/firewall/address-groups"
echo "--- Done ---"
