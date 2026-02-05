---
subcategory: "S3 (Simple Storage)"
page_title: "RadosGW: radosgw_s3_bucket_lifecycle_configuration"
description: |-
  Manages lifecycle configuration for an S3 bucket in RadosGW.
  Lifecycle rules allow you to define actions that RadosGW applies to objects during their lifetime. Common use cases include:
  Expiring (deleting) objects after a certain number of daysTransitioning objects to different storage classesCleaning up incomplete multipart uploadsManaging noncurrent versions in versioned buckets
  ~> Note: RadosGW supports a subset of Amazon S3 lifecycle features. Some advanced filtering options (like object size filtering) may not be available. See the Ceph documentation https://docs.ceph.com/en/latest/radosgw/s3/ for details.
  ~> Important: Only one lifecycle configuration can exist per bucket. This resource will replace any existing lifecycle configuration.
---

# radosgw_s3_bucket_lifecycle_configuration

Manages lifecycle configuration for an S3 bucket in RadosGW.

Lifecycle rules allow you to define actions that RadosGW applies to objects during their lifetime. Common use cases include:
- Expiring (deleting) objects after a certain number of days
- Transitioning objects to different storage classes
- Cleaning up incomplete multipart uploads
- Managing noncurrent versions in versioned buckets

~> **Note:** RadosGW supports a subset of Amazon S3 lifecycle features. Some advanced filtering options (like object size filtering) may not be available. See the [Ceph documentation](https://docs.ceph.com/en/latest/radosgw/s3/) for details.

~> **Important:** Only one lifecycle configuration can exist per bucket. This resource will replace any existing lifecycle configuration.

## Example Usage

```terraform
# Basic lifecycle rule - expire objects after 90 days
resource "radosgw_s3_bucket" "example" {
  bucket = "my-lifecycle-bucket"
}

resource "radosgw_s3_bucket_lifecycle_configuration" "expire_old_objects" {
  bucket = radosgw_s3_bucket.example.bucket

  rule {
    id     = "expire-old-objects"
    status = "Enabled"

    expiration {
      days = 90
    }
  }
}

# Lifecycle with prefix filter - only apply to logs/
resource "radosgw_s3_bucket_lifecycle_configuration" "expire_logs" {
  bucket = radosgw_s3_bucket.example.bucket

  rule {
    id     = "expire-logs"
    status = "Enabled"

    filter {
      prefix = "logs/"
    }

    expiration {
      days = 30
    }
  }
}

# Multiple rules - different expiration for different prefixes
resource "radosgw_s3_bucket" "multi_rule" {
  bucket = "multi-rule-bucket"
}

resource "radosgw_s3_bucket_lifecycle_configuration" "multi_rule" {
  bucket = radosgw_s3_bucket.multi_rule.bucket

  rule {
    id     = "expire-temp"
    status = "Enabled"

    filter {
      prefix = "temp/"
    }

    expiration {
      days = 7
    }
  }

  rule {
    id     = "expire-archive"
    status = "Enabled"

    filter {
      prefix = "archive/"
    }

    expiration {
      days = 365
    }
  }
}

# Transition to different storage class
resource "radosgw_s3_bucket" "tiered" {
  bucket = "tiered-storage-bucket"
}

resource "radosgw_s3_bucket_lifecycle_configuration" "tiering" {
  bucket = radosgw_s3_bucket.tiered.bucket

  rule {
    id     = "move-to-cold-storage"
    status = "Enabled"

    transition {
      days          = 30
      storage_class = "COLD"
    }

    expiration {
      days = 365
    }
  }
}

# Versioned bucket - manage noncurrent versions
resource "radosgw_s3_bucket" "versioned" {
  bucket     = "versioned-bucket"
  versioning = "enabled"
}

resource "radosgw_s3_bucket_lifecycle_configuration" "noncurrent_cleanup" {
  bucket = radosgw_s3_bucket.versioned.bucket

  rule {
    id     = "cleanup-old-versions"
    status = "Enabled"

    noncurrent_version_expiration {
      noncurrent_days = 30
    }
  }
}

# Cleanup incomplete multipart uploads
resource "radosgw_s3_bucket_lifecycle_configuration" "abort_multipart" {
  bucket = radosgw_s3_bucket.example.bucket

  rule {
    id     = "abort-incomplete-uploads"
    status = "Enabled"

    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }
  }
}

# Filter by tag
resource "radosgw_s3_bucket_lifecycle_configuration" "tagged_expiration" {
  bucket = radosgw_s3_bucket.example.bucket

  rule {
    id     = "expire-temporary-tagged"
    status = "Enabled"

    filter {
      tag {
        key   = "Environment"
        value = "Development"
      }
    }

    expiration {
      days = 14
    }
  }
}

# Complex filter with AND condition
resource "radosgw_s3_bucket_lifecycle_configuration" "complex_filter" {
  bucket = radosgw_s3_bucket.example.bucket

  rule {
    id     = "complex-rule"
    status = "Enabled"

    filter {
      and {
        prefix = "data/"
        tags = {
          "Project" = "Analytics"
          "Tier"    = "Archive"
        }
      }
    }

    expiration {
      days = 180
    }
  }
}

# Disabled rule (for temporary suspension)
resource "radosgw_s3_bucket_lifecycle_configuration" "disabled_rule" {
  bucket = radosgw_s3_bucket.example.bucket

  rule {
    id     = "disabled-cleanup"
    status = "Disabled"

    expiration {
      days = 30
    }
  }
}
```

<!-- schema generated by tfplugindocs -->

## Argument Reference

The following arguments are supported:


* `bucket` - (Required) The name of the bucket to apply the lifecycle configuration to.


* `rule` - (Optional) A lifecycle rule for the bucket. At least one rule is required. (see [below for nested schema](#nestedblock--rule))




## Attributes Reference

The following attributes are exported:

* `id` - The resource identifier (bucket name).
* `bucket` - See Argument Reference above.
* `rule` - See Argument Reference above.

<a id="nestedblock--rule"></a>
### Nested Schema for `rule`

Required:

- `id` (String) Unique identifier for the rule. Maximum 255 characters.
- `status` (String) Whether the rule is currently being applied. Valid values: `Enabled`, `Disabled`.



- `abort_incomplete_multipart_upload` (Block List) Specifies when incomplete multipart uploads are aborted. (see [below for nested schema](#nestedblock--rule--abort_incomplete_multipart_upload))
- `expiration` (Block List) Specifies when objects expire (are deleted). (see [below for nested schema](#nestedblock--rule--expiration))
- `filter` (Block List) Filter that identifies the objects to which the rule applies. If not specified, the rule applies to all objects in the bucket. (see [below for nested schema](#nestedblock--rule--filter))
- `noncurrent_version_expiration` (Block List) Specifies when noncurrent object versions expire. Only valid for versioned buckets. (see [below for nested schema](#nestedblock--rule--noncurrent_version_expiration))
- `noncurrent_version_transition` (Block List) Specifies when noncurrent object versions transition to a different storage class. Only valid for versioned buckets. (see [below for nested schema](#nestedblock--rule--noncurrent_version_transition))
- `transition` (Block List) Specifies when objects transition to a different storage class. (see [below for nested schema](#nestedblock--rule--transition))


<a id="nestedblock--rule--abort_incomplete_multipart_upload"></a>
### Nested Schema for `rule.abort_incomplete_multipart_upload`

Required:

- `days_after_initiation` (Number) Number of days after initiating a multipart upload when it should be aborted.



<a id="nestedblock--rule--expiration"></a>
### Nested Schema for `rule.expiration`



- `days` (Number) Number of days after object creation when the object expires.
- `expired_object_delete_marker` (Boolean) Whether to remove expired object delete markers. Only valid for versioned buckets.



<a id="nestedblock--rule--filter"></a>
### Nested Schema for `rule.filter`



- `and` (Block List) A logical AND to combine multiple filter conditions. Use this to apply a rule to objects that match all specified conditions. (see [below for nested schema](#nestedblock--rule--filter--and))
- `prefix` (String) Object key prefix that identifies one or more objects to which the rule applies.
- `tag` (Block List) A tag to filter objects. The rule applies only to objects that have the specified tag. (see [below for nested schema](#nestedblock--rule--filter--tag))


<a id="nestedblock--rule--filter--and"></a>
### Nested Schema for `rule.filter.and`



- `prefix` (String) Object key prefix.
- `tags` (Map of String) Map of tags that objects must have to match.



<a id="nestedblock--rule--filter--tag"></a>
### Nested Schema for `rule.filter.tag`

Required:

- `key` (String) The tag key.
- `value` (String) The tag value.




<a id="nestedblock--rule--noncurrent_version_expiration"></a>
### Nested Schema for `rule.noncurrent_version_expiration`

Required:

- `noncurrent_days` (Number) Number of days after an object becomes noncurrent when it expires.



- `newer_noncurrent_versions` (Number) Number of noncurrent versions to retain. If specified, the rule only applies after this many noncurrent versions exist.



<a id="nestedblock--rule--noncurrent_version_transition"></a>
### Nested Schema for `rule.noncurrent_version_transition`

Required:

- `noncurrent_days` (Number) Number of days after an object becomes noncurrent when the transition occurs.
- `storage_class` (String) The storage class to transition noncurrent versions to.



- `newer_noncurrent_versions` (Number) Number of noncurrent versions to retain before transitioning.



<a id="nestedblock--rule--transition"></a>
### Nested Schema for `rule.transition`

Required:

- `days` (Number) Number of days after object creation when the transition occurs.
- `storage_class` (String) The storage class to transition objects to. The available storage classes depend on your RadosGW configuration.

## Import

Import is supported using the following syntax:

```shell
# Import a bucket lifecycle configuration by bucket name
terraform import radosgw_s3_bucket_lifecycle_configuration.example "my-bucket-name"
```
