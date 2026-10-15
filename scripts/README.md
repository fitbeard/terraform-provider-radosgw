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

### compose.yml
A Docker Compose definition for a local OpenID Connect (OIDC) provider used as the
external IdP for RGW STS `AssumeRoleWithWebIdentity` testing. It runs
[simple-oidc-provider](https://github.com/Plainscope/oidc-provider) on
`http://localhost:9080` (issuer `http://localhost:9080`, client `test-client` /
`test-secret`, one directory user `radosgw@terraform` / `test-password`).

**Usage (via Makefile):**
```bash
make oidc-up      # start the provider and wait until healthy
make oidc-token   # mint a JWT (id_token) from it
make oidc-stop    # stop and remove it
```

**Issuer address.** The issuer must be reachable from wherever the tests and
`radosgw` run, and must match the token's `iss` claim. The Makefile auto-detects
it: on a normal host / CI runner it uses `http://localhost:9080`; in a dev
container that uses *docker-outside-of-docker* (the published port is not on the
container's own `localhost`, but the Docker host is reachable as
`host.docker.internal`) it uses `http://host.docker.internal:9080`. Both the
container's advertised `ISSUER` and the client are driven from the same value, so
the flow and later RGW validation agree. Override with `OIDC_ISSUER=...` if needed:

```bash
make oidc-up OIDC_ISSUER=http://host.docker.internal:9080
```

The image ships its own container healthcheck, so `make oidc-wait` polls
`docker inspect` health rather than the published port — which works regardless of
how ports are mapped.

### jwt-token.sh
Mint a JWT (`id_token`) from the OIDC provider above via the Authorization Code +
PKCE flow, printing the token to stdout for use with STS WebIdentity tests.

**Usage:**
```bash
./scripts/jwt-token.sh            # prints the id_token (JWT) to stdout
DEBUG=1 ./scripts/jwt-token.sh    # also dumps the token response + claims to stderr
TOKEN=$(./scripts/jwt-token.sh)   # capture for use in a test
```

**Environment variables** (defaults match `compose.yml`): `OIDC_ISSUER`,
`OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URI`, `OIDC_USERNAME`,
`OIDC_PASSWORD`.

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
