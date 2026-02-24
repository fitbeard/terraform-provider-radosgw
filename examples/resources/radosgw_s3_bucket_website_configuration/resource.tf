# Basic website configuration with index document
resource "radosgw_s3_bucket" "example" {
  bucket = "my-website-bucket"
}

resource "radosgw_s3_bucket_website_configuration" "example" {
  bucket = radosgw_s3_bucket.example.bucket

  index_document {
    suffix = "index.html"
  }
}

# Website with index and error documents
resource "radosgw_s3_bucket_website_configuration" "with_error_page" {
  bucket = radosgw_s3_bucket.example.bucket

  index_document {
    suffix = "index.html"
  }

  error_document {
    key = "error.html"
  }
}

# Redirect all requests to another host
resource "radosgw_s3_bucket" "redirect" {
  bucket = "my-redirect-bucket"
}

resource "radosgw_s3_bucket_website_configuration" "redirect" {
  bucket = radosgw_s3_bucket.redirect.bucket

  redirect_all_requests_to {
    host_name = "www.example.com"
    protocol  = "https"
  }
}

# Website with routing rules
resource "radosgw_s3_bucket_website_configuration" "with_routing" {
  bucket = radosgw_s3_bucket.example.bucket

  index_document {
    suffix = "index.html"
  }

  error_document {
    key = "error.html"
  }

  routing_rule {
    condition {
      key_prefix_equals = "docs/"
    }
    redirect {
      replace_key_prefix_with = "documents/"
    }
  }

  routing_rule {
    condition {
      http_error_code_returned_equals = "404"
    }
    redirect {
      replace_key_with = "error.html"
    }
  }
}
