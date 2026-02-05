# Ceph Development Cluster Scripts

This directory contains scripts for managing a local Ceph development cluster with RADOS Gateway (RGW).

## Scripts

### bootstrap-ceph.sh
Bootstrap a complete Ceph cluster for development and testing.

**Usage:**
```bash
./scripts/bootstrap-ceph.sh
```

**Features:**
- Creates a full Ceph cluster with MON, MGR, OSDs, and RGW
- Configures the cluster for single-node development
- Sets up the Ceph Dashboard (http://127.0.0.1:8080, admin/password)
- Starts RADOS Gateway on port 7480
- Uses in-memory storage (memstore) for speed

**Environment Variables:**
- `CEPH_DIR`: Cluster data directory (default: `/tmp/ceph-dev`)

### stop-ceph.sh
Stop all Ceph daemons gracefully.

**Usage:**
```bash
./scripts/stop-ceph.sh
```

To also remove cluster data:
```bash
./scripts/stop-ceph.sh
rm -rf /tmp/ceph-dev
```

### status-ceph.sh
Check the status of the Ceph cluster.

**Usage:**
```bash
./scripts/status-ceph.sh
```

Shows:
- Running daemons and their PIDs
- Cluster health status
- OSD tree
- Pool list
- Service URLs

### create-rgw-user.sh
Create a RADOS Gateway user for testing.

**Usage:**
```bash
./scripts/create-rgw-user.sh [USER_ID] [DISPLAY_NAME]
```

**Examples:**
```bash
# Create user with default name
./scripts/create-rgw-user.sh myuser

# Create user with custom display name
./scripts/create-rgw-user.sh myuser "My Test User"
```

## Quick Start

1. **Bootstrap the cluster:**
   ```bash
   ./scripts/bootstrap-ceph.sh
   ```

2. **Check cluster status:**
   ```bash
   ./scripts/status-ceph.sh
   ```

3. **Create an RGW user:**
   ```bash
   ./scripts/create-rgw-user.sh testuser "Test User"
   ```

4. **Stop the cluster when done:**
   ```bash
   ./scripts/stop-ceph.sh
   ```

## Using with Terraform Provider

The cluster is configured for use with the terraform-provider-radosgw:

**Dashboard API (for provider testing):**
- Endpoint: `http://127.0.0.1:8080/`
- Username: `admin`
- Password: `password`

**RGW S3 API:**
- Endpoint: `http://127.0.0.1:7480/`

## Troubleshooting

### Check daemon logs
Since the cluster logs to stderr, check the terminal where you ran bootstrap-ceph.sh.

### Manual daemon check
```bash
ps aux | grep ceph
```

### Query cluster directly
```bash
ceph --conf /tmp/ceph-dev/ceph.conf status
ceph --conf /tmp/ceph-dev/ceph.conf health detail
```

### Check RGW admin API
```bash
radosgw-admin --conf /tmp/ceph-dev/ceph.conf user list
```

### Clean restart
```bash
./scripts/stop-ceph.sh
rm -rf /tmp/ceph-dev
./scripts/bootstrap-ceph.sh
```
