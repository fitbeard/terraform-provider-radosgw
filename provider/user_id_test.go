package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestBuildFullUserID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		userID string
		tenant string
		want   string
	}{
		{
			name:   "empty user ID is preserved",
			userID: "",
			tenant: "tenant",
			want:   "",
		},
		{
			name:   "non-tenant user ID is unchanged",
			userID: "alice",
			tenant: "",
			want:   "alice",
		},
		{
			name:   "tenant user ID is qualified",
			userID: "alice",
			tenant: "tenant",
			want:   "tenant$alice",
		},
		{
			name:   "already-qualified user ID is preserved",
			userID: "tenant$alice",
			tenant: "other",
			want:   "tenant$alice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildFullUserID(tt.userID, tt.tenant)
			if got != tt.want {
				t.Fatalf("buildFullUserID(%q, %q) = %q, want %q", tt.userID, tt.tenant, got, tt.want)
			}
		})
	}
}

func TestSplitTenantQualifiedUserID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		userID      string
		wantTenant  string
		wantLocalID string
		wantOK      bool
	}{
		{
			name:        "tenant-qualified ID",
			userID:      "tenant$alice",
			wantTenant:  "tenant",
			wantLocalID: "alice",
			wantOK:      true,
		},
		{
			name:        "plain user ID",
			userID:      "alice",
			wantTenant:  "",
			wantLocalID: "alice",
			wantOK:      false,
		},
		{
			name:        "empty local ID",
			userID:      "tenant$",
			wantTenant:  "tenant",
			wantLocalID: "",
			wantOK:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotTenant, gotLocalID, gotOK := splitTenantQualifiedUserID(tt.userID)
			if gotTenant != tt.wantTenant || gotLocalID != tt.wantLocalID || gotOK != tt.wantOK {
				t.Fatalf("splitTenantQualifiedUserID(%q) = (%q, %q, %t), want (%q, %q, %t)",
					tt.userID,
					gotTenant,
					gotLocalID,
					gotOK,
					tt.wantTenant,
					tt.wantLocalID,
					tt.wantOK,
				)
			}
		})
	}
}

func TestResolveUserIDEarlyReturn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		userID string
	}{
		{
			name:   "empty user ID",
			userID: "",
		},
		{
			name:   "tenant-qualified user ID",
			userID: "tenant$alice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveUserID(context.Background(), nil, tt.userID)
			if err != nil {
				t.Fatalf("resolveUserID() returned unexpected error: %v", err)
			}
			if got != tt.userID {
				t.Fatalf("resolveUserID() = %q, want %q", got, tt.userID)
			}
		})
	}
}

func TestResolveUserIDDirectMatch(t *testing.T) {
	t.Parallel()

	client := newResolveUserIDTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/user" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("uid"); got != "alice" {
			t.Fatalf("uid = %q, want alice", got)
		}

		_, _ = fmt.Fprint(w, `{"user_id":"alice"}`)
	}))

	got, err := resolveUserID(context.Background(), client, "alice")
	if err != nil {
		t.Fatalf("resolveUserID() returned unexpected error: %v", err)
	}
	if got != "alice" {
		t.Fatalf("resolveUserID() = %q, want alice", got)
	}
}

func TestResolveUserIDTenantFallback(t *testing.T) {
	t.Parallel()

	client := newResolveUserIDTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/admin/user":
			writeAdminError(w, http.StatusNotFound, "NoSuchUser")
		case "/admin/metadata/user":
			_, _ = fmt.Fprint(w, `["tenant$alice","bob"]`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))

	got, err := resolveUserID(context.Background(), client, "alice")
	if err != nil {
		t.Fatalf("resolveUserID() returned unexpected error: %v", err)
	}
	if got != "tenant$alice" {
		t.Fatalf("resolveUserID() = %q, want tenant$alice", got)
	}
}

func TestResolveUserIDNoFallbackMatch(t *testing.T) {
	t.Parallel()

	client := newResolveUserIDTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/admin/user":
			writeAdminError(w, http.StatusNotFound, "NoSuchUser")
		case "/admin/metadata/user":
			_, _ = fmt.Fprint(w, `["tenant$bob"]`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))

	_, err := resolveUserID(context.Background(), client, "alice")
	if !errors.Is(err, admin.ErrNoSuchUser) {
		t.Fatalf("resolveUserID() error = %v, want ErrNoSuchUser", err)
	}
}

func TestResolveUserIDAmbiguousFallback(t *testing.T) {
	t.Parallel()

	client := newResolveUserIDTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/admin/user":
			writeAdminError(w, http.StatusNotFound, "NoSuchUser")
		case "/admin/metadata/user":
			_, _ = fmt.Fprint(w, `["tenant-a$alice","tenant-b$alice"]`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))

	_, err := resolveUserID(context.Background(), client, "alice")
	if err == nil {
		t.Fatal("resolveUserID() returned nil error, want ambiguity error")
	}
	if !strings.Contains(err.Error(), "ambiguous") ||
		!strings.Contains(err.Error(), "tenant-a$alice") ||
		!strings.Contains(err.Error(), "tenant-b$alice") {
		t.Fatalf("resolveUserID() error = %v, want ambiguity with both matches", err)
	}
}

