package provider

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &SNSTopicResource{}
var _ resource.ResourceWithImportState = &SNSTopicResource{}

func NewSNSTopicResource() resource.Resource {
	return &SNSTopicResource{}
}

// SNSTopicResource defines the resource implementation.
type SNSTopicResource struct {
	client    *RadosgwClient
	iamClient *IAMClient
}

// SNSTopicResourceModel describes the resource data model.
type SNSTopicResourceModel struct {
	// User-configurable attributes
	Name               types.String `tfsdk:"name"`
	PushEndpoint       types.String `tfsdk:"push_endpoint"`
	OpaqueData         types.String `tfsdk:"opaque_data"`
	Persistent         types.Bool   `tfsdk:"persistent"`
	VerifySSL          types.Bool   `tfsdk:"verify_ssl"`
	CloudEvents        types.Bool   `tfsdk:"cloudevents"`
	UseSSL             types.Bool   `tfsdk:"use_ssl"`
	CALocation         types.String `tfsdk:"ca_location"`
	Mechanism          types.String `tfsdk:"mechanism"`
	UserName           types.String `tfsdk:"user_name"`
	Password           types.String `tfsdk:"password"`
	AMQPExchange       types.String `tfsdk:"amqp_exchange"`
	AMQPAckLevel       types.String `tfsdk:"amqp_ack_level"`
	KafkaAckLevel      types.String `tfsdk:"kafka_ack_level"`
	KafkaBrokers       types.String `tfsdk:"kafka_brokers"`
	TimeToLive         types.Int64  `tfsdk:"time_to_live"`
	MaxRetries         types.Int64  `tfsdk:"max_retries"`
	RetrySleepDuration types.Int64  `tfsdk:"retry_sleep_duration"`

	// Computed attributes
	ARN  types.String `tfsdk:"arn"`
	User types.String `tfsdk:"user"`
}

// =============================================================================
// XML Response Types for RadosGW SNS API
// =============================================================================

type createTopicResponseXML struct {
	XMLName xml.Name `xml:"CreateTopicResponse"`
	Result  struct {
		TopicArn string `xml:"TopicArn"`
	} `xml:"CreateTopicResult"`
}

type getTopicAttributesResponseXML struct {
	XMLName xml.Name `xml:"GetTopicAttributesResponse"`
	Result  struct {
		Attributes struct {
			Entries []snsAttributeEntry `xml:"entry"`
		} `xml:"Attributes"`
	} `xml:"GetTopicAttributesResult"`
}

type snsAttributeEntry struct {
	Key   string `xml:"key"`
	Value string `xml:"value"`
}

// snsTopicAttributes holds the parsed top-level attributes from GetTopicAttributes.
type snsTopicAttributes struct {
	User       string
	Name       string
	TopicArn   string
	OpaqueData string
	EndPoint   string // JSON-encoded snsEndpointInfo
}

// snsEndpointInfo represents the JSON structure embedded in the EndPoint attribute.
type snsEndpointInfo struct {
	EndpointAddress    string `json:"EndpointAddress"`
	EndpointArgs       string `json:"EndpointArgs"`
	EndpointTopic      string `json:"EndpointTopic"`
	HasStoredSecret    bool   `json:"HasStoredSecret"`
	Persistent         bool   `json:"Persistent"`
	TimeToLive         string `json:"TimeToLive"`
	MaxRetries         string `json:"MaxRetries"`
	RetrySleepDuration string `json:"RetrySleepDuration"`
}

// =============================================================================
// Resource Interface Methods
// =============================================================================

func (r *SNSTopicResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sns_topic"
}

