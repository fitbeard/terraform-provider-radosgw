#!/bin/bash
set -euo pipefail

# Ceph cluster bootstrap script for development
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CEPH_DIR="${CEPH_DIR:-/tmp/ceph-dev}"
FSID="6bb5784d-86b1-4b48-aff7-04d5dd22ef07"
NUM_OSDS=5
MON_HOST="127.0.0.1:6789"
RGW_PORT=7480
DASHBOARD_PORT=8080

echo "=== Ceph Development Cluster Bootstrap ==="
echo "Cluster directory: $CEPH_DIR"

# Create directory structure
echo "Creating directory structure..."
mkdir -p "$CEPH_DIR"/{mon,mgr/ceph-mgr1,rgw/ceph-rgw1,run,crash}
for i in $(seq 0 $((NUM_OSDS - 1))); do
    mkdir -p "$CEPH_DIR/osd/ceph-$i"
done

# Generate ceph.conf
echo "Generating ceph.conf..."
cat > "$CEPH_DIR/ceph.conf" <<EOF
[global]
admin_socket = $CEPH_DIR/\$name.\$pid.asok
auth_allow_insecure_global_id_reclaim = false
auth_client_required = cephx
auth_cluster_required = cephx
auth_service_required = cephx
crash_dir = $CEPH_DIR/crash
debug_ms = 0
exporter_sock_dir = $CEPH_DIR/run
fsid = $FSID
immutable_object_cache_sock = $CEPH_DIR/run/immutable_object_cache.sock
keyring = $CEPH_DIR/keyring
log_to_file = false
log_to_stderr = true
mon_allow_pool_size_one = true
mon_host = [v2:127.0.0.1:3300/0,v1:127.0.0.1:6789/0]
mon_osd_backfillfull_ratio = .99
mon_osd_full_ratio = .99
mon_osd_nearfull_ratio = .99
osd_crush_chooseleaf_type = 0
osd_failsafe_full_ratio = .99
osd_pool_default_min_size = 1
osd_pool_default_size = 1
pid_file = $CEPH_DIR/\$type.\$id.pid
public_network = 127.0.0.1/32
run_dir = $CEPH_DIR/run

[mon]
debug_mon = 0
mgr_initial_modules = dashboard
mon_allow_pool_delete = true
mon_cluster_log_to_file = false
mon_cluster_log_to_stderr = true
mon_data = $CEPH_DIR/mon/ceph-\$id
mon_data_avail_crit = 1
mon_data_avail_warn = 2
mon_initial_members = mon1
mon_max_pg_per_osd = 1000
ms_bind_msgr2 = true
ms_bind_port_min = 3300
ms_bind_port_max = 6800

[mgr]
debug_mgr = 0
mgr_data = $CEPH_DIR/mgr/ceph-\$id

[osd]
debug_osd = 0
osd_data = $CEPH_DIR/osd/ceph-\$id
osd_fast_shutdown = false
osd_objectstore = memstore
osd_scrub_load_threshold = 2000

[client.rgw.rgw1]
debug_rgw = 0
rgw_data = $CEPH_DIR/rgw/ceph-rgw1
rgw_frontends = beast port=$RGW_PORT
rgw_s3_auth_use_sts = true
rgw_sts_key = 6c524a7c95760b945426019b19988846
rgw_verify_ssl = false
EOF

# Generate keyring
echo "Generating keyring..."
cat > "$CEPH_DIR/keyring" <<EOF
[mon.]
key = AQBDm89oNP7bAxAA6TgZ1toOkhDjUNEkRL18Gg==
caps mon = allow *

[client.admin]
key = AQB5m89objcKIxAAda2ULz/l3NH+mv9XzKePHQ==
caps mon = allow *
caps mds = allow *
caps osd = allow *
caps mgr = allow *

[mgr.mgr1]
key = AQCDm89oNP7bAxAA6TgZ1toOkhDjUNEkRL18Gg==
caps mon = allow *
caps osd = allow *
caps mds = allow *

[client.rgw.rgw1]
key = AQDRm89oNP7bAxAA6TgZ1toOkhDjUNEkRL18Gg==
caps mon = allow rw
caps osd = allow rwx
caps mgr = allow rw
EOF

# Add OSD keys
for i in $(seq 0 $((NUM_OSDS - 1))); do
    cat >> "$CEPH_DIR/keyring" <<EOF

[osd.$i]
key = AQCzsPFolNPNNhAAkglWKcr2qZB4lCK/u9A1Zw==
caps mon = allow profile osd
caps mgr = allow profile osd
caps osd = allow *
EOF
done

# Create monitor map
echo "Creating monitor map..."
monmaptool --conf "$CEPH_DIR/ceph.conf" \
    "$CEPH_DIR/monmap" \
    --create \
    --fsid "$FSID"

monmaptool --conf "$CEPH_DIR/ceph.conf" \
    "$CEPH_DIR/monmap" \
    --addv mon1 "[v2:127.0.0.1:3300/0,v1:127.0.0.1:6789/0]" \
    --enable-all-features

# Initialize monitor filesystem
echo "Initializing monitor filesystem..."
ceph-mon --conf "$CEPH_DIR/ceph.conf" \
    --mkfs \
    --id mon1 \
    --monmap "$CEPH_DIR/monmap" \
    --keyring "$CEPH_DIR/keyring"

rm "$CEPH_DIR/monmap"