func TestResolveUserIDListUsersError(t *testing.T) {
	t.Parallel()

	client := newResolveUserIDTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/admin/user":
			writeAdminError(w, http.StatusNotFound, "NoSuchUser")
		case "/admin/metadata/user":
			writeAdminError(w, http.StatusInternalServerError, "InternalError")
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))

	_, err := resolveUserID(context.Background(), client, "alice")
	if err == nil {
		t.Fatal("resolveUserID() returned nil error, want list users error")
	}
	if !strings.Contains(err.Error(), "tenant lookup failed") {
		t.Fatalf("resolveUserID() error = %v, want tenant lookup context", err)
	}
}

func TestPopulateUserResourceModelTenantIDs(t *testing.T) {
	t.Parallel()

	maxBuckets := 1000
	suspended := 0

	tests := []struct {
		name         string
		data         UserResourceModel
		user         admin.User
		wantID       string
		wantUserID   string
		wantTenant   string
		wantMax      int64
		wantSuspend  bool
		wantUserType string
	}{
		{
			name: "preserves configured local user ID",
			data: UserResourceModel{
				UserID: types.StringValue("alice"),
				Tenant: types.StringValue("tenant"),
			},
			user: admin.User{
				ID:         "tenant$alice",
				Tenant:     "",
				MaxBuckets: &maxBuckets,
				Suspended:  &suspended,
				Type:       "rgw",
			},
			wantID:       "tenant$alice",
			wantUserID:   "alice",
			wantTenant:   "tenant",
			wantMax:      1000,
			wantSuspend:  false,
			wantUserType: "rgw",
		},
		{
			name: "derives local user ID and tenant from API ID",
			data: UserResourceModel{},
			user: admin.User{
				ID:         "tenant$bob",
				MaxBuckets: &maxBuckets,
				Suspended:  &suspended,
				Type:       "rgw",
			},
			wantID:       "tenant$bob",
			wantUserID:   "bob",
			wantTenant:   "tenant",
			wantMax:      1000,
			wantSuspend:  false,
			wantUserType: "rgw",
		},
		{
			name: "normalizes configured tenant-qualified user ID",
			data: UserResourceModel{
				UserID: types.StringValue("tenant$carol"),
			},
			user: admin.User{
				ID:         "tenant$carol",
				Tenant:     "tenant",
				MaxBuckets: &maxBuckets,
				Suspended:  &suspended,
				Type:       "rgw",
			},
			wantID:       "tenant$carol",
			wantUserID:   "carol",
			wantTenant:   "tenant",
			wantMax:      1000,
			wantSuspend:  false,
			wantUserType: "rgw",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			populateUserResourceModel(&tt.data, tt.user)

			if got := tt.data.ID.ValueString(); got != tt.wantID {
				t.Fatalf("ID = %q, want %q", got, tt.wantID)
			}
			if got := tt.data.UserID.ValueString(); got != tt.wantUserID {
				t.Fatalf("UserID = %q, want %q", got, tt.wantUserID)
			}
			if got := tt.data.Tenant.ValueString(); got != tt.wantTenant {
				t.Fatalf("Tenant = %q, want %q", got, tt.wantTenant)
			}
			if got := tt.data.MaxBuckets.ValueInt64(); got != tt.wantMax {
				t.Fatalf("MaxBuckets = %d, want %d", got, tt.wantMax)
			}
			if got := tt.data.Suspended.ValueBool(); got != tt.wantSuspend {
				t.Fatalf("Suspended = %t, want %t", got, tt.wantSuspend)
			}
			if got := tt.data.Type.ValueString(); got != tt.wantUserType {
				t.Fatalf("Type = %q, want %q", got, tt.wantUserType)
			}
		})
	}
}

func newResolveUserIDTestClient(t *testing.T, handler http.Handler) *RadosgwClient {
	t.Helper()

	adminClient, err := admin.New("http://radosgw.test", "access-key", "secret-key", resolveUserIDHTTPClient{handler: handler})
	if err != nil {
		t.Fatalf("admin.New() returned error: %v", err)
	}

	return &RadosgwClient{Admin: adminClient}
}

type resolveUserIDHTTPClient struct {
	handler http.Handler
}

func (c resolveUserIDHTTPClient) Do(req *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	c.handler.ServeHTTP(recorder, req)
	return recorder.Result(), nil
}

func writeAdminError(w http.ResponseWriter, statusCode int, code string) {
	w.WriteHeader(statusCode)
	_, _ = fmt.Fprintf(w, `{"Code":%q,"RequestId":"test-request","HostId":"test-host"}`, code)
}
