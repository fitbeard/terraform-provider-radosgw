package provider

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &SNSTopicDataSource{}

func NewSNSTopicDataSource() datasource.DataSource {
	return &SNSTopicDataSource{}
}

// SNSTopicDataSource defines the data source implementation.
type SNSTopicDataSource struct {
	client    *RadosgwClient
	iamClient *IAMClient
}

// SNSTopicDataSourceModel describes the data source data model.
type SNSTopicDataSourceModel struct {
	Name               types.String `tfsdk:"name"`
	ARN                types.String `tfsdk:"arn"`
	User               types.String `tfsdk:"user"`
	PushEndpoint       types.String `tfsdk:"push_endpoint"`
	OpaqueData         types.String `tfsdk:"opaque_data"`
	Persistent         types.Bool   `tfsdk:"persistent"`
	VerifySSL          types.Bool   `tfsdk:"verify_ssl"`
	CloudEvents        types.Bool   `tfsdk:"cloudevents"`
	UseSSL             types.Bool   `tfsdk:"use_ssl"`
	CALocation         types.String `tfsdk:"ca_location"`
	Mechanism          types.String `tfsdk:"mechanism"`
	AMQPExchange       types.String `tfsdk:"amqp_exchange"`
	AMQPAckLevel       types.String `tfsdk:"amqp_ack_level"`
	KafkaAckLevel      types.String `tfsdk:"kafka_ack_level"`
	KafkaBrokers       types.String `tfsdk:"kafka_brokers"`
	TimeToLive         types.Int64  `tfsdk:"time_to_live"`
	MaxRetries         types.Int64  `tfsdk:"max_retries"`
	RetrySleepDuration types.Int64  `tfsdk:"retry_sleep_duration"`
}

func (d *SNSTopicDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sns_topic"
}

func (d *SNSTopicDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Use this data source to get information about an existing SNS topic in RadosGW. " +
			"By using this data source, you can reference SNS topics without having to hard code ARNs.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the SNS topic to look up.",
				Required:            true,
			},
			"arn": schema.StringAttribute{
				MarkdownDescription: "The ARN of the SNS topic.",
				Computed:            true,
			},
			"user": schema.StringAttribute{
				MarkdownDescription: "The RadosGW user that owns the topic.",
				Computed:            true,
			},
			"push_endpoint": schema.StringAttribute{
				MarkdownDescription: "The push endpoint URL for the topic.",
				Computed:            true,
			},
			"opaque_data": schema.StringAttribute{
				MarkdownDescription: "Opaque data attached to the topic.",
				Computed:            true,
			},
			"persistent": schema.BoolAttribute{
				MarkdownDescription: "Whether the topic is persistent.",
				Computed:            true,
			},
			"verify_ssl": schema.BoolAttribute{
				MarkdownDescription: "Whether SSL certificate verification is enabled.",
				Computed:            true,
			},
			"cloudevents": schema.BoolAttribute{
				MarkdownDescription: "Whether CloudEvents format is enabled.",
				Computed:            true,
			},
			"use_ssl": schema.BoolAttribute{
				MarkdownDescription: "Whether SSL is used for the endpoint connection.",
				Computed:            true,
			},
			"ca_location": schema.StringAttribute{
				MarkdownDescription: "Path to the CA certificate file for SSL verification.",
				Computed:            true,
			},
			"mechanism": schema.StringAttribute{
				MarkdownDescription: "Authentication mechanism (e.g., `plain`, `scram-sha-256`, `scram-sha-512`).",
				Computed:            true,
			},
			"amqp_exchange": schema.StringAttribute{
				MarkdownDescription: "The AMQP exchange name (for AMQP endpoints).",
				Computed:            true,
			},
			"amqp_ack_level": schema.StringAttribute{
				MarkdownDescription: "The AMQP acknowledgment level (`none`, `broker`, `routable`).",
				Computed:            true,
			},
			"kafka_ack_level": schema.StringAttribute{
				MarkdownDescription: "The Kafka acknowledgment level (`none`, `broker`).",
				Computed:            true,
			},
			"kafka_brokers": schema.StringAttribute{
				MarkdownDescription: "Comma-separated list of Kafka broker endpoints.",
				Computed:            true,
			},
			"time_to_live": schema.Int64Attribute{
				MarkdownDescription: "Time-to-live (in seconds) for persistent messages.",
				Computed:            true,
			},
			"max_retries": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of retries for failed deliveries.",
				Computed:            true,
			},
			"retry_sleep_duration": schema.Int64Attribute{
				MarkdownDescription: "Sleep duration (in seconds) between delivery retries.",
				Computed:            true,
			},
		},
	}
}

