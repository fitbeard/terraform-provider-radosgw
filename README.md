# Terraform Provider for Ceph RADOS Gateway (RGW)

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

This provider enables [Terraform](https://www.terraform.io) and [OpenTofu](https://opentofu.org) to manage resources in [Ceph RADOS Gateway (RGW)](https://docs.ceph.com/en/latest/radosgw/).

## Documentation

- [registry.terraform.io](https://registry.terraform.io/providers/fitbeard/radosgw/latest/docs)
- [search.opentofu.org](https://search.opentofu.org/provider/fitbeard/radosgw/latest)

## Supported Ceph Versions

This provider officially supports the following Ceph releases:

| Release | Version | Status |
|---------|---------|--------|
| Tentacle | 20.x | Full support |
| Squid | 19.x | Full support |
| Reef | 18.x | Supported (some features limited) |

Older versions may work but are not officially tested.

## Development

### Requirements

- [Terraform](https://www.terraform.io/downloads.html) or [OpenTofu](https://opentofu.org/docs/intro/install) 1.x
- [Go](https://golang.org/doc/install) 1.25
- Access to a Ceph cluster with RGW

### Building

```bash
make build      # Build the provider
make install    # Install to local Terraform plugins directory
make fmt        # Format code
make lint       # Run linter
make test       # Run unit tests
make testacc    # Run acceptance tests (requires running RGW)
make docs       # Generate documentation
```

### Development Environment

This project includes a [VS Code Dev Container](https://code.visualstudio.com/docs/devcontainers/containers) with all dependencies pre-installed and a complete Ceph development cluster for testing.

To get started:

1. Open the project in VS Code
2. Click "Reopen in Container" when prompted (or use Command Palette: "Dev Containers: Reopen in Container")
3. Bootstrap the local Ceph cluster:

```bash
# Bootstrap a local Ceph cluster
./scripts/bootstrap-ceph.sh

# Check cluster status
./scripts/status-ceph.sh

# Create a test user
./scripts/create-rgw-user.sh testuser "Test User"

# Run acceptance tests
make testacc
```

See [scripts/README.md](scripts/README.md) for detailed documentation on the development environment.

### Running Acceptance Tests

Acceptance tests create real resources and require a running RGW instance:

```bash
export RADOSGW_ENDPOINT=http://localhost:7480
export RADOSGW_ACCESS_KEY=your-access-key
export RADOSGW_SECRET_KEY=your-secret-key

# Run all acceptance tests
make testacc

# Run with specific Ceph version for compatibility checks
CEPH_VERSION=squid make testacc

# Run a specific test
go test -v -run TestAccRadosgwIAMUser_basic ./provider/...
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
