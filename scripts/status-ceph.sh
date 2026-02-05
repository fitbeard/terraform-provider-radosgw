#!/bin/bash
set -euo pipefail

# Check Ceph cluster status

CEPH_DIR="${CEPH_DIR:-/tmp/ceph-dev}"

if [ ! -d "$CEPH_DIR" ]; then
    echo "ERROR: Cluster directory not found: $CEPH_DIR"
    echo "Cluster is not running."
    exit 1
fi

if [ ! -f "$CEPH_DIR/ceph.conf" ]; then
    echo "ERROR: Configuration file not found: $CEPH_DIR/ceph.conf"
    exit 1
fi

echo "=== Ceph Cluster Status ==="
echo ""

# Check if monitor is running
if [ -f "$CEPH_DIR/mon.pid" ]; then
    MON_PID=$(cat "$CEPH_DIR/mon.pid")
    if kill -0 "$MON_PID" 2>/dev/null; then
        echo "Monitor is running (PID: $MON_PID)"
    else
        echo "Monitor is not running (stale PID file)"
    fi
else
    echo "Monitor PID file not found"
fi

# Check if manager is running
if [ -f "$CEPH_DIR/mgr.pid" ]; then
    MGR_PID=$(cat "$CEPH_DIR/mgr.pid")
    if kill -0 "$MGR_PID" 2>/dev/null; then
        echo "Manager is running (PID: $MGR_PID)"
    else
        echo "Manager is not running (stale PID file)"
    fi
else
    echo "Manager PID file not found"
fi

# Check OSDs
if [ -f "$CEPH_DIR/osd.pids" ]; then
    OSD_COUNT=0
    OSD_RUNNING=0
    while read -r OSD_PID; do
        OSD_COUNT=$((OSD_COUNT + 1))
        if kill -0 "$OSD_PID" 2>/dev/null; then
            OSD_RUNNING=$((OSD_RUNNING + 1))
        fi
    done < "$CEPH_DIR/osd.pids"
    echo "OSDs running: $OSD_RUNNING/$OSD_COUNT"
else
    echo "No OSD PID file found"
fi

# Check RGW
if [ -f "$CEPH_DIR/rgw.pid" ]; then
    RGW_PID=$(cat "$CEPH_DIR/rgw.pid")
    if kill -0 "$RGW_PID" 2>/dev/null; then
        echo "RADOS Gateway is running (PID: $RGW_PID)"
    else
        echo "RADOS Gateway is not running (stale PID file)"
    fi
else
    echo "RADOS Gateway PID file not found"
fi

echo ""
echo "=== Detailed Cluster Status ==="
if ceph --conf "$CEPH_DIR/ceph.conf" status 2>/dev/null; then
    echo ""
    echo "=== OSD Tree ==="
    ceph --conf "$CEPH_DIR/ceph.conf" osd tree
    echo ""
    echo "=== Pool List ==="
    ceph --conf "$CEPH_DIR/ceph.conf" osd pool ls detail
else
    echo "ERROR: Unable to query cluster status"
    exit 1
fi

echo ""
echo "=== Service URLs ==="
DASHBOARD_URL=$(ceph --conf "$CEPH_DIR/ceph.conf" mgr services -f json 2>/dev/null | grep -o '"dashboard":"[^"]*"' | cut -d'"' -f4 || echo "N/A")
echo "Dashboard: $DASHBOARD_URL (admin/password)"
echo "RGW: http://127.0.0.1:7480/"
