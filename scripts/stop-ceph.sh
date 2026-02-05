#!/bin/bash
set -euo pipefail

# Stop Ceph cluster script

CEPH_DIR="${CEPH_DIR:-/tmp/ceph-dev}"

echo "=== Stopping Ceph Development Cluster ==="

if [ ! -d "$CEPH_DIR" ]; then
    echo "Cluster directory not found: $CEPH_DIR"
    exit 1
fi

# Stop RGW
if [ -f "$CEPH_DIR/rgw.pid" ]; then
    RGW_PID=$(cat "$CEPH_DIR/rgw.pid")
    if kill -0 "$RGW_PID" 2>/dev/null; then
        echo "Stopping RADOS Gateway (PID: $RGW_PID)..."
        kill -9 "$RGW_PID"
        wait "$RGW_PID" 2>/dev/null || true
    fi
    rm -f "$CEPH_DIR/rgw.pid"
    # Kill what's left just in case
    killall -9 radosgw || true
fi

# Stop Manager
if [ -f "$CEPH_DIR/mgr.pid" ]; then
    MGR_PID=$(cat "$CEPH_DIR/mgr.pid")
    if kill -0 "$MGR_PID" 2>/dev/null; then
        echo "Stopping Manager (PID: $MGR_PID)..."
        kill "$MGR_PID"
        wait "$MGR_PID" 2>/dev/null || true
    fi
    rm -f "$CEPH_DIR/mgr.pid"
fi

# Stop OSDs
if [ -f "$CEPH_DIR/osd.pids" ]; then
    while read -r OSD_PID; do
        if kill -0 "$OSD_PID" 2>/dev/null; then
            echo "Stopping OSD (PID: $OSD_PID)..."
            kill "$OSD_PID"
            wait "$OSD_PID" 2>/dev/null || true
        fi
    done < "$CEPH_DIR/osd.pids"
    rm -f "$CEPH_DIR/osd.pids"
fi

# Stop Monitor
if [ -f "$CEPH_DIR/mon.pid" ]; then
    MON_PID=$(cat "$CEPH_DIR/mon.pid")
    if kill -0 "$MON_PID" 2>/dev/null; then
        echo "Stopping Monitor (PID: $MON_PID)..."
        kill "$MON_PID"
        wait "$MON_PID" 2>/dev/null || true
    fi
    rm -f "$CEPH_DIR/mon.pid"
fi

# Clean up any remaining processes
echo "Cleaning up remaining processes..."
pkill -f "ceph-mon --conf $CEPH_DIR" || true
pkill -f "ceph-mgr --conf $CEPH_DIR" || true
pkill -f "ceph-osd --conf $CEPH_DIR" || true
pkill -f "radosgw --conf $CEPH_DIR" || true

sleep 2

echo "Ceph cluster stopped."
echo ""
echo "To remove cluster data:"
echo "  rm -rf $CEPH_DIR"
