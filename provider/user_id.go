package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// buildFullUserID constructs the RGW Admin API user ID.
// For tenant users, the format is "tenant$user_id".
// For non-tenant users, it is just "user_id".
func buildFullUserID(userID, tenant string) string {
	if userID == "" || strings.Contains(userID, "$") {
		return userID
	}
	if tenant != "" {
		return tenant + "$" + userID
	}
	return userID
}

func splitTenantQualifiedUserID(userID string) (tenant string, localUserID string, ok bool) {
	tenant, localUserID, ok = strings.Cut(userID, "$")
	if !ok {
		return "", userID, false
	}
	return tenant, localUserID, true
}

// resolveUserID returns a user ID suitable for RGW Admin API calls.
//
// Existing configurations commonly pass radosgw_iam_user.user_id to dependent
// resources. For tenant users that attribute is the local user name while RGW
// APIs expect "tenant$user_id". To preserve that contract, direct lookups use
// the input first, then fall back to a unique tenant-qualified match.
func resolveUserID(ctx context.Context, client *RadosgwClient, userID string) (string, error) {
	if userID == "" || strings.Contains(userID, "$") {
		return userID, nil
	}

	_, err := client.Admin.GetUser(ctx, admin.User{ID: userID})
	if err == nil {
		return userID, nil
	}
	if !errors.Is(err, admin.ErrNoSuchUser) {
		return "", err
	}

	users, listErr := client.Admin.GetUsers(ctx)
	if listErr != nil {
		return "", fmt.Errorf("user %q was not found directly and tenant lookup failed: %w; for tenant users, use the tenant-qualified format tenant$user_id", userID, listErr)
	}
	if users == nil {
		return "", err
	}

	suffix := "$" + userID
	matches := make([]string, 0, 1)
	for _, candidate := range *users {
		if strings.HasSuffix(candidate, suffix) {
			matches = append(matches, candidate)
		}
	}

	switch len(matches) {
	case 0:
		return "", err
	case 1:
		tflog.Debug(ctx, "Resolved tenant-qualified RGW user ID", map[string]any{
			"user_id":          userID,
			"resolved_user_id": matches[0],
		})
		return matches[0], nil
	default:
		return "", fmt.Errorf("user ID %q is ambiguous; it matches multiple tenant users (%s). Use the tenant-qualified format tenant$user_id", userID, strings.Join(matches, ", "))
	}
}