func (r *SNSTopicResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an SNS topic in RadosGW for bucket notifications. " +
			"Topics define push endpoints where bucket event notifications are sent. " +
			"Supports HTTP, AMQP 0.9.1, and Kafka endpoints.\n\n" +
			"~> **Note:** Updating a topic uses `CreateTopic` as an upsert, which replaces **all** " +
			"topic attributes in a single API call. The provider automatically preserves any " +
			"existing topic policy (managed by `radosgw_sns_topic_policy`) through updates. " +
			"However, updating a topic may require re-creating any bucket notifications " +
			"associated with it. See the " +
			"[Ceph Bucket Notifications documentation](https://docs.ceph.com/en/latest/radosgw/notifications/) for details.\n\n" +
			"~> **Ceph Reef (18.x) compatibility:** On Ceph Reef, the `GetTopicAttributes` API returns a " +
			"limited set of attributes. The provider automatically preserves configured values for " +
			"attributes that the API does not return (e.g., `user`, `time_to_live`, `max_retries`, " +
			"`retry_sleep_duration`, and endpoint arguments). These attributes may appear empty " +
			"when importing a topic on Reef.",

		Attributes: map[string]schema.Attribute{
			// ---- Required ----
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the topic. Must be unique per tenant. " +
					"Changing the name forces creation of a new topic.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			// ---- Optional: general ----
			"push_endpoint": schema.StringAttribute{
				MarkdownDescription: "The URI of the endpoint to send push notifications to. " +
					"Supported protocols:\n" +
					"  - HTTP: `http[s]://<fqdn>[:<port>]`\n" +
					"  - AMQP 0.9.1: `amqp[s]://[<user>:<password>@]<fqdn>[:<port>][/<vhost>]`\n" +
					"  - Kafka: `kafka://[<user>:<password>@]<fqdn>[:<port>]`",
				Optional: true,
			},
			"opaque_data": schema.StringAttribute{
				MarkdownDescription: "Opaque data set in the topic configuration and added to all " +
					"notifications triggered by the topic.",
				Optional: true,
			},
			"persistent": schema.BoolAttribute{
				MarkdownDescription: "Whether notifications to this endpoint are persistent " +
					"(asynchronous). Persistent notifications are committed to storage and " +
					"retried until delivered. Default is `false`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},

			// ---- Optional: TLS / transport ----
			"verify_ssl": schema.BoolAttribute{
				MarkdownDescription: "Whether the server certificate is validated by the " +
					"client. Default is `true`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"cloudevents": schema.BoolAttribute{
				MarkdownDescription: "Whether HTTP headers should include attributes according " +
					"to the [S3 CloudEvents Spec](https://github.com/cloudevents/spec/blob/main/cloudevents/adapters/aws-s3.md). " +
					"Only applicable to HTTP endpoints. Default is `false`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"use_ssl": schema.BoolAttribute{
				MarkdownDescription: "Whether a secure connection is used to connect to the " +
					"broker. Applicable to Kafka and AMQP endpoints. Default is `false`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"ca_location": schema.StringAttribute{
				MarkdownDescription: "Path to a PEM-encoded CA certificate file for " +
					"authenticating the broker when using SSL.",
				Optional: true,
			},

			// ---- Optional: authentication ----
			"mechanism": schema.StringAttribute{
				MarkdownDescription: "SASL mechanism for Kafka authentication. " +
					"Supported values: `PLAIN`, `SCRAM-SHA-256`, `SCRAM-SHA-512`, " +
					"`GSSAPI`, `OAUTHBEARER`. Default is `PLAIN`.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						"PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512",
						"GSSAPI", "OAUTHBEARER",
					),
				},
			},
			"user_name": schema.StringAttribute{
				MarkdownDescription: "Username for broker authentication. Overrides the " +
					"user in the endpoint URI if both are provided. Must be transmitted " +
					"over HTTPS.",
				Optional:  true,
				Sensitive: true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password for broker authentication. Overrides the " +
					"password in the endpoint URI if both are provided. Must be transmitted " +
					"over HTTPS.",
				Optional:  true,
				Sensitive: true,
			},

			// ---- Optional: AMQP-specific ----
			"amqp_exchange": schema.StringAttribute{
				MarkdownDescription: "The AMQP exchange name. The exchange must exist and " +
					"be able to route messages based on topics. Required for AMQP endpoints.",
				Optional: true,
			},
			"amqp_ack_level": schema.StringAttribute{
				MarkdownDescription: "AMQP acknowledgement level. Valid values: `none`, " +
					"`broker`, `routable`. Default is `broker`.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf("none", "broker", "routable"),
				},
			},

			// ---- Optional: Kafka-specific ----
			"kafka_ack_level": schema.StringAttribute{
				MarkdownDescription: "Kafka acknowledgement level. Valid values: `none`, " +
					"`broker`. Default is `broker`.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf("none", "broker"),
				},
			},
			"kafka_brokers": schema.StringAttribute{
				MarkdownDescription: "Comma-separated list of Kafka brokers in `host:port` " +
					"format. Added to the Kafka URI to support Kafka clusters.",
				Optional: true,
			},

			// ---- Optional: persistence settings ----
			"time_to_live": schema.Int64Attribute{
				MarkdownDescription: "Maximum time in seconds to retain notifications. " +
					"Only applicable when `persistent` is `true`. Zero means infinite. " +
					"Not returned by `GetTopicAttributes` on Ceph Reef (18.x); the configured value is preserved in state.",
				Optional: true,
			},
			"max_retries": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of retries before expiring a notification. " +
					"Only applicable when `persistent` is `true`. Zero means infinite. " +
					"Not returned by `GetTopicAttributes` on Ceph Reef (18.x); the configured value is preserved in state.",
				Optional: true,
			},
			"retry_sleep_duration": schema.Int64Attribute{
				MarkdownDescription: "Time in seconds between notification delivery retries. " +
					"Only applicable when `persistent` is `true`. Zero means no delay. " +
					"Not returned by `GetTopicAttributes` on Ceph Reef (18.x); the configured value is preserved in state.",
				Optional: true,
			},

			// ---- Computed ----
			"arn": schema.StringAttribute{
				MarkdownDescription: "The ARN (Amazon Resource Name) of the topic in the format " +
					"`arn:aws:sns:<zone-group>:<tenant>:<topic>`.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user": schema.StringAttribute{
				MarkdownDescription: "The name of the user that created the topic. " +
					"Not returned by Ceph Reef (18.x).",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *SNSTopicResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*RadosgwClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *RadosgwClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
	r.iamClient = NewIAMClient(
		client.Admin.Endpoint,
		client.Admin.AccessKey,
		client.Admin.SecretKey,
		client.Admin.HTTPClient,
	)
}

func (r *SNSTopicResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SNSTopicResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := buildSNSTopicParams(plan)
	params.Set("Action", "CreateTopic")
	params.Set("Name", plan.Name.ValueString())

	body, err := r.iamClient.DoPostRequest(ctx, params, "sns")
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating SNS Topic",
			fmt.Sprintf("Could not create topic %s: %s", plan.Name.ValueString(), err.Error()),
		)
		return
	}

	var response createTopicResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Response",
			fmt.Sprintf("Could not parse CreateTopic response: %s", err.Error()),
		)
		return
	}

	plan.ARN = types.StringValue(response.Result.TopicArn)

	// Read back to populate computed fields (user, etc.)
	topicAttrs, err := r.readTopic(ctx, plan.ARN.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Created Topic",
			fmt.Sprintf("Topic was created but could not be read back: %s", err.Error()),
		)
		return
	}

	plan.User = types.StringValue(topicAttrs.User)
	// On older Ceph (Reef), User may not be returned by GetTopicAttributes
	if topicAttrs.User == "" {
		plan.User = types.StringNull()
	}

	tflog.Trace(ctx, "Created SNS topic", map[string]any{
		"name": plan.Name.ValueString(),
		"arn":  plan.ARN.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SNSTopicResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SNSTopicResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Preserve sensitive fields that are not returned by the API
	currentUserName := state.UserName
	currentPassword := state.Password

	// Save current state for fallback when the API doesn't return certain
	// attributes. Older Ceph versions (e.g. Reef) return a limited set of
	// fields from GetTopicAttributes.
	prevState := state

	topicAttrs, err := r.readTopic(ctx, state.ARN.ValueString())
	if err != nil {
		if isSNSTopicNotFound(err) {
			tflog.Info(ctx, "SNS topic not found, removing from state", map[string]any{
				"arn": state.ARN.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading SNS Topic",
			fmt.Sprintf("Could not read topic %s: %s", state.ARN.ValueString(), err.Error()),
		)
		return
	}

	// Parse the nested EndPoint JSON and its EndpointArgs query string
	endpointInfo, endpointArgs, err := parseSNSEndpointInfo(topicAttrs.EndPoint)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Topic Endpoint",
			fmt.Sprintf("Could not parse topic endpoint info: %s", err.Error()),
		)
		return
	}

	// Map API response to model

	// Top-level attributes
	state.Name = types.StringValue(topicAttrs.Name)
	state.ARN = types.StringValue(topicAttrs.TopicArn)

	// User may be empty on older Ceph versions (Reef)
	if topicAttrs.User != "" {
		state.User = types.StringValue(topicAttrs.User)
	} else {
		state.User = prevState.User
	}

	// Push endpoint
	if endpointInfo.EndpointAddress != "" {
		state.PushEndpoint = types.StringValue(endpointInfo.EndpointAddress)
	} else {
		state.PushEndpoint = types.StringNull()
	}

	// Opaque data
	if topicAttrs.OpaqueData != "" {
		state.OpaqueData = types.StringValue(topicAttrs.OpaqueData)
	} else {
		state.OpaqueData = types.StringNull()
	}

	// Persistent flag (from EndPoint JSON, not EndpointArgs)
	state.Persistent = types.BoolValue(endpointInfo.Persistent)

	// Persistence settings (from EndPoint JSON).
	// On older Ceph (Reef), these fields may not be present in the JSON —
	// preserve state values so that the plan does not drift.
	if v := parseSNSOptionalInt(endpointInfo.TimeToLive); !v.IsNull() {
		state.TimeToLive = v
	} else {
		state.TimeToLive = prevState.TimeToLive
	}
	if v := parseSNSOptionalInt(endpointInfo.MaxRetries); !v.IsNull() {
		state.MaxRetries = v
	} else {
		state.MaxRetries = prevState.MaxRetries
	}
	if v := parseSNSOptionalInt(endpointInfo.RetrySleepDuration); !v.IsNull() {
		state.RetrySleepDuration = v
	} else {
		state.RetrySleepDuration = prevState.RetrySleepDuration
	}

	// Booleans from EndpointArgs — preserve state when key is absent (Reef compat)
	state.VerifySSL = parseSNSBoolArgPreserve(endpointArgs, "verify-ssl", prevState.VerifySSL)
	state.CloudEvents = parseSNSBoolArgPreserve(endpointArgs, "cloudevents", prevState.CloudEvents)
	state.UseSSL = parseSNSBoolArgPreserve(endpointArgs, "use-ssl", prevState.UseSSL)

	// Strings from EndpointArgs — preserve state when key is absent (Reef compat)
	state.CALocation = parseSNSStringArgPreserve(endpointArgs, "ca-location", prevState.CALocation)
	state.Mechanism = parseSNSStringArgPreserve(endpointArgs, "mechanism", prevState.Mechanism)
	state.AMQPExchange = parseSNSStringArgPreserve(endpointArgs, "amqp-exchange", prevState.AMQPExchange)
	state.AMQPAckLevel = parseSNSStringArgPreserve(endpointArgs, "amqp-ack-level", prevState.AMQPAckLevel)
	state.KafkaAckLevel = parseSNSStringArgPreserve(endpointArgs, "kafka-ack-level", prevState.KafkaAckLevel)
	state.KafkaBrokers = parseSNSStringArgPreserve(endpointArgs, "kafka-brokers", prevState.KafkaBrokers)

	// Preserve sensitive fields (not returned by the API)
	state.UserName = currentUserName
	state.Password = currentPassword

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SNSTopicResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state SNSTopicResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read the existing policy before the upsert — CreateTopic replaces ALL
	// attributes, so an omitted Policy would be reset to empty.  Including
	// it in the CreateTopic call preserves it atomically.
	existingPolicy := ""
	preAttrs, err := r.readTopicAllAttributes(ctx, state.ARN.ValueString())
	if err == nil {
		existingPolicy = preAttrs["Policy"]
	}

	// Use CreateTopic as upsert — this atomically sets all topic attributes.
	params := buildSNSTopicParams(plan)
	params.Set("Action", "CreateTopic")
	params.Set("Name", plan.Name.ValueString())

	// Include existing policy in the upsert so it is not cleared.
	if existingPolicy != "" {
		idx := nextAttributeIndex(params)
		params.Set(fmt.Sprintf("Attributes.entry.%d.key", idx), "Policy")
		params.Set(fmt.Sprintf("Attributes.entry.%d.value", idx), existingPolicy)
	}

	body, err := r.iamClient.DoPostRequest(ctx, params, "sns")
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating SNS Topic",
			fmt.Sprintf("Could not update topic %s: %s", plan.Name.ValueString(), err.Error()),
		)
		return
	}

	var response createTopicResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Response",
			fmt.Sprintf("Could not parse CreateTopic response: %s", err.Error()),
		)
		return
	}

	// Preserve computed fields
	plan.ARN = types.StringValue(response.Result.TopicArn)
	plan.User = state.User

	tflog.Debug(ctx, "Updated SNS topic", map[string]any{
		"name": plan.Name.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SNSTopicResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SNSTopicResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("Action", "DeleteTopic")
	params.Set("TopicArn", state.ARN.ValueString())

	_, err := r.iamClient.DoPostRequest(ctx, params, "sns")
	if err != nil {
		// Deleting an already-deleted topic is not considered an error by RadosGW
		if isSNSTopicNotFound(err) {
			tflog.Info(ctx, "SNS topic already deleted", map[string]any{
				"arn": state.ARN.ValueString(),
			})
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting SNS Topic",
			fmt.Sprintf("Could not delete topic %s: %s", state.ARN.ValueString(), err.Error()),
		)
		return
	}

	tflog.Trace(ctx, "Deleted SNS topic", map[string]any{
		"arn": state.ARN.ValueString(),
	})
}

func (r *SNSTopicResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("arn"), req, resp)
}

