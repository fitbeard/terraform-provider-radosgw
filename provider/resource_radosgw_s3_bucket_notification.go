package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &S3BucketNotificationResource{}
var _ resource.ResourceWithImportState = &S3BucketNotificationResource{}

func NewS3BucketNotificationResource() resource.Resource {
	return &S3BucketNotificationResource{}
}

// S3BucketNotificationResource defines the resource implementation.
type S3BucketNotificationResource struct {
	client *RadosgwClient
}

// S3BucketNotificationResourceModel describes the resource data model.
type S3BucketNotificationResourceModel struct {
	Bucket types.String `tfsdk:"bucket"`
	Topic  types.List   `tfsdk:"topic"`
}

// TopicConfigurationModel describes a single topic notification configuration.
type TopicConfigurationModel struct {
	ID           types.String `tfsdk:"id"`
	TopicARN     types.String `tfsdk:"topic_arn"`
	Events       types.Set    `tfsdk:"events"`
	FilterPrefix types.String `tfsdk:"filter_prefix"`
	FilterSuffix types.String `tfsdk:"filter_suffix"`
}

// =============================================================================
// Resource Interface Methods
// =============================================================================

func (r *S3BucketNotificationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket_notification"
}

func (r *S3BucketNotificationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages S3 bucket notification configuration in RadosGW. " +
			"Bucket notifications send event information to SNS topic endpoints when " +
			"specific events occur on the bucket.\n\n" +
			"~> **Note:** S3 buckets only support a single notification configuration. " +
			"Declaring multiple `radosgw_s3_bucket_notification` resources for the same bucket " +
			"will cause a perpetual difference in configuration. Use multiple `topic` blocks " +
			"within a single resource to configure multiple notifications.\n\n" +
			"~> **Note:** RadosGW only supports SNS topic destinations for bucket notifications. " +
			"SQS, Lambda, and EventBridge destinations are not supported.",

		Attributes: map[string]schema.Attribute{
			"bucket": schema.StringAttribute{
				MarkdownDescription: "The name of the S3 bucket to configure notifications for.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},

		Blocks: map[string]schema.Block{
			"topic": schema.ListNestedBlock{
				MarkdownDescription: "Notification configuration for an SNS topic destination. " +
					"Multiple `topic` blocks can be specified to send different events or " +
					"filtered subsets of events to different topics.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Unique identifier for the notification configuration. " +
								"If not specified, a random ID is generated.",
							Optional: true,
							Computed: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"topic_arn": schema.StringAttribute{
							MarkdownDescription: "The ARN of the SNS topic to publish notifications to. " +
								"The topic must already exist.",
							Required: true,
						},
						"events": schema.SetAttribute{
							MarkdownDescription: "The S3 event types that trigger notifications. " +
								"Supported events include:\n" +
								"  - `s3:ObjectCreated:*` — all object creation events\n" +
								"  - `s3:ObjectCreated:Put`\n" +
								"  - `s3:ObjectCreated:Post`\n" +
								"  - `s3:ObjectCreated:Copy`\n" +
								"  - `s3:ObjectCreated:CompleteMultipartUpload`\n" +
								"  - `s3:ObjectRemoved:*` — all object removal events\n" +
								"  - `s3:ObjectRemoved:Delete`\n" +
								"  - `s3:ObjectRemoved:DeleteMarkerCreated`\n" +
								"  - `s3:ObjectLifecycle:Expiration:*` — lifecycle expiration (Ceph extension)\n" +
								"  - `s3:ObjectLifecycle:Expiration:Current`\n" +
								"  - `s3:ObjectLifecycle:Expiration:NonCurrent`\n" +
								"  - `s3:ObjectLifecycle:Expiration:DeleteMarker`\n" +
								"  - `s3:ObjectLifecycle:Expiration:AbortMultipartUpload`\n" +
								"  - `s3:ObjectLifecycle:Transition:Current`\n" +
								"  - `s3:ObjectLifecycle:Transition:NonCurrent`\n" +
								"  - `s3:ObjectSynced:*` — multisite sync events (Ceph extension)\n" +
								"  - `s3:ObjectRestore:*` — object restore events",
							Required:    true,
							ElementType: types.StringType,
						},
						"filter_prefix": schema.StringAttribute{
							MarkdownDescription: "Object key name prefix to filter notifications. " +
								"Only events for objects whose keys start with this prefix " +
								"will trigger a notification.",
							Optional: true,
						},
						"filter_suffix": schema.StringAttribute{
							MarkdownDescription: "Object key name suffix to filter notifications. " +
								"Only events for objects whose keys end with this suffix " +
								"will trigger a notification.",
							Optional: true,
						},
					},
				},
			},
		},
	}
}

func (r *S3BucketNotificationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
}

