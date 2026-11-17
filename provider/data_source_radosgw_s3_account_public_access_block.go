package provider

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	s3controltypes "github.com/aws/aws-sdk-go-v2/service/s3control/types"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &S3AccountPublicAccessBlockDataSource{}

func NewS3AccountPublicAccessBlockDataSource() datasource.DataSource {
	return &S3AccountPublicAccessBlockDataSource{}
}

// S3AccountPublicAccessBlockDataSource reads an account's public access block.
type S3AccountPublicAccessBlockDataSource struct {
	client *RadosgwClient
}

type S3AccountPublicAccessBlockDataSourceModel struct {
	AccountID             types.String `tfsdk:"account_id"`
	BlockPublicAcls       types.Bool   `tfsdk:"block_public_acls"`
	BlockPublicPolicy     types.Bool   `tfsdk:"block_public_policy"`
	IgnorePublicAcls      types.Bool   `tfsdk:"ignore_public_acls"`
	RestrictPublicBuckets types.Bool   `tfsdk:"restrict_public_buckets"`
}

func (d *S3AccountPublicAccessBlockDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_account_public_access_block"
}

func (d *S3AccountPublicAccessBlockDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the account-wide S3 Public Access Block configuration of a RadosGW account, via " +
			"the S3 Control `GetPublicAccessBlock` API.\n\n" +
			"~> **Ceph version:** account-wide Public Access Block is **Ceph 21 (Umbrella) only**. Reading it when the " +
			"provider's `ceph_version` targets an older release returns an error.\n\n" +
			"~> **Note:** When the account has no public-access-block configuration, all four settings are reported as " +
			"`false` — the effective state, in which nothing is blocked.",

		Attributes: map[string]schema.Attribute{
			"account_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the RadosGW account.",
				Required:            true,
			},
			"block_public_acls": schema.BoolAttribute{
				MarkdownDescription: "Whether RadosGW blocks public ACLs for buckets in this account.",
				Computed:            true,
			},
			"block_public_policy": schema.BoolAttribute{
				MarkdownDescription: "Whether RadosGW blocks public bucket policies for buckets in this account.",
				Computed:            true,
			},
			"ignore_public_acls": schema.BoolAttribute{
				MarkdownDescription: "Whether RadosGW ignores public ACLs for buckets in this account.",
				Computed:            true,
			},
			"restrict_public_buckets": schema.BoolAttribute{
				MarkdownDescription: "Whether RadosGW restricts public bucket policies for buckets in this account.",
				Computed:            true,
			},
		},
	}
}

func (d *S3AccountPublicAccessBlockDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*RadosgwClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *RadosgwClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *S3AccountPublicAccessBlockDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config S3AccountPublicAccessBlockDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !d.client.CephVersion.GreaterThanOrEqual(CephVersion_Umbrella) {
		resp.Diagnostics.AddError(
			"radosgw_s3_account_public_access_block requires Ceph 21 (Umbrella)",
			fmt.Sprintf("Account-wide Public Access Block (the S3 Control API) was added in Ceph 21.1.1 (Umbrella) "+
				"and was not backported to Squid or Tentacle, but the provider targets %s.", d.client.CephVersion),
		)
		return
	}

	accountID := config.AccountID.ValueString()

	tflog.Debug(ctx, "Reading S3 account public access block", map[string]any{"account_id": accountID})

	output, err := d.client.S3Control.GetPublicAccessBlock(ctx, &s3control.GetPublicAccessBlockInput{
		AccountId: aws.String(accountID),
	})
	if err != nil {
		if isS3NoSuchPublicAccessBlock(err) {
			// No configuration: report the effective state (nothing blocked).
			config.BlockPublicAcls = types.BoolValue(false)
			config.BlockPublicPolicy = types.BoolValue(false)
			config.IgnorePublicAcls = types.BoolValue(false)
			config.RestrictPublicBuckets = types.BoolValue(false)
			resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading S3 Account Public Access Block",
			fmt.Sprintf("Could not read public access block for account %q: %s", accountID, err),
		)
		return
	}

	cfg := output.PublicAccessBlockConfiguration
	if cfg == nil {
		cfg = &s3controltypes.PublicAccessBlockConfiguration{}
	}
	config.BlockPublicAcls = types.BoolValue(aws.ToBool(cfg.BlockPublicAcls))
	config.BlockPublicPolicy = types.BoolValue(aws.ToBool(cfg.BlockPublicPolicy))
	config.IgnorePublicAcls = types.BoolValue(aws.ToBool(cfg.IgnorePublicAcls))
	config.RestrictPublicBuckets = types.BoolValue(aws.ToBool(cfg.RestrictPublicBuckets))

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