// =============================================================================
// Helper Functions
// =============================================================================

// buildSNSTopicParams converts the resource model into CreateTopic
// Attributes.entry.N.key/value query parameters.
func buildSNSTopicParams(model SNSTopicResourceModel) url.Values {
	params := url.Values{}
	idx := 1

	addAttr := func(key, value string) {
		params.Set(fmt.Sprintf("Attributes.entry.%d.key", idx), key)
		params.Set(fmt.Sprintf("Attributes.entry.%d.value", idx), value)
		idx++
	}

	// General attributes
	if !model.PushEndpoint.IsNull() && model.PushEndpoint.ValueString() != "" {
		addAttr("push-endpoint", model.PushEndpoint.ValueString())
	}
	if !model.OpaqueData.IsNull() && model.OpaqueData.ValueString() != "" {
		addAttr("OpaqueData", model.OpaqueData.ValueString())
	}
	if !model.Persistent.IsNull() {
		addAttr("persistent", strconv.FormatBool(model.Persistent.ValueBool()))
	}

	// TLS / transport
	if !model.VerifySSL.IsNull() {
		addAttr("verify-ssl", strconv.FormatBool(model.VerifySSL.ValueBool()))
	}
	if !model.CloudEvents.IsNull() {
		addAttr("cloudevents", strconv.FormatBool(model.CloudEvents.ValueBool()))
	}
	if !model.UseSSL.IsNull() {
		addAttr("use-ssl", strconv.FormatBool(model.UseSSL.ValueBool()))
	}
	if !model.CALocation.IsNull() && model.CALocation.ValueString() != "" {
		addAttr("ca-location", model.CALocation.ValueString())
	}

	// Authentication
	if !model.Mechanism.IsNull() && model.Mechanism.ValueString() != "" {
		addAttr("mechanism", model.Mechanism.ValueString())
	}
	if !model.UserName.IsNull() && model.UserName.ValueString() != "" {
		addAttr("user-name", model.UserName.ValueString())
	}
	if !model.Password.IsNull() && model.Password.ValueString() != "" {
		addAttr("password", model.Password.ValueString())
	}

	// AMQP-specific
	if !model.AMQPExchange.IsNull() && model.AMQPExchange.ValueString() != "" {
		addAttr("amqp-exchange", model.AMQPExchange.ValueString())
	}
	if !model.AMQPAckLevel.IsNull() && model.AMQPAckLevel.ValueString() != "" {
		addAttr("amqp-ack-level", model.AMQPAckLevel.ValueString())
	}

	// Kafka-specific
	if !model.KafkaAckLevel.IsNull() && model.KafkaAckLevel.ValueString() != "" {
		addAttr("kafka-ack-level", model.KafkaAckLevel.ValueString())
	}
	if !model.KafkaBrokers.IsNull() && model.KafkaBrokers.ValueString() != "" {
		addAttr("kafka-brokers", model.KafkaBrokers.ValueString())
	}

	// Persistence settings
	if !model.TimeToLive.IsNull() {
		addAttr("time_to_live", strconv.FormatInt(model.TimeToLive.ValueInt64(), 10))
	}
	if !model.MaxRetries.IsNull() {
		addAttr("max_retries", strconv.FormatInt(model.MaxRetries.ValueInt64(), 10))
	}
	if !model.RetrySleepDuration.IsNull() {
		addAttr("retry_sleep_duration", strconv.FormatInt(model.RetrySleepDuration.ValueInt64(), 10))
	}

	return params
}