func (r *S3BucketNotificationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3BucketNotificationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := plan.Bucket.ValueString()

	// Build the notification configuration from the plan
	notifConfig, diags := r.buildNotificationConfiguration(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Put the notification configuration on the bucket
	_, err := r.client.S3.PutBucketNotificationConfiguration(ctx, &s3.PutBucketNotificationConfigurationInput{
		Bucket:                    aws.String(bucket),
		NotificationConfiguration: notifConfig,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Bucket Notification",
			fmt.Sprintf("Could not set notification configuration on bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	// Read back to populate computed fields (auto-generated IDs)
	output, err := r.client.S3.GetBucketNotificationConfiguration(ctx, &s3.GetBucketNotificationConfigurationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Bucket Notification",
			fmt.Sprintf("Notification was set but could not be read back from bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	// Flatten API response back to state
	plan.Bucket = types.StringValue(bucket)
	topicList, diags := flattenTopicConfigurations(ctx, output.TopicConfigurations)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Topic = topicList

	tflog.Trace(ctx, "Created S3 bucket notification", map[string]any{
		"bucket": bucket,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketNotificationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3BucketNotificationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := state.Bucket.ValueString()

	output, err := r.client.S3.GetBucketNotificationConfiguration(ctx, &s3.GetBucketNotificationConfigurationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		var apiErr smithy.APIError
		if ok := errors.As(err, &apiErr); ok {
			if apiErr.ErrorCode() == "NoSuchBucket" {
				tflog.Info(ctx, "Bucket not found, removing notification from state", map[string]any{
					"bucket": bucket,
				})
				resp.State.RemoveResource(ctx)
				return
			}
		}
		resp.Diagnostics.AddError(
			"Error Reading Bucket Notification",
			fmt.Sprintf("Could not read notification configuration from bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	// If no topic configurations exist, the resource has been removed out-of-band
	if len(output.TopicConfigurations) == 0 {
		tflog.Info(ctx, "No notification configurations found, removing from state", map[string]any{
			"bucket": bucket,
		})
		resp.State.RemoveResource(ctx)
		return
	}

	state.Bucket = types.StringValue(bucket)
	topicList, diags := flattenTopicConfigurations(ctx, output.TopicConfigurations)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Topic = topicList

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3BucketNotificationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan S3BucketNotificationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := plan.Bucket.ValueString()

	// Build the notification configuration from the plan
	notifConfig, diags := r.buildNotificationConfiguration(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Clear existing notifications first — RadosGW merges rather than replaces
	// topic configurations when the topic ARN changes.
	_, err := r.client.S3.PutBucketNotificationConfiguration(ctx, &s3.PutBucketNotificationConfigurationInput{
		Bucket:                    aws.String(bucket),
		NotificationConfiguration: &s3types.NotificationConfiguration{},
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Bucket Notification",
			fmt.Sprintf("Could not clear existing notification configuration on bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	// Put the new notification configuration
	_, err = r.client.S3.PutBucketNotificationConfiguration(ctx, &s3.PutBucketNotificationConfigurationInput{
		Bucket:                    aws.String(bucket),
		NotificationConfiguration: notifConfig,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Bucket Notification",
			fmt.Sprintf("Could not update notification configuration on bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	// Read back to get computed fields
	output, err := r.client.S3.GetBucketNotificationConfiguration(ctx, &s3.GetBucketNotificationConfigurationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Bucket Notification",
			fmt.Sprintf("Notification was updated but could not be read back from bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	plan.Bucket = types.StringValue(bucket)
	topicList, diags := flattenTopicConfigurations(ctx, output.TopicConfigurations)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Topic = topicList

	tflog.Debug(ctx, "Updated S3 bucket notification", map[string]any{
		"bucket": bucket,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketNotificationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3BucketNotificationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := state.Bucket.ValueString()

	// Delete by putting an empty notification configuration
	_, err := r.client.S3.PutBucketNotificationConfiguration(ctx, &s3.PutBucketNotificationConfigurationInput{
		Bucket:                    aws.String(bucket),
		NotificationConfiguration: &s3types.NotificationConfiguration{},
	})
	if err != nil {
		var apiErr smithy.APIError
		if ok := errors.As(err, &apiErr); ok {
			if apiErr.ErrorCode() == "NoSuchBucket" {
				tflog.Info(ctx, "Bucket already deleted, notification is gone", map[string]any{
					"bucket": bucket,
				})
				return
			}
		}
		resp.Diagnostics.AddError(
			"Error Deleting Bucket Notification",
			fmt.Sprintf("Could not remove notification configuration from bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	tflog.Trace(ctx, "Deleted S3 bucket notification", map[string]any{
		"bucket": bucket,
	})
}

func (r *S3BucketNotificationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("bucket"), req, resp)
}

// =============================================================================
// Helper Functions
// =============================================================================

// buildNotificationConfiguration converts the Terraform model into the S3 SDK
// NotificationConfiguration type.
func (r *S3BucketNotificationResource) buildNotificationConfiguration(ctx context.Context, model S3BucketNotificationResourceModel) (*s3types.NotificationConfiguration, diag.Diagnostics) {
	var diags diag.Diagnostics
	notifConfig := &s3types.NotificationConfiguration{}

	if model.Topic.IsNull() || model.Topic.IsUnknown() {
		return notifConfig, diags
	}

	var topicModels []TopicConfigurationModel
	diags.Append(model.Topic.ElementsAs(ctx, &topicModels, false)...)
	if diags.HasError() {
		return nil, diags
	}

	topicConfigs := make([]s3types.TopicConfiguration, 0, len(topicModels))
	for i, tm := range topicModels {
		tc := s3types.TopicConfiguration{
			TopicArn: aws.String(tm.TopicARN.ValueString()),
		}

		// ID — RadosGW requires this field to be present
		if !tm.ID.IsNull() && !tm.ID.IsUnknown() && tm.ID.ValueString() != "" {
			tc.Id = aws.String(tm.ID.ValueString())
		} else {
			tc.Id = aws.String(fmt.Sprintf("tf-s3-topic-%d", i))
		}

		// Events
		var events []string
		diags.Append(tm.Events.ElementsAs(ctx, &events, false)...)
		if diags.HasError() {
			return nil, diags
		}
		tc.Events = make([]s3types.Event, len(events))
		for i, e := range events {
			tc.Events[i] = s3types.Event(e)
		}

		// Filters
		filterRules := make([]s3types.FilterRule, 0, 2)
		if !tm.FilterPrefix.IsNull() && tm.FilterPrefix.ValueString() != "" {
			filterRules = append(filterRules, s3types.FilterRule{
				Name:  s3types.FilterRuleNamePrefix,
				Value: aws.String(tm.FilterPrefix.ValueString()),
			})
		}
		if !tm.FilterSuffix.IsNull() && tm.FilterSuffix.ValueString() != "" {
			filterRules = append(filterRules, s3types.FilterRule{
				Name:  s3types.FilterRuleNameSuffix,
				Value: aws.String(tm.FilterSuffix.ValueString()),
			})
		}
		if len(filterRules) > 0 {
			tc.Filter = &s3types.NotificationConfigurationFilter{
				Key: &s3types.S3KeyFilter{
					FilterRules: filterRules,
				},
			}
		}

		topicConfigs = append(topicConfigs, tc)
	}

	notifConfig.TopicConfigurations = topicConfigs
	return notifConfig, diags
}

// flattenTopicConfigurations converts the S3 API response into a Terraform
// types.List of TopicConfigurationModel objects.
func flattenTopicConfigurations(ctx context.Context, configs []s3types.TopicConfiguration) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	if len(configs) == 0 {
		return types.ListNull(topicConfigurationObjectType()), diags
	}

	topicObjects := make([]attr.Value, 0, len(configs))
	for _, tc := range configs {
		// Events
		eventValues := make([]attr.Value, len(tc.Events))
		for i, e := range tc.Events {
			eventValues[i] = types.StringValue(string(e))
		}
		eventsSet, setDiags := types.SetValue(types.StringType, eventValues)
		diags.Append(setDiags...)

		// Filter rules
		filterPrefix := types.StringNull()
		filterSuffix := types.StringNull()
		if tc.Filter != nil && tc.Filter.Key != nil {
			for _, rule := range tc.Filter.Key.FilterRules {
				switch rule.Name {
				case s3types.FilterRuleNamePrefix:
					filterPrefix = types.StringValue(aws.ToString(rule.Value))
				case s3types.FilterRuleNameSuffix:
					filterSuffix = types.StringValue(aws.ToString(rule.Value))
				}
			}
		}

		// ID
		idVal := types.StringNull()
		if tc.Id != nil && *tc.Id != "" {
			idVal = types.StringValue(aws.ToString(tc.Id))
		}

		obj, objDiags := types.ObjectValue(topicConfigurationAttrTypes(), map[string]attr.Value{
			"id":            idVal,
			"topic_arn":     types.StringValue(aws.ToString(tc.TopicArn)),
			"events":        eventsSet,
			"filter_prefix": filterPrefix,
			"filter_suffix": filterSuffix,
		})
		diags.Append(objDiags...)
		topicObjects = append(topicObjects, obj)
	}

	result, listDiags := types.ListValue(topicConfigurationObjectType(), topicObjects)
	diags.Append(listDiags...)
	return result, diags
}

// topicConfigurationAttrTypes returns the attribute types for a TopicConfigurationModel.
func topicConfigurationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":            types.StringType,
		"topic_arn":     types.StringType,
		"events":        types.SetType{ElemType: types.StringType},
		"filter_prefix": types.StringType,
		"filter_suffix": types.StringType,
	}
}

// topicConfigurationObjectType returns the Object type for a TopicConfigurationModel.
func topicConfigurationObjectType() attr.Type {
	return types.ObjectType{AttrTypes: topicConfigurationAttrTypes()}
}
