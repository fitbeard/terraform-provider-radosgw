package provider

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure RadosgwProvider satisfies various provider interfaces.
var _ provider.Provider = &RadosgwProvider{}

// RadosgwProvider defines the provider implementation.
type RadosgwProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// RadosgwProviderModel describes the provider data model.
type RadosgwProviderModel struct {
	Endpoint              types.String `tfsdk:"endpoint"`
	AccessKey             types.String `tfsdk:"access_key"`
	SecretKey             types.String `tfsdk:"secret_key"`
	TLSInsecureSkipVerify types.Bool   `tfsdk:"tls_insecure_skip_verify"`
	RootCACertificate     types.String `tfsdk:"root_ca_certificate"`
	RootCACertificateFile types.String `tfsdk:"root_ca_certificate_file"`
}

// RadosgwClient holds both admin and S3 clients
type RadosgwClient struct {
	Admin *admin.API
	S3    *s3.Client
}

func (p *RadosgwProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "radosgw"
	resp.Version = p.version
}

func (p *RadosgwProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Terraform provider for managing Ceph RADOS Gateway (RadosGW) resources via Admin and S3 APIs.

## Required User Capabilities

The RadosGW user configured in this provider requires specific capabilities to manage different resources:

| Capability | Resources |
|------------|-----------|
| ` + "`users=*`" + ` | ` + "`radosgw_iam_user`" + `, ` + "`radosgw_iam_subuser`" + `, ` + "`radosgw_iam_access_key`" + `, ` + "`radosgw_iam_user_caps`" + `, ` + "`radosgw_iam_quota`" + `, ` + "`radosgw_iam_user`" + `, ` + "`radosgw_iam_users`" + ` |
| ` + "`buckets=*`" + ` | ` + "`radosgw_s3_bucket`" + `, ` + "`radosgw_s3_bucket_link`" + `, ` + "`radosgw_s3_bucket_acl`" + `, ` + "`radosgw_s3_bucket_policy`" + `, ` + "`radosgw_s3_bucket_lifecycle_configuration`" + ` |
| ` + "`oidc-provider=*`" + ` | ` + "`radosgw_iam_openid_connect_provider`" + ` |
| ` + "`roles=*`" + ` | ` + "`radosgw_iam_role`" + `, ` + "`radosgw_iam_role_policy`" + `, ` + "`radosgw_iam_roles`" + ` |
| ` + "`metadata=*`" + ` | ` + "`radosgw_iam_users`" + ` |

To grant all required capabilities to a user:

` + "```bash" + `
radosgw-admin caps add --uid=admin --caps="buckets=*;metadata=*;oidc-provider=*;roles=*;users=*"
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "RadosGW endpoint URL. Can be set via the `RADOSGW_ENDPOINT` environment variable.",
				Optional:            true,
			},
			"access_key": schema.StringAttribute{
				MarkdownDescription: "RadosGW access key. Can be set via the `RADOSGW_ACCESS_KEY` environment variable.",
				Optional:            true,
			},
			"secret_key": schema.StringAttribute{
				MarkdownDescription: "RadosGW secret key. Can be set via the `RADOSGW_SECRET_KEY` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"tls_insecure_skip_verify": schema.BoolAttribute{
				MarkdownDescription: "Skip TLS certificate verification for HTTPS connections. This is useful when connecting to RadosGW with self-signed certificates or certificates signed by an untrusted CA. Has no effect on plain HTTP connections. Can be set via the `RADOSGW_TLS_INSECURE_SKIP_VERIFY` environment variable. Default is `false`.",
				Optional:            true,
			},
			"root_ca_certificate": schema.StringAttribute{
				MarkdownDescription: "PEM-encoded root CA certificate content to use for TLS verification. Can be set via the `RADOSGW_ROOT_CA_CERTIFICATE` environment variable.",
				Optional:            true,
			},
			"root_ca_certificate_file": schema.StringAttribute{
				MarkdownDescription: "Path to a PEM-encoded root CA certificate file to use for TLS verification. Can be set via the `RADOSGW_ROOT_CA_CERTIFICATE_FILE` environment variable.",
				Optional:            true,
			},
		},
	}
}