// nextAttributeIndex finds the next unused Attributes.entry.N index in
// the given url.Values.  Used to append extra attributes (e.g. Policy)
// after buildSNSTopicParams has already populated the params.
func nextAttributeIndex(params url.Values) int {
	idx := 1
	for params.Get(fmt.Sprintf("Attributes.entry.%d.key", idx)) != "" {
		idx++
	}
	return idx
}

// readTopic calls GetTopicAttributes and returns parsed top-level attributes.
func (r *SNSTopicResource) readTopic(ctx context.Context, arn string) (*snsTopicAttributes, error) {
	params := url.Values{}
	params.Set("Action", "GetTopicAttributes")
	params.Set("TopicArn", arn)

	body, err := r.iamClient.DoPostRequest(ctx, params, "sns")
	if err != nil {
		return nil, err
	}

	var response getTopicAttributesResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("could not parse GetTopicAttributes response: %w", err)
	}

	// Convert entries to map
	attrs := make(map[string]string)
	for _, entry := range response.Result.Attributes.Entries {
		attrs[entry.Key] = entry.Value
	}

	return &snsTopicAttributes{
		User:       attrs["User"],
		Name:       attrs["Name"],
		TopicArn:   attrs["TopicArn"],
		OpaqueData: attrs["OpaqueData"],
		EndPoint:   attrs["EndPoint"],
	}, nil
}

