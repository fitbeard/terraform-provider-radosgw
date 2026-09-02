package provider

import (
	"fmt"
	"net/url"
	"testing"

	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/ceph/go-ceph/rgw/admin"
)

// =============================================================================
// Pure string / collection helpers
// =============================================================================

func TestContains(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, s, substr string
		want            bool
	}{
		{"present", "hello world", "o w", true},
		{"prefix", "NoSuchBucket", "NoSuch", true},
		{"suffix", "NoSuchBucket", "Bucket", true},
		{"absent", "hello", "xyz", false},
		{"substr longer than s", "ab", "abc", false},
		{"empty substr matches", "abc", "", true},
		{"empty s and substr", "", "", true},
		{"empty s nonempty substr", "", "x", false},
		{"exact", "abc", "abc", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contains(tt.s, tt.substr); got != tt.want {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
			// searchString is the inner scan; for substr no longer than s it must agree.
			if len(tt.substr) <= len(tt.s) {
				if got := searchString(tt.s, tt.substr); got != tt.want {
					t.Errorf("searchString(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
				}
			}
		})
	}
}

func TestMergePermissions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		p1, p2, want string
	}{
		{"*", "read", "*"},
		{"read", "*", "*"},
		{"read", "write", "*"},
		{"write", "read", "*"},
		{"read", "read", "read"},
		{"write", "write", "write"},
		{"*", "*", "*"},
		{"read", "unknown", "*"}, // defensive fallback
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s+%s", tt.p1, tt.p2), func(t *testing.T) {
			if got := mergePermissions(tt.p1, tt.p2); got != tt.want {
				t.Errorf("mergePermissions(%q, %q) = %q, want %q", tt.p1, tt.p2, got, tt.want)
			}
		})
	}
}

func TestNextAttributeIndex(t *testing.T) {
	t.Parallel()

	if got := nextAttributeIndex(url.Values{}); got != 1 {
		t.Errorf("empty params: got %d, want 1", got)
	}

	one := url.Values{}
	one.Set("Attributes.entry.1.key", "policy")
	if got := nextAttributeIndex(one); got != 2 {
		t.Errorf("one entry: got %d, want 2", got)
	}

	three := url.Values{}
	three.Set("Attributes.entry.1.key", "a")
	three.Set("Attributes.entry.2.key", "b")
	three.Set("Attributes.entry.3.key", "c")
	if got := nextAttributeIndex(three); got != 4 {
		t.Errorf("three entries: got %d, want 4", got)
	}
}

func TestToStringSet(t *testing.T) {
	t.Parallel()

	got := toStringSet([]string{"a", "b", "a", "c"})
	if len(got) != 3 {
		t.Errorf("expected 3 unique keys, got %d (%v)", len(got), got)
	}
	for _, want := range []string{"a", "b", "c"} {
		if _, ok := got[want]; !ok {
			t.Errorf("missing key %q", want)
		}
	}
	if _, ok := got["z"]; ok {
		t.Error("unexpected key present")
	}

	if empty := toStringSet(nil); len(empty) != 0 {
		t.Errorf("nil input: expected empty set, got %v", empty)
	}
}

// =============================================================================
// S3 error classifiers
// =============================================================================

func TestIsS3ErrorCode(t *testing.T) {
	t.Parallel()
	if isS3ErrorCode(nil, "NoSuchBucket") {
		t.Error("nil error must be false")
	}
	if !isS3ErrorCode(fmt.Errorf("operation error S3: GetBucketAcl, NoSuchBucket: ..."), "NoSuchBucket") {
		t.Error("matching code must be true")
	}
	if isS3ErrorCode(fmt.Errorf("some other failure"), "NoSuchBucket") {
		t.Error("non-matching code must be false")
	}
}

func TestIsS3NoSuchWebsiteConfiguration(t *testing.T) {
	t.Parallel()
	if isS3NoSuchWebsiteConfiguration(nil) {
		t.Error("nil must be false")
	}
	if !isS3NoSuchWebsiteConfiguration(fmt.Errorf("api error NoSuchWebsiteConfiguration: ...")) {
		t.Error("matching must be true")
	}
	if isS3NoSuchWebsiteConfiguration(fmt.Errorf("NoSuchBucket")) {
		t.Error("different code must be false")
	}
}

func TestIsS3NoSuchCORSConfiguration(t *testing.T) {
	t.Parallel()
	if isS3NoSuchCORSConfiguration(nil) {
		t.Error("nil must be false")
	}
	if !isS3NoSuchCORSConfiguration(fmt.Errorf("api error NoSuchCORSConfiguration: ...")) {
		t.Error("matching must be true")
	}
	if isS3NoSuchCORSConfiguration(fmt.Errorf("NoSuchBucket")) {
		t.Error("different code must be false")
	}
}

func TestIsBucketNotFoundS3Error(t *testing.T) {
	t.Parallel()
	if isBucketNotFoundS3Error(nil) {
		t.Error("nil must be false")
	}
	if !isBucketNotFoundS3Error(fmt.Errorf("api error NoSuchBucket: ...")) {
		t.Error("NoSuchBucket must be true")
	}
	if !isBucketNotFoundS3Error(fmt.Errorf("StatusCode: 404")) {
		t.Error("404 must be true")
	}
	if isBucketNotFoundS3Error(fmt.Errorf("AccessDenied")) {
		t.Error("other code must be false")
	}
}