func (d *SNSTopicDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SNSTopicDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config SNSTopicDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	topicName := config.Name.ValueString()

	tflog.Debug(ctx, "Reading RadosGW SNS topic data source", map[string]any{
		"name": topicName,
	})

	// Construct the ARN from the topic name and call GetTopicAttributes
	topicARN := fmt.Sprintf("arn:aws:sns:default::%s", topicName)

	params := url.Values{}
	params.Set("Action", "GetTopicAttributes")
	params.Set("TopicArn", topicARN)

	body, err := d.iamClient.DoPostRequest(ctx, params, "sns")
	if err != nil {
		if isSNSTopicNotFound(err) {
			resp.Diagnostics.AddError(
				"SNS Topic Not Found",
				fmt.Sprintf("SNS topic with name %q does not exist.", topicName),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading SNS Topic",
			fmt.Sprintf("Could not read SNS topic %s: %s", topicName, err.Error()),
		)
		return
	}

	var response getTopicAttributesResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Response",
			fmt.Sprintf("Could not parse GetTopicAttributes response: %s", err.Error()),
		)
		return
	}

	// Convert entries to map
	attrs := make(map[string]string)
	for _, entry := range response.Result.Attributes.Entries {
		attrs[entry.Key] = entry.Value
	}

	// Parse the nested EndPoint JSON and its EndpointArgs query string
	endpointInfo, endpointArgs, err := parseSNSEndpointInfo(attrs["EndPoint"])
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Topic Endpoint",
			fmt.Sprintf("Could not parse topic endpoint info: %s", err.Error()),
		)
		return
	}

	// Map API response to model
	config.Name = types.StringValue(attrs["Name"])
	config.ARN = types.StringValue(attrs["TopicArn"])
	config.User = types.StringValue(attrs["User"])

	// Push endpoint
	if endpointInfo.EndpointAddress != "" {
		config.PushEndpoint = types.StringValue(endpointInfo.EndpointAddress)
	} else {
		config.PushEndpoint = types.StringNull()
	}

	// Opaque data
	if attrs["OpaqueData"] != "" {
		config.OpaqueData = types.StringValue(attrs["OpaqueData"])
	} else {
		config.OpaqueData = types.StringNull()
	}

	// Persistent flag (from EndPoint JSON)
	config.Persistent = types.BoolValue(endpointInfo.Persistent)

	// Persistence settings (from EndPoint JSON)
	config.TimeToLive = parseSNSOptionalInt(endpointInfo.TimeToLive)
	config.MaxRetries = parseSNSOptionalInt(endpointInfo.MaxRetries)
	config.RetrySleepDuration = parseSNSOptionalInt(endpointInfo.RetrySleepDuration)

	// Booleans from EndpointArgs
	config.VerifySSL = parseSNSBoolArg(endpointArgs, "verify-ssl", true)
	config.CloudEvents = parseSNSBoolArg(endpointArgs, "cloudevents", false)
	config.UseSSL = parseSNSBoolArg(endpointArgs, "use-ssl", false)

	// Strings from EndpointArgs
	config.CALocation = parseSNSStringArg(endpointArgs, "ca-location")
	config.Mechanism = parseSNSStringArg(endpointArgs, "mechanism")
	config.AMQPExchange = parseSNSStringArg(endpointArgs, "amqp-exchange")
	config.AMQPAckLevel = parseSNSStringArg(endpointArgs, "amqp-ack-level")
	config.KafkaAckLevel = parseSNSStringArg(endpointArgs, "kafka-ack-level")
	config.KafkaBrokers = parseSNSStringArg(endpointArgs, "kafka-brokers")

	tflog.Trace(ctx, "Read SNS topic data source", map[string]any{
		"name": attrs["Name"],
		"arn":  attrs["TopicArn"],
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