// readTopicAllAttributes calls GetTopicAttributes and returns all key-value
// pairs, including Policy. Used by Update to preserve the policy through
// CreateTopic upserts.
func (r *SNSTopicResource) readTopicAllAttributes(ctx context.Context, arn string) (map[string]string, error) {
	params := url.Values{}
	params.Set("Action", "GetTopicAttributes")
	params.Set("TopicArn", arn)

	body, err := r.iamClient.DoPostRequest(ctx, params, "sns")
	if err != nil {
		return nil, err
	}

	var response getTopicAttributesResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("could not parse GetTopicAttributes response: %w", err)
	}

	attrs := make(map[string]string)
	for _, entry := range response.Result.Attributes.Entries {
		attrs[entry.Key] = entry.Value
	}
	return attrs, nil
}

// parseSNSEndpointInfo parses the JSON-encoded EndPoint attribute value into
// the endpoint info struct and a parsed EndpointArgs query string.
func parseSNSEndpointInfo(endpointJSON string) (*snsEndpointInfo, url.Values, error) {
	if endpointJSON == "" {
		return &snsEndpointInfo{}, url.Values{}, nil
	}

	var info snsEndpointInfo
	if err := json.Unmarshal([]byte(endpointJSON), &info); err != nil {
		return nil, nil, fmt.Errorf("failed to parse EndPoint JSON: %w", err)
	}

	args, err := url.ParseQuery(info.EndpointArgs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse EndpointArgs: %w", err)
	}

	return &info, args, nil
}

