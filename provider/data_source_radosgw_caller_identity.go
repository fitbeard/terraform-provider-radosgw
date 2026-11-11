package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &CallerIdentityDataSource{}

func NewCallerIdentityDataSource() datasource.DataSource {
	return &CallerIdentityDataSource{}
}

// CallerIdentityDataSource returns details about the identity the provider's
// credentials resolve to, via the STS GetCallerIdentity API.
type CallerIdentityDataSource struct {
	client *RadosgwClient
}

// CallerIdentityDataSourceModel describes the data source data model.
type CallerIdentityDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	AccountID types.String `tfsdk:"account_id"`
	ARN       types.String `tfsdk:"arn"`
	UserID    types.String `tfsdk:"user_id"`
}

func (d *CallerIdentityDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_caller_identity"
}

func (d *CallerIdentityDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to get information about the identity that the provider's " +
			"credentials resolve to, as reported by the STS `GetCallerIdentity` API. This is the RadosGW " +
			"equivalent of `aws_caller_identity`.\n\n" +
			"The returned values depend on the kind of caller:\n\n" +
			"| Caller | `account_id` | `arn` | `user_id` |\n" +
			"|--------|--------------|-------|-----------|\n" +
			"| Plain (non-account) user | *(empty)* | `arn:aws:iam:::user/<uid>` | the user's UID |\n" +
			"| Account root | account ID | `arn:aws:iam::<account>:root` | the account ID |\n" +
			"| Account member user | account ID | `arn:aws:iam::<account>:user/<name>` | a unique ID |\n\n" +
			"~> **Ceph version:** `GetCallerIdentity` requires **Ceph 21 (Umbrella)**. When the provider's " +
			"`ceph_version` is set to an older release, this data source returns an error.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The account ID of the caller, mirroring `account_id` (for parity with " +
					"`aws_caller_identity`, whose `id` is the account ID).",
				Computed: true,
			},
			"account_id": schema.StringAttribute{
				MarkdownDescription: "The RadosGW account ID of the caller. Empty for a plain (non-account) user; " +
					"the account ID for an account root or account member user.",
				Computed: true,
			},
			"arn": schema.StringAttribute{
				MarkdownDescription: "The ARN associated with the credentials used to make the request " +
					"(e.g. `arn:aws:iam:::user/admin`, `arn:aws:iam::<account>:root`, or " +
					"`arn:aws:iam::<account>:user/<name>`).",
				Computed: true,
			},
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the calling entity: the user's UID for a plain user, " +
					"the account ID for an account root, or a unique ID for an account member user.",
				Computed: true,
			},
		},
	}
}

func (d *CallerIdentityDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *CallerIdentityDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CallerIdentityDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// GetCallerIdentity is only available on Ceph 21 (Umbrella). Fail with a clear
	// message when the provider targets an older release.
	if d.client.CephVersion.LessThan(CephVersion_Umbrella) {
		resp.Diagnostics.AddError(
			"GetCallerIdentity requires Ceph 21 (Umbrella)",
			fmt.Sprintf("The STS GetCallerIdentity API is only available on Ceph 21 (Umbrella) or higher, but the "+
				"provider targets %s. Set the provider `ceph_version` to `21`/`umbrella` on an Umbrella cluster to use "+
				"this data source.", d.client.CephVersion),
		)
		return
	}

	tflog.Debug(ctx, "Reading RadosGW caller identity")

	identity, err := getCallerIdentity(ctx, newAccountIAMClient(d.client))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Caller Identity",
			fmt.Sprintf("Could not get caller identity: %s", err.Error()),
		)
		return
	}

	data.AccountID = types.StringValue(identity.Account)
	data.ID = types.StringValue(identity.Account)
	data.ARN = types.StringValue(identity.Arn)
	data.UserID = types.StringValue(identity.UserID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
