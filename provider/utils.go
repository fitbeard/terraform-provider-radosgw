package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
)

// =============================================================================
// Retry Utilities
// =============================================================================

const (
	// DefaultOperationTimeout is the default timeout for retryable operations.
	// Based on AWS examples best practices, 2 minutes is a reasonable default.
	DefaultOperationTimeout = 2 * time.Minute
)

// isConcurrentModificationError checks if an error is a ConcurrentModification error.
// Note: go-ceph doesn't expose this as a typed error, so we check the error string.
func isConcurrentModificationError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "ConcurrentModification")
}

// retryOnConcurrentModification wraps an operation with retry logic for ConcurrentModification errors
// using retry.RetryContext helper.
func retryOnConcurrentModification(ctx context.Context, operation string, fn func() error) error {
	var lastErr error

	err := retry.RetryContext(ctx, DefaultOperationTimeout, func() *retry.RetryError {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// Only retry on ConcurrentModification errors
		if isConcurrentModificationError(lastErr) {
			tflog.Debug(ctx, "ConcurrentModification detected, retrying", map[string]any{
				"operation": operation,
				"error":     lastErr.Error(),
			})
			return retry.RetryableError(lastErr)
		}

		// All other errors are non-retryable
		return retry.NonRetryableError(lastErr)
	})

	if err != nil {
		tflog.Warn(ctx, "Operation failed", map[string]any{
			"operation": operation,
			"error":     err.Error(),
		})
	}

	return err
}

// =============================================================================
// IAM Client and AWS SigV4 Signing
// =============================================================================

// HTTPClient is an interface that matches the http.Client.Do method signature.
// This allows using custom HTTP clients (e.g., with TLS configuration).
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// IAMClient provides AWS IAM-compatible API operations for RadosGW.
// This client uses AWS SigV4 signing and can be used for OIDC providers,
// roles, policies, and other IAM-like operations supported by RadosGW.
type IAMClient struct {
	Endpoint   string
	AccessKey  string
	SecretKey  string
	HTTPClient HTTPClient
	Signer     *v4.Signer
}

// NewIAMClient creates a new IAM client for RadosGW.
func NewIAMClient(endpoint, accessKey, secretKey string, httpClient HTTPClient) *IAMClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &IAMClient{
		Endpoint:   endpoint,
		AccessKey:  accessKey,
		SecretKey:  secretKey,
		HTTPClient: httpClient,
		Signer:     v4.NewSigner(),
	}
}

// emptyPayloadHash is the SHA256 hash of an empty string
const emptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// DoRequest executes a signed IAM API request and returns the response body.
// The service parameter should be "iam" for IAM operations or "sts" for STS operations.
func (c *IAMClient) DoRequest(ctx context.Context, params url.Values, service string) ([]byte, error) {
	// Build the full URL with query parameters
	reqURL := fmt.Sprintf("%s/?%s", c.Endpoint, params.Encode())

	tflog.Debug(ctx, "Making IAM API request", map[string]interface{}{
		"action":   params.Get("Action"),
		"service":  service,
		"endpoint": c.Endpoint,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set required headers
	req.Header.Set("Host", req.URL.Host)

	// Create credentials for signing
	credentials := aws.Credentials{
		AccessKeyID:     c.AccessKey,
		SecretAccessKey: c.SecretKey,
	}

	// Sign the request using AWS SDK v4 signer
	// Using empty region for RadosGW compatibility
	// The service is typically "iam" for IAM operations, but RadosGW uses "s3" for signing
	err = c.Signer.SignHTTP(ctx, credentials, req, emptyPayloadHash, "s3", "", time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	tflog.Debug(ctx, "Received IAM API response", map[string]interface{}{
		"status_code": resp.StatusCode,
		"body":        string(body),
	})

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.parseErrorResponse(resp.StatusCode, body, params.Get("Action"))
	}

	return body, nil
}

// =============================================================================
// IAM Error Types
// =============================================================================

// IAMErrorResponse represents an AWS IAM-style error response.
type IAMErrorResponse struct {
	XMLName xml.Name `xml:"ErrorResponse"`
	Error   struct {
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	} `xml:"Error"`
	RequestID string `xml:"RequestId"`
}

// IAMError represents a parsed IAM API error.
type IAMError struct {
	Code       string
	Message    string
	StatusCode int
	Action     string
}

func (e *IAMError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %s (HTTP %d)", e.Code, e.Message, e.StatusCode)
	}
	return fmt.Sprintf("%s (HTTP %d)", e.Code, e.StatusCode)
}

// Is implements error comparison for IAMError.
func (e *IAMError) Is(target error) bool {
	t, ok := target.(*IAMError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// Common IAM error codes
var (
	ErrNoSuchEntity        = &IAMError{Code: "NoSuchEntity"}
	ErrEntityAlreadyExists = &IAMError{Code: "EntityAlreadyExists"}
	ErrMalformedInput      = &IAMError{Code: "MalformedInput"}
	ErrInvalidInput        = &IAMError{Code: "InvalidInput"}
	ErrLimitExceeded       = &IAMError{Code: "LimitExceeded"}
	ErrAccessDenied        = &IAMError{Code: "AccessDenied"}
)

func (c *IAMClient) parseErrorResponse(statusCode int, body []byte, action string) error {
	// Check for specific HTTP status codes first
	if statusCode == 405 {
		return &IAMError{
			Code:       "MethodNotAllowed",
			Message:    fmt.Sprintf("operation not supported: %s. This may require a newer Ceph version.", action),
			StatusCode: statusCode,
			Action:     action,
		}
	}

	// Try to parse XML error response
	var errResp IAMErrorResponse
	if err := xml.Unmarshal(body, &errResp); err == nil && errResp.Error.Code != "" {
		return &IAMError{
			Code:       errResp.Error.Code,
			Message:    errResp.Error.Message,
			StatusCode: statusCode,
			Action:     action,
		}
	}

	// Fallback for unparseable responses
	return &IAMError{
		Code:       "UnknownError",
		Message:    string(body),
		StatusCode: statusCode,
		Action:     action,
	}
}

// HashPayload computes the SHA256 hash of a payload.
func HashPayload(payload []byte) string {
	if len(payload) == 0 {
		return emptyPayloadHash
	}
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:])
}
