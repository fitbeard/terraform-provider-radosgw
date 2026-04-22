#!/usr/bin/env bash
set -euo pipefail

# Create RGW user for testing

CEPH_DIR="${CEPH_DIR:-/tmp/ceph-dev}"
USER_ID="${1:-testuser}"
DISPLAY_NAME="${2:-Test User}"

if [[ ! -f "${CEPH_DIR}/ceph.conf" ]]; then
    echo "ERROR: Cluster not found at ${CEPH_DIR}"
    exit 1
fi

detected_version=""
if [[ -n "${CEPH_VERSION:-}" ]]; then
    detected_version="${CEPH_VERSION}"
elif command -v radosgw-admin &>/dev/null; then
    detected_version=$(radosgw-admin --version 2>/dev/null | grep -oP 'ceph version \K[0-9]+' || echo "0")
fi

normalized_version=0
case "$(echo "${detected_version:-0}" | tr '[:upper:]' '[:lower:]')" in
    reef)     normalized_version=18 ;;
    squid)    normalized_version=19 ;;
    tentacle) normalized_version=20 ;;
    *)        normalized_version=$(echo "${detected_version}" | grep -oP '^[0-9]+' || echo "0") ;;
esac

# accounts=* is only supported on Squid (19+) and later
CAPS="buckets=*;metadata=*;oidc-provider=*;roles=*;users=*"
if [[ "${normalized_version}" -ge 19 ]]; then
    CAPS="accounts=*;${CAPS}"
fi

echo "Creating RGW user: ${USER_ID}"

radosgw-admin --conf "${CEPH_DIR}/ceph.conf" user create \
    --uid="${USER_ID}" \
    --display-name="${DISPLAY_NAME}" \
    --access-key="${USER_ID}" \
    --secret-key="secretkey" \
    --caps="${CAPS}"

echo ""
echo "User created successfully!"
echo ""
echo "To list all users:"
echo "  radosgw-admin --conf ${CEPH_DIR}/ceph.conf user list"
echo ""
echo "To get user info:"
echo "  radosgw-admin --conf ${CEPH_DIR}/ceph.conf user info --uid=${USER_ID}"
