package provider

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &OIDCProviderDataSource{}

func NewIAMOIDCProviderDataSource() datasource.DataSource {
	return &OIDCProviderDataSource{}
}

// OIDCProviderDataSource defines the data source implementation.
type OIDCProviderDataSource struct {
	client    *RadosgwClient
	iamClient *IAMClient
}

// OIDCProviderDataSourceModel describes the data source data model.
type OIDCProviderDataSourceModel struct {
	ARN            types.String `tfsdk:"arn"`
	URL            types.String `tfsdk:"url"`
	ClientIDList   types.Set    `tfsdk:"client_id_list"`
	ThumbprintList types.List   `tfsdk:"thumbprint_list"`
}

// XML response structures for ListOpenIDConnectProviders
type listOIDCProvidersResponseXML struct {
	XMLName xml.Name                `xml:"ListOpenIDConnectProvidersResponse"`
	Result  listOIDCProvidersResult `xml:"ListOpenIDConnectProvidersResult"`
}

type listOIDCProvidersResult struct {
	OpenIDConnectProviderList struct {
		Members []oidcProviderListEntry `xml:"member"`
	} `xml:"OpenIDConnectProviderList"`
}

type oidcProviderListEntry struct {
	Arn string `xml:"Arn"`
}

func (d *OIDCProviderDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_openid_connect_provider"
}

func (d *OIDCProviderDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about an OpenID Connect (OIDC) identity provider in RadosGW. " +
			"Use this data source to get details about an existing OIDC provider by its ARN or URL.\n\n" +
			"~> **Note:** RadosGW normalizes OIDC provider URLs by stripping the protocol (`http://`, `https://`) " +
			"when constructing ARNs. This means `http://example.com` and `https://example.com` refer to the " +
			"**same** OIDC provider with ARN `arn:aws:iam:::oidc-provider/example.com`. You cannot have " +
			"separate providers for the same domain with different protocols.",

		Attributes: map[string]schema.Attribute{
			"arn": schema.StringAttribute{
				MarkdownDescription: "Amazon Resource Name (ARN) of the OIDC provider. " +
					"Format: `arn:aws:iam:::oidc-provider/<url>` (URL portion excludes protocol). " +
					"Exactly one of `arn` or `url` must be specified.",
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot("arn"), path.MatchRoot("url")),
				},
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "URL of the identity provider. This value corresponds to the `iss` claim in OIDC tokens. " +
					"Can be specified with or without the protocol prefix (`http://` or `https://`). " +
					"The protocol is stripped for matching against provider ARNs. " +
					"Exactly one of `arn` or `url` must be specified.",
				Optional: true,
				Computed: true,
			},
			"client_id_list": schema.SetAttribute{
				MarkdownDescription: "List of client IDs (also known as audiences) that are registered with this provider. " +
					"These values correspond to the `aud` claim in OIDC tokens.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"thumbprint_list": schema.ListAttribute{
				MarkdownDescription: "List of certificate thumbprints for the OpenID Connect provider's IDP certificate(s). " +
					"Each thumbprint is a hex-encoded SHA-1 hash (40 characters).",
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *OIDCProviderDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.iamClient = NewIAMClient(
		client.Admin.Endpoint,
		client.Admin.AccessKey,
		client.Admin.SecretKey,
		client.Admin.HTTPClient,
	)
}

func (d *OIDCProviderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config OIDCProviderDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var arn string

	if !config.ARN.IsNull() && config.ARN.ValueString() != "" {
		// Lookup by ARN
		arn = config.ARN.ValueString()
	} else if !config.URL.IsNull() && config.URL.ValueString() != "" {
		// Lookup by URL - need to find the ARN first
		providerURL := config.URL.ValueString()

		// Normalize URL - remove https:// prefix if present
		providerURL = strings.TrimPrefix(providerURL, "https://")
		providerURL = strings.TrimPrefix(providerURL, "http://")

		foundArn, err := d.findOIDCProviderByURL(ctx, providerURL)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Finding OIDC Provider",
				fmt.Sprintf("Could not find OIDC provider with URL %s: %s", providerURL, err.Error()),
			)
			return
		}
		arn = foundArn
	} else {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"Either 'arn' or 'url' must be specified.",
		)
		return
	}

	// Get the OIDC provider details
	params := url.Values{}
	params.Set("Action", "GetOpenIDConnectProvider")
	params.Set("OpenIDConnectProviderArn", arn)

	body, err := d.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			resp.Diagnostics.AddError(
				"OIDC Provider Not Found",
				fmt.Sprintf("OIDC provider with ARN %s does not exist.", arn),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading OIDC Provider",
			fmt.Sprintf("Could not read OIDC provider %s: %s", arn, err.Error()),
		)
		return
	}

	var response getOIDCProviderResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Response",
			fmt.Sprintf("Could not parse GetOpenIDConnectProvider response: %s", err.Error()),
		)
		return
	}

	// Populate the model
	config.ARN = types.StringValue(arn)
	config.URL = types.StringValue(response.Result.URL)

	clientIDSet, diags := types.SetValueFrom(ctx, types.StringType, response.Result.ClientIDList.Members)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.ClientIDList = clientIDSet

	thumbprintList, diags := types.ListValueFrom(ctx, types.StringType, response.Result.ThumbprintList.Members)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.ThumbprintList = thumbprintList

	tflog.Trace(ctx, "Read OIDC provider data source", map[string]any{
		"arn": arn,
		"url": response.Result.URL,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// findOIDCProviderByURL lists all OIDC providers and finds one matching the given URL.
func (d *OIDCProviderDataSource) findOIDCProviderByURL(ctx context.Context, targetURL string) (string, error) {
	params := url.Values{}
	params.Set("Action", "ListOpenIDConnectProviders")

	body, err := d.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		return "", fmt.Errorf("failed to list OIDC providers: %w", err)
	}

	var response listOIDCProvidersResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse ListOpenIDConnectProviders response: %w", err)
	}

	// Normalize target URL for comparison
	normalizedTarget := strings.ToLower(strings.TrimSuffix(targetURL, "/"))

	for _, provider := range response.Result.OpenIDConnectProviderList.Members {
		// Extract URL from ARN: arn:aws:iam:::oidc-provider/<url>
		providerURL := urlFromOIDCProviderARN(provider.Arn)
		normalizedProvider := strings.ToLower(strings.TrimSuffix(providerURL, "/"))

		if normalizedProvider == normalizedTarget {
			return provider.Arn, nil
		}
	}

	return "", fmt.Errorf("no OIDC provider found with URL: %s", targetURL)
}

// urlFromOIDCProviderARN extracts the URL portion from an OIDC provider ARN.
// ARN format: arn:aws:iam:::oidc-provider/<url>
func urlFromOIDCProviderARN(arn string) string {
	parts := strings.SplitN(arn, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}