// parseSNSBoolArg extracts a boolean value from EndpointArgs.
// Returns defaultVal when the key is absent (meaning the server default applies).
func parseSNSBoolArg(args url.Values, key string, defaultVal bool) types.Bool {
	v := args.Get(key)
	if v == "" {
		return types.BoolValue(defaultVal)
	}
	return types.BoolValue(v == "true")
}

// parseSNSBoolArgPreserve extracts a boolean from EndpointArgs.
// If the key is absent (older Ceph versions may not return all args),
// preserves the provided state value instead of falling back to a default.
func parseSNSBoolArgPreserve(args url.Values, key string, stateVal types.Bool) types.Bool {
	if !args.Has(key) {
		return stateVal
	}
	return types.BoolValue(args.Get(key) == "true")
}

// parseSNSStringArg extracts a string value from EndpointArgs.
// Returns types.StringNull() if the key is absent or empty.
func parseSNSStringArg(args url.Values, key string) types.String {
	v := args.Get(key)
	if v == "" {
		return types.StringNull()
	}
	return types.StringValue(v)
}

// parseSNSStringArgPreserve extracts a string from EndpointArgs.
// If the key is absent, preserves the provided state value.
func parseSNSStringArgPreserve(args url.Values, key string, stateVal types.String) types.String {
	if !args.Has(key) {
		return stateVal
	}
	v := args.Get(key)
	if v == "" {
		return types.StringNull()
	}
	return types.StringValue(v)
}

// parseSNSOptionalInt parses an optional integer field from the EndPoint JSON.
// RadosGW returns "None" for unset values and a numeric string otherwise.
func parseSNSOptionalInt(s string) types.Int64 {
	if s == "" || s == "None" {
		return types.Int64Null()
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return types.Int64Null()
	}
	return types.Int64Value(v)
}

// isSNSTopicNotFound checks whether the error indicates the topic does not exist.
func isSNSTopicNotFound(err error) bool {
	var iamErr *IAMError
	if !errors.As(err, &iamErr) {
		return false
	}
	return iamErr.Code == "NotFound" || iamErr.Code == "NoSuchEntity" || iamErr.StatusCode == 404
}