func (p *RadosgwProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config RadosgwProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Check environment variables
	endpoint := os.Getenv("RADOSGW_ENDPOINT")
	accessKey := os.Getenv("RADOSGW_ACCESS_KEY")
	secretKey := os.Getenv("RADOSGW_SECRET_KEY")
	tlsInsecureSkipVerify := os.Getenv("RADOSGW_TLS_INSECURE_SKIP_VERIFY") == "true"
	rootCACertificate := os.Getenv("RADOSGW_ROOT_CA_CERTIFICATE")
	rootCACertificateFile := os.Getenv("RADOSGW_ROOT_CA_CERTIFICATE_FILE")

	// Override with config values if provided
	if !config.Endpoint.IsNull() {
		endpoint = config.Endpoint.ValueString()
	}
	if !config.AccessKey.IsNull() {
		accessKey = config.AccessKey.ValueString()
	}
	if !config.SecretKey.IsNull() {
		secretKey = config.SecretKey.ValueString()
	}
	if !config.TLSInsecureSkipVerify.IsNull() {
		tlsInsecureSkipVerify = config.TLSInsecureSkipVerify.ValueBool()
	}
	if !config.RootCACertificate.IsNull() {
		rootCACertificate = config.RootCACertificate.ValueString()
	}
	if !config.RootCACertificateFile.IsNull() {
		rootCACertificateFile = config.RootCACertificateFile.ValueString()
	}

	// Validate required fields
	if endpoint == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Missing RadosGW Endpoint",
			"The provider cannot create the RadosGW client as there is a missing or empty value for the RadosGW endpoint. "+
				"Set the endpoint value in the configuration or use the RADOSGW_ENDPOINT environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if accessKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("access_key"),
			"Missing RadosGW Access Key",
			"The provider cannot create the RadosGW client as there is a missing or empty value for the RadosGW access key. "+
				"Set the access_key value in the configuration or use the RADOSGW_ACCESS_KEY environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if secretKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("secret_key"),
			"Missing RadosGW Secret Key",
			"The provider cannot create the RadosGW client as there is a missing or empty value for the RadosGW secret key. "+
				"Set the secret_key value in the configuration or use the RADOSGW_SECRET_KEY environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "radosgw_endpoint", endpoint)
	ctx = tflog.SetField(ctx, "radosgw_access_key", accessKey)
	ctx = tflog.SetField(ctx, "radosgw_secret_key", secretKey)
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "radosgw_secret_key")

	tflog.Debug(ctx, "Creating RadosGW clients")

	// Configure TLS
	tlsConfig := &tls.Config{
		InsecureSkipVerify: tlsInsecureSkipVerify, //nolint:gosec // User-configured option for self-signed certs
	}

	// Load CA certificate if provided
	if rootCACertificate != "" || rootCACertificateFile != "" {
		caCertPool := x509.NewCertPool()

		// Load from certificate content
		if rootCACertificate != "" {
			if !caCertPool.AppendCertsFromPEM([]byte(rootCACertificate)) {
				resp.Diagnostics.AddError(
					"Invalid Root CA Certificate",
					"The provided root_ca_certificate content could not be parsed as a valid PEM-encoded certificate.",
				)
				return
			}
			tflog.Debug(ctx, "Loaded root CA certificate from content")
		}

		// Load from certificate file
		if rootCACertificateFile != "" {
			caCert, err := os.ReadFile(rootCACertificateFile)
			if err != nil {
				resp.Diagnostics.AddError(
					"Unable to Read Root CA Certificate File",
					"Failed to read the root CA certificate file at "+rootCACertificateFile+".\n\n"+
						"Error: "+err.Error(),
				)
				return
			}
			if !caCertPool.AppendCertsFromPEM(caCert) {
				resp.Diagnostics.AddError(
					"Invalid Root CA Certificate File",
					"The file at "+rootCACertificateFile+" could not be parsed as a valid PEM-encoded certificate.",
				)
				return
			}
			tflog.Debug(ctx, "Loaded root CA certificate from file", map[string]any{
				"file": rootCACertificateFile,
			})
		}

		tlsConfig.RootCAs = caCertPool
	}

	// Create custom HTTP transport with TLS config
	httpTransport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	// Create custom HTTP client
	httpClient := &http.Client{
		Transport: httpTransport,
	}

	// Create Admin API client
	adminClient, err := admin.New(endpoint, accessKey, secretKey, httpClient)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create RadosGW Admin API Client",
			"An unexpected error occurred when creating the RadosGW Admin API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"RadosGW Client Error: "+err.Error(),
		)
		return
	}

	// Create S3 client with custom endpoint and HTTP client
	s3Client := s3.NewFromConfig(aws.Config{
		Region:      "default",
		Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		HTTPClient:  httpClient,
	}, func(o *s3.Options) {
		o.BaseEndpoint = &endpoint
		o.UsePathStyle = true
	})

	client := &RadosgwClient{
		Admin: adminClient,
		S3:    s3Client,
	}

	resp.DataSourceData = client
	resp.ResourceData = client

	tflog.Info(ctx, "Configured RadosGW provider", map[string]any{
		"success": true,
	})
}

func (p *RadosgwProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewIAMUserResource,
		NewIAMQuotaResource,
		NewIAMUserCapsResource,
		NewIAMSubuserResource,
		NewIAMOIDCProviderResource,
		NewIAMAcessKeyResource,
		NewIAMRoleResource,
		NewIAMRolePolicyResource,
		NewS3BucketLinkResource,
		NewS3BucketResource,
		NewS3BucketAclResource,
		NewS3BucketPolicyResource,
		NewS3BucketLifecycleResource,
	}
}

func (p *RadosgwProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewIAMPolicyDocumentDataSource,
		NewIAMOIDCProviderDataSource,
		NewIAMUserDataSource,
		NewIAMUsersDataSource,
		NewIAMRoleDataSource,
		NewIAMRolesDataSource,
		NewIAMAccessKeysDataSource,
		NewIAMUserCapsDataSource,
		NewIAMSubusersDataSource,
		NewIAMQuotaDataSource,
		NewS3BucketDataSource,
		NewS3BucketPolicyDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &RadosgwProvider{
			version: version,
		}
	}
}
