#!/bin/bash
set -euo pipefail

# Create RGW user for testing

CEPH_DIR="${CEPH_DIR:-/tmp/ceph-dev}"
USER_ID="${1:-testuser}"
DISPLAY_NAME="${2:-Test User}"

if [ ! -f "$CEPH_DIR/ceph.conf" ]; then
    echo "ERROR: Cluster not found at $CEPH_DIR"
    exit 1
fi

echo "Creating RGW user: $USER_ID"

radosgw-admin --conf "$CEPH_DIR/ceph.conf" user create \
    --uid="$USER_ID" \
    --display-name="$DISPLAY_NAME" \
    --access-key="$USER_ID" \
    --secret-key="secretkey" \
    --caps="buckets=*;metadata=*;oidc-provider=*;roles=*;users=*"

echo ""
echo "User created successfully!"
echo ""
echo "To list all users:"
echo "  radosgw-admin --conf $CEPH_DIR/ceph.conf user list"
echo ""
echo "To get user info:"
echo "  radosgw-admin --conf $CEPH_DIR/ceph.conf user info --uid=$USER_ID"