# Start monitor
echo "Starting monitor..."
nohup ceph-mon --conf "$CEPH_DIR/ceph.conf" \
    --id mon1 \
    --foreground \
    > "$CEPH_DIR/mon.log" 2>&1 &
MON_PID=$!
echo $MON_PID > "$CEPH_DIR/mon.pid"

# Wait for monitor
echo "Waiting for monitor to be ready..."
for i in {1..30}; do
    if ceph --conf "$CEPH_DIR/ceph.conf" status &>/dev/null; then
        echo "Monitor is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "ERROR: Monitor failed to start"
        exit 1
    fi
    sleep 1
done

# Initialize and start OSDs
echo "Initializing OSDs..."
for i in $(seq 0 $((NUM_OSDS - 1))); do
    echo "  Creating OSD $i..."
    ceph-osd --conf "$CEPH_DIR/ceph.conf" \
        --id "$i" \
        --mkfs
    
    nohup ceph-osd --conf "$CEPH_DIR/ceph.conf" \
        --id "$i" \
        --foreground \
        > "$CEPH_DIR/osd.$i.log" 2>&1 &
    echo $! >> "$CEPH_DIR/osd.pids"
done

# Wait for OSDs
echo "Waiting for OSDs to be ready..."
for i in {1..30}; do
    NUM_UP=$(ceph --conf "$CEPH_DIR/ceph.conf" osd stat -f json 2>/dev/null | grep -o '"num_up_osds":[0-9]*' | cut -d: -f2 || echo 0)
    if [ "$NUM_UP" -ge "$NUM_OSDS" ]; then
        echo "All OSDs are ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "ERROR: OSDs failed to start properly"
        exit 1
    fi
    sleep 1
done

# Configure erasure code profile
echo "Configuring erasure code profile..."
ceph --conf "$CEPH_DIR/ceph.conf" osd erasure-code-profile set default \
    k=2 m=1 crush-failure-domain=osd --force --yes-i-really-mean-it

# Configure device classes
echo "Configuring device classes..."
LAST_OSD=$((NUM_OSDS - 1))
ceph --conf "$CEPH_DIR/ceph.conf" osd crush rm-device-class "osd.$LAST_OSD"
ceph --conf "$CEPH_DIR/ceph.conf" osd crush set-device-class hdd "osd.$LAST_OSD"

# Start manager
echo "Starting manager..."
nohup ceph-mgr --conf "$CEPH_DIR/ceph.conf" \
    --id mgr1 \
    --foreground \
    > "$CEPH_DIR/mgr.log" 2>&1 &
MGR_PID=$!
echo $MGR_PID > "$CEPH_DIR/mgr.pid"

# Wait for manager
echo "Waiting for manager to be ready..."
for i in {1..30}; do
    if ceph --conf "$CEPH_DIR/ceph.conf" mgr stat &>/dev/null; then
        echo "Manager is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "ERROR: Manager failed to start"
        exit 1
    fi
    sleep 1
done

# Start RGW
echo "Starting RADOS Gateway..."
nohup radosgw --conf "$CEPH_DIR/ceph.conf" \
    --id rgw.rgw1 \
    --foreground \
    > "$CEPH_DIR/rgw.log" 2>&1 &
RGW_PID=$!
echo $RGW_PID > "$CEPH_DIR/rgw.pid"

# Wait for RGW
echo "Waiting for RADOS Gateway to be ready..."
for i in {1..30}; do
    if curl -sf http://127.0.0.1:$RGW_PORT/ &>/dev/null; then
        echo "RADOS Gateway is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "ERROR: RADOS Gateway failed to start"
        exit 1
    fi
    sleep 1
done

# Configure dashboard
echo "Configuring dashboard..."
ceph --conf "$CEPH_DIR/ceph.conf" config set mgr mgr/dashboard/ssl false
echo "password" | ceph --conf "$CEPH_DIR/ceph.conf" dashboard ac-user-create admin -i /dev/stdin administrator

# Wait for dashboard
echo "Waiting for dashboard to be ready..."
for i in {1..30}; do
    DASHBOARD_URL=$(ceph --conf "$CEPH_DIR/ceph.conf" mgr services -f json 2>/dev/null | grep -o '"dashboard":"[^"]*"' | cut -d'"' -f4 || echo "")
    if [ -n "$DASHBOARD_URL" ]; then
        echo "Dashboard is ready at: $DASHBOARD_URL"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "WARNING: Dashboard may not be ready"
    fi
    sleep 1
done

echo ""
echo "=== Ceph Cluster Bootstrap Complete ==="
echo "Cluster directory: $CEPH_DIR"
echo "Configuration: $CEPH_DIR/ceph.conf"
echo "Dashboard URL: http://127.0.0.1:$DASHBOARD_PORT/ (admin/password)"
echo "RGW URL: http://127.0.0.1:$RGW_PORT/"
echo ""
echo "Service logs:"
echo "  Monitor:  $CEPH_DIR/mon.log"
echo "  Manager:  $CEPH_DIR/mgr.log"
echo "  OSD:      $CEPH_DIR/osd.*.log"
echo "  RGW:      $CEPH_DIR/rgw.log"
echo ""
echo "To check cluster status:"
echo "  ceph --conf $CEPH_DIR/ceph.conf status"
echo ""
echo "To stop the cluster:"
echo "  $SCRIPT_DIR/stop-ceph.sh"
echo ""

# Display initial status
echo "=== Cluster Status ==="
ceph --conf "$CEPH_DIR/ceph.conf" status