func TestIsS3BucketNotFound(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"typed NotFound", &s3types.NotFound{}, true},
		{"typed NoSuchBucket", &s3types.NoSuchBucket{}, true},
		{"api NoSuchBucket", &smithy.GenericAPIError{Code: "NoSuchBucket"}, true},
		{"api NotFound", &smithy.GenericAPIError{Code: "NotFound"}, true},
		{"api 404", &smithy.GenericAPIError{Code: "404"}, true},
		{"api AccessDenied", &smithy.GenericAPIError{Code: "AccessDenied"}, false},
		{"plain error", fmt.Errorf("boom"), false},
		{"wrapped typed", fmt.Errorf("read failed: %w", &s3types.NotFound{}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isS3BucketNotFound(tt.err); got != tt.want {
				t.Errorf("isS3BucketNotFound(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// =============================================================================
// SNS / account error classifiers
// =============================================================================

func TestIsConcurrentModificationError(t *testing.T) {
	t.Parallel()
	if isConcurrentModificationError(nil) {
		t.Error("nil must be false")
	}
	if !isConcurrentModificationError(fmt.Errorf("ConcurrentModification: try again")) {
		t.Error("matching must be true")
	}
	if isConcurrentModificationError(fmt.Errorf("NoSuchKey")) {
		t.Error("other must be false")
	}
}

func TestIsTransientSNSError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"concurrent modification", fmt.Errorf("ConcurrentModification"), true},
		{"api NoSuchKey", &smithy.GenericAPIError{Code: "NoSuchKey"}, true},
		{"api ServiceUnavailable", &smithy.GenericAPIError{Code: "ServiceUnavailable"}, true},
		{"api SlowDown", &smithy.GenericAPIError{Code: "SlowDown"}, true},
		{"api AccessDenied", &smithy.GenericAPIError{Code: "AccessDenied"}, false},
		{"iam 5xx", &IAMError{Code: "InternalError", StatusCode: 503}, true},
		{"iam transient code", &IAMError{Code: "SlowDown", StatusCode: 400}, true},
		{"iam non-transient", &IAMError{Code: "NoSuchEntity", StatusCode: 404}, false},
		{"plain error", fmt.Errorf("boom"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransientSNSError(tt.err); got != tt.want {
				t.Errorf("isTransientSNSError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsAccountNotFoundError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"admin ErrNoSuchKey", admin.ErrNoSuchKey, true},
		{"wrapped admin ErrNoSuchKey", fmt.Errorf("get account: %w", admin.ErrNoSuchKey), true},
		{"string NoSuchKey", fmt.Errorf("something NoSuchKey happened"), true},
		{"string NoSuchAccount", fmt.Errorf("NoSuchAccount"), true},
		{"other", fmt.Errorf("AccessDenied"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAccountNotFoundError(tt.err); got != tt.want {
				t.Errorf("isAccountNotFoundError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// =============================================================================
// IAMError type + parseErrorResponse
// =============================================================================

func TestIAMErrorErrorAndIs(t *testing.T) {
	t.Parallel()

	withMsg := &IAMError{Code: "NoSuchEntity", Message: "not found", StatusCode: 404}
	if got, want := withMsg.Error(), "NoSuchEntity: not found (HTTP 404)"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	noMsg := &IAMError{Code: "AccessDenied", StatusCode: 403}
	if got, want := noMsg.Error(), "AccessDenied (HTTP 403)"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	// Is matches purely on Code (see ErrNoSuchEntity sentinel usage).
	if !withMsg.Is(ErrNoSuchEntity) {
		t.Error("expected IAMError with code NoSuchEntity to match ErrNoSuchEntity")
	}
	if noMsg.Is(ErrNoSuchEntity) {
		t.Error("AccessDenied must not match ErrNoSuchEntity")
	}
	if withMsg.Is(fmt.Errorf("plain")) {
		t.Error("must not match a non-IAMError target")
	}
}

func TestParseErrorResponse(t *testing.T) {
	t.Parallel()
	c := &IAMClient{}

	assertIAM := func(t *testing.T, err error, wantCode string, wantStatus int) {
		t.Helper()
		iamErr, ok := err.(*IAMError)
		if !ok {
			t.Fatalf("expected *IAMError, got %T (%v)", err, err)
		}
		if iamErr.Code != wantCode {
			t.Errorf("code = %q, want %q", iamErr.Code, wantCode)
		}
		if iamErr.StatusCode != wantStatus {
			t.Errorf("status = %d, want %d", iamErr.StatusCode, wantStatus)
		}
	}

	t.Run("405 method not allowed", func(t *testing.T) {
		assertIAM(t, c.parseErrorResponse(405, nil, "CreateAccount"), "MethodNotAllowed", 405)
	})
	t.Run("IAM-style xml", func(t *testing.T) {
		body := []byte(`<ErrorResponse><Error><Code>NoSuchEntity</Code><Message>nope</Message></Error></ErrorResponse>`)
		assertIAM(t, c.parseErrorResponse(404, body, "GetRole"), "NoSuchEntity", 404)
	})
	t.Run("S3-style xml", func(t *testing.T) {
		body := []byte(`<Error><Code>AccessDenied</Code><Message>denied</Message></Error>`)
		assertIAM(t, c.parseErrorResponse(403, body, "CreateTopic"), "AccessDenied", 403)
	})
	t.Run("unparseable", func(t *testing.T) {
		assertIAM(t, c.parseErrorResponse(500, []byte("not xml"), "Whatever"), "UnknownError", 500)
	})
}
