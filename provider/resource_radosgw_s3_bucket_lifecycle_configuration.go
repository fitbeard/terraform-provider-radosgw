package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
var _ resource.Resource = &BucketLifecycleResource{}
var _ resource.ResourceWithImportState = &BucketLifecycleResource{}

func NewS3BucketLifecycleResource() resource.Resource {
	return &BucketLifecycleResource{}
}

// BucketLifecycleResource defines the resource implementation.
type BucketLifecycleResource struct {
	client *RadosgwClient
}

// BucketLifecycleResourceModel describes the resource data model.
type BucketLifecycleResourceModel struct {
	Bucket types.String `tfsdk:"bucket"`
	Rule   types.List   `tfsdk:"rule"`
	ID     types.String `tfsdk:"id"`
}

// LifecycleRuleModel describes a lifecycle rule.
type LifecycleRuleModel struct {
	ID                             types.String `tfsdk:"id"`
	Status                         types.String `tfsdk:"status"`
	Filter                         types.List   `tfsdk:"filter"`
	Expiration                     types.List   `tfsdk:"expiration"`
	Transition                     types.List   `tfsdk:"transition"`
	NoncurrentVersionExpiration    types.List   `tfsdk:"noncurrent_version_expiration"`
	NoncurrentVersionTransition    types.List   `tfsdk:"noncurrent_version_transition"`
	AbortIncompleteMultipartUpload types.List   `tfsdk:"abort_incomplete_multipart_upload"`
}

// LifecycleFilterModel describes a lifecycle rule filter.
type LifecycleFilterModel struct {
	Prefix types.String `tfsdk:"prefix"`
	Tag    types.List   `tfsdk:"tag"`
	And    types.List   `tfsdk:"and"`
}

// LifecycleFilterAndModel describes the AND condition in a filter.
type LifecycleFilterAndModel struct {
	Prefix types.String `tfsdk:"prefix"`
	Tags   types.Map    `tfsdk:"tags"`
}

// LifecycleTagModel describes a tag filter.
type LifecycleTagModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

// LifecycleExpirationModel describes expiration settings.
type LifecycleExpirationModel struct {
	Days                      types.Int64 `tfsdk:"days"`
	ExpiredObjectDeleteMarker types.Bool  `tfsdk:"expired_object_delete_marker"`
}

// LifecycleTransitionModel describes transition settings.
type LifecycleTransitionModel struct {
	Days         types.Int64  `tfsdk:"days"`
	StorageClass types.String `tfsdk:"storage_class"`
}

// LifecycleNoncurrentVersionExpirationModel describes noncurrent version expiration.
type LifecycleNoncurrentVersionExpirationModel struct {
	NoncurrentDays          types.Int64 `tfsdk:"noncurrent_days"`
	NewerNoncurrentVersions types.Int64 `tfsdk:"newer_noncurrent_versions"`
}

// LifecycleNoncurrentVersionTransitionModel describes noncurrent version transition.
type LifecycleNoncurrentVersionTransitionModel struct {
	NoncurrentDays          types.Int64  `tfsdk:"noncurrent_days"`
	NewerNoncurrentVersions types.Int64  `tfsdk:"newer_noncurrent_versions"`
	StorageClass            types.String `tfsdk:"storage_class"`
}

// LifecycleAbortIncompleteMultipartUploadModel describes abort incomplete multipart upload settings.
type LifecycleAbortIncompleteMultipartUploadModel struct {
	DaysAfterInitiation types.Int64 `tfsdk:"days_after_initiation"`
}

func (r *BucketLifecycleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket_lifecycle_configuration"
}

func (r *BucketLifecycleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages lifecycle configuration for an S3 bucket in RadosGW.

Lifecycle rules allow you to define actions that RadosGW applies to objects during their lifetime. Common use cases include:
- Expiring (deleting) objects after a certain number of days
- Transitioning objects to different storage classes
- Cleaning up incomplete multipart uploads
- Managing noncurrent versions in versioned buckets

~> **Note:** RadosGW supports a subset of Amazon S3 lifecycle features. Some advanced filtering options (like object size filtering) may not be available. See the [Ceph documentation](https://docs.ceph.com/en/latest/radosgw/s3/) for details.

~> **Important:** Only one lifecycle configuration can exist per bucket. This resource will replace any existing lifecycle configuration.`,

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The resource identifier (bucket name).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"bucket": schema.StringAttribute{
				MarkdownDescription: "The name of the bucket to apply the lifecycle configuration to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"rule": schema.ListNestedBlock{
				MarkdownDescription: "A lifecycle rule for the bucket. At least one rule is required.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Unique identifier for the rule. Maximum 255 characters.",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.LengthBetween(1, 255),
							},
						},
						"status": schema.StringAttribute{
							MarkdownDescription: "Whether the rule is currently being applied. Valid values: `Enabled`, `Disabled`.",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.OneOf("Enabled", "Disabled"),
							},
						},
					},
					Blocks: map[string]schema.Block{
						"filter": schema.ListNestedBlock{
							MarkdownDescription: "Filter that identifies the objects to which the rule applies. If not specified, the rule applies to all objects in the bucket.",
							Validators: []validator.List{
								listvalidator.SizeAtMost(1),
							},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"prefix": schema.StringAttribute{
										MarkdownDescription: "Object key prefix that identifies one or more objects to which the rule applies.",
										Optional:            true,
									},
								},
								Blocks: map[string]schema.Block{
									"tag": schema.ListNestedBlock{
										MarkdownDescription: "A tag to filter objects. The rule applies only to objects that have the specified tag.",
										Validators: []validator.List{
											listvalidator.SizeAtMost(1),
										},
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"key": schema.StringAttribute{
													MarkdownDescription: "The tag key.",
													Required:            true,
												},
												"value": schema.StringAttribute{
													MarkdownDescription: "The tag value.",
													Required:            true,
												},
											},
										},
									},
									"and": schema.ListNestedBlock{
										MarkdownDescription: "A logical AND to combine multiple filter conditions. Use this to apply a rule to objects that match all specified conditions.",
										Validators: []validator.List{
											listvalidator.SizeAtMost(1),
										},
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"prefix": schema.StringAttribute{
													MarkdownDescription: "Object key prefix.",
													Optional:            true,
												},
												"tags": schema.MapAttribute{
													MarkdownDescription: "Map of tags that objects must have to match.",
													Optional:            true,
													ElementType:         types.StringType,
												},
											},
										},
									},
								},
							},
						},
						"expiration": schema.ListNestedBlock{
							MarkdownDescription: "Specifies when objects expire (are deleted).",
							Validators: []validator.List{
								listvalidator.SizeAtMost(1),
							},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"days": schema.Int64Attribute{
										MarkdownDescription: "Number of days after object creation when the object expires.",
										Optional:            true,
										Validators: []validator.Int64{
											int64validator.AtLeast(1),
										},
									},
									"expired_object_delete_marker": schema.BoolAttribute{
										MarkdownDescription: "Whether to remove expired object delete markers. Only valid for versioned buckets.",
										Optional:            true,
									},
								},
							},
						},
						"transition": schema.ListNestedBlock{
							MarkdownDescription: "Specifies when objects transition to a different storage class.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"days": schema.Int64Attribute{
										MarkdownDescription: "Number of days after object creation when the transition occurs.",
										Required:            true,
										Validators: []validator.Int64{
											int64validator.AtLeast(0),
										},
									},
									"storage_class": schema.StringAttribute{
										MarkdownDescription: "The storage class to transition objects to. The available storage classes depend on your RadosGW configuration.",
										Required:            true,
									},
								},
							},
						},
						"noncurrent_version_expiration": schema.ListNestedBlock{
							MarkdownDescription: "Specifies when noncurrent object versions expire. Only valid for versioned buckets.",
							Validators: []validator.List{
								listvalidator.SizeAtMost(1),
							},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"noncurrent_days": schema.Int64Attribute{
										MarkdownDescription: "Number of days after an object becomes noncurrent when it expires.",
										Required:            true,
										Validators: []validator.Int64{
											int64validator.AtLeast(1),
										},
									},
									"newer_noncurrent_versions": schema.Int64Attribute{
										MarkdownDescription: "Number of noncurrent versions to retain. If specified, the rule only applies after this many noncurrent versions exist.",
										Optional:            true,
										Validators: []validator.Int64{
											int64validator.AtLeast(1),
										},
									},
								},
							},
						},
						"noncurrent_version_transition": schema.ListNestedBlock{
							MarkdownDescription: "Specifies when noncurrent object versions transition to a different storage class. Only valid for versioned buckets.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"noncurrent_days": schema.Int64Attribute{
										MarkdownDescription: "Number of days after an object becomes noncurrent when the transition occurs.",
										Required:            true,
										Validators: []validator.Int64{
											int64validator.AtLeast(0),
										},
									},
									"newer_noncurrent_versions": schema.Int64Attribute{
										MarkdownDescription: "Number of noncurrent versions to retain before transitioning.",
										Optional:            true,
										Validators: []validator.Int64{
											int64validator.AtLeast(1),
										},
									},
									"storage_class": schema.StringAttribute{
										MarkdownDescription: "The storage class to transition noncurrent versions to.",
										Required:            true,
									},
								},
							},
						},
						"abort_incomplete_multipart_upload": schema.ListNestedBlock{
							MarkdownDescription: "Specifies when incomplete multipart uploads are aborted.",
							Validators: []validator.List{
								listvalidator.SizeAtMost(1),
							},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"days_after_initiation": schema.Int64Attribute{
										MarkdownDescription: "Number of days after initiating a multipart upload when it should be aborted.",
										Required:            true,
										Validators: []validator.Int64{
											int64validator.AtLeast(1),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *BucketLifecycleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *BucketLifecycleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BucketLifecycleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := plan.Bucket.ValueString()

	// Build lifecycle configuration
	lifecycleConfig, diags := r.buildLifecycleConfiguration(ctx, plan.Rule)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Put lifecycle configuration
	_, err := r.client.S3.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket:                 aws.String(bucket),
		LifecycleConfiguration: lifecycleConfig,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Bucket Lifecycle Configuration",
			fmt.Sprintf("Could not create lifecycle configuration for bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	plan.ID = types.StringValue(bucket)

	tflog.Trace(ctx, "Created bucket lifecycle configuration", map[string]any{
		"bucket": bucket,
	})

	// Read back the configuration and preserve rule order from plan
	output, err := r.client.S3.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Bucket Lifecycle Configuration After Create",
			fmt.Sprintf("Could not read lifecycle configuration for bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	// Extract rule IDs from plan to preserve order
	expectedOrder := r.extractRuleIDsFromList(ctx, plan.Rule)
	rules, ruleDiags := r.flattenLifecycleRules(ctx, output.Rules, expectedOrder)
	resp.Diagnostics.Append(ruleDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Rule = rules

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BucketLifecycleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BucketLifecycleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := state.Bucket.ValueString()

	// Get lifecycle configuration
	output, err := r.client.S3.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		var apiErr smithy.APIError
		if ok := errors.As(err, &apiErr); ok {
			if apiErr.ErrorCode() == "NoSuchLifecycleConfiguration" {
				tflog.Info(ctx, "Bucket lifecycle configuration not found, removing from state", map[string]any{
					"bucket": bucket,
				})
				resp.State.RemoveResource(ctx)
				return
			}
			if apiErr.ErrorCode() == "NoSuchBucket" {
				tflog.Info(ctx, "Bucket not found, removing from state", map[string]any{
					"bucket": bucket,
				})
				resp.State.RemoveResource(ctx)
				return
			}
		}
		resp.Diagnostics.AddError(
			"Error Reading Bucket Lifecycle Configuration",
			fmt.Sprintf("Could not read lifecycle configuration for bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	// Extract rule IDs from current state to preserve order
	expectedOrder := r.extractRuleIDsFromList(ctx, state.Rule)

	// Convert rules to Terraform state
	rules, diags := r.flattenLifecycleRules(ctx, output.Rules, expectedOrder)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Rule = rules
	state.ID = types.StringValue(bucket)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *BucketLifecycleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan BucketLifecycleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := plan.Bucket.ValueString()

	// Build lifecycle configuration
	lifecycleConfig, diags := r.buildLifecycleConfiguration(ctx, plan.Rule)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Put lifecycle configuration (replaces existing)
	_, err := r.client.S3.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket:                 aws.String(bucket),
		LifecycleConfiguration: lifecycleConfig,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Bucket Lifecycle Configuration",
			fmt.Sprintf("Could not update lifecycle configuration for bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	plan.ID = types.StringValue(bucket)

	tflog.Debug(ctx, "Updated bucket lifecycle configuration", map[string]any{
		"bucket": bucket,
	})

	// Read back the configuration and preserve rule order from plan
	output, err := r.client.S3.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Bucket Lifecycle Configuration After Update",
			fmt.Sprintf("Could not read lifecycle configuration for bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	// Extract rule IDs from plan to preserve order
	expectedOrder := r.extractRuleIDsFromList(ctx, plan.Rule)
	rules, ruleDiags := r.flattenLifecycleRules(ctx, output.Rules, expectedOrder)
	resp.Diagnostics.Append(ruleDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Rule = rules

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BucketLifecycleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BucketLifecycleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := state.Bucket.ValueString()

	// Delete lifecycle configuration
	_, err := r.client.S3.DeleteBucketLifecycle(ctx, &s3.DeleteBucketLifecycleInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		var apiErr smithy.APIError
		if ok := errors.As(err, &apiErr); ok {
			if apiErr.ErrorCode() == "NoSuchLifecycleConfiguration" || apiErr.ErrorCode() == "NoSuchBucket" {
				tflog.Info(ctx, "Bucket or lifecycle configuration already deleted", map[string]any{
					"bucket": bucket,
				})
				return
			}
		}
		resp.Diagnostics.AddError(
			"Error Deleting Bucket Lifecycle Configuration",
			fmt.Sprintf("Could not delete lifecycle configuration for bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	tflog.Trace(ctx, "Deleted bucket lifecycle configuration", map[string]any{
		"bucket": bucket,
	})
}

func (r *BucketLifecycleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("bucket"), req, resp)
}

// buildLifecycleConfiguration converts Terraform state to AWS SDK lifecycle configuration.
func (r *BucketLifecycleResource) buildLifecycleConfiguration(ctx context.Context, rulesList types.List) (*s3types.BucketLifecycleConfiguration, diag.Diagnostics) {
	var diags diag.Diagnostics

	if rulesList.IsNull() || rulesList.IsUnknown() {
		return nil, diags
	}

	var rules []LifecycleRuleModel
	diags.Append(rulesList.ElementsAs(ctx, &rules, false)...)
	if diags.HasError() {
		return nil, diags
	}

	var s3Rules []s3types.LifecycleRule
	for _, rule := range rules {
		s3Rule := s3types.LifecycleRule{
			ID:     aws.String(rule.ID.ValueString()),
			Status: s3types.ExpirationStatus(rule.Status.ValueString()),
		}

		// Build filter
		filter, filterDiags := r.buildLifecycleFilter(ctx, rule.Filter)
		diags.Append(filterDiags...)
		s3Rule.Filter = filter

		// Build expiration
		if !rule.Expiration.IsNull() && !rule.Expiration.IsUnknown() {
			var expirations []LifecycleExpirationModel
			diags.Append(rule.Expiration.ElementsAs(ctx, &expirations, false)...)
			if len(expirations) > 0 {
				exp := expirations[0]
				s3Exp := &s3types.LifecycleExpiration{}
				if !exp.Days.IsNull() {
					s3Exp.Days = aws.Int32(int32(exp.Days.ValueInt64()))
				}
				if !exp.ExpiredObjectDeleteMarker.IsNull() {
					s3Exp.ExpiredObjectDeleteMarker = aws.Bool(exp.ExpiredObjectDeleteMarker.ValueBool())
				}
				s3Rule.Expiration = s3Exp
			}
		}

		// Build transitions
		if !rule.Transition.IsNull() && !rule.Transition.IsUnknown() {
			var transitions []LifecycleTransitionModel
			diags.Append(rule.Transition.ElementsAs(ctx, &transitions, false)...)
			for _, t := range transitions {
				s3Rule.Transitions = append(s3Rule.Transitions, s3types.Transition{
					Days:         aws.Int32(int32(t.Days.ValueInt64())),
					StorageClass: s3types.TransitionStorageClass(t.StorageClass.ValueString()),
				})
			}
		}

		// Build noncurrent version expiration
		if !rule.NoncurrentVersionExpiration.IsNull() && !rule.NoncurrentVersionExpiration.IsUnknown() {
			var nves []LifecycleNoncurrentVersionExpirationModel
			diags.Append(rule.NoncurrentVersionExpiration.ElementsAs(ctx, &nves, false)...)
			if len(nves) > 0 {
				nve := nves[0]
				s3Nve := &s3types.NoncurrentVersionExpiration{
					NoncurrentDays: aws.Int32(int32(nve.NoncurrentDays.ValueInt64())),
				}
				if !nve.NewerNoncurrentVersions.IsNull() {
					s3Nve.NewerNoncurrentVersions = aws.Int32(int32(nve.NewerNoncurrentVersions.ValueInt64()))
				}
				s3Rule.NoncurrentVersionExpiration = s3Nve
			}
		}

		// Build noncurrent version transitions
		if !rule.NoncurrentVersionTransition.IsNull() && !rule.NoncurrentVersionTransition.IsUnknown() {
			var nvts []LifecycleNoncurrentVersionTransitionModel
			diags.Append(rule.NoncurrentVersionTransition.ElementsAs(ctx, &nvts, false)...)
			for _, nvt := range nvts {
				s3Nvt := s3types.NoncurrentVersionTransition{
					NoncurrentDays: aws.Int32(int32(nvt.NoncurrentDays.ValueInt64())),
					StorageClass:   s3types.TransitionStorageClass(nvt.StorageClass.ValueString()),
				}
				if !nvt.NewerNoncurrentVersions.IsNull() {
					s3Nvt.NewerNoncurrentVersions = aws.Int32(int32(nvt.NewerNoncurrentVersions.ValueInt64()))
				}
				s3Rule.NoncurrentVersionTransitions = append(s3Rule.NoncurrentVersionTransitions, s3Nvt)
			}
		}

		// Build abort incomplete multipart upload
		if !rule.AbortIncompleteMultipartUpload.IsNull() && !rule.AbortIncompleteMultipartUpload.IsUnknown() {
			var aborts []LifecycleAbortIncompleteMultipartUploadModel
			diags.Append(rule.AbortIncompleteMultipartUpload.ElementsAs(ctx, &aborts, false)...)
			if len(aborts) > 0 {
				abort := aborts[0]
				s3Rule.AbortIncompleteMultipartUpload = &s3types.AbortIncompleteMultipartUpload{
					DaysAfterInitiation: aws.Int32(int32(abort.DaysAfterInitiation.ValueInt64())),
				}
			}
		}

		s3Rules = append(s3Rules, s3Rule)
	}

	return &s3types.BucketLifecycleConfiguration{
		Rules: s3Rules,
	}, diags
}

// buildLifecycleFilter converts Terraform filter to AWS SDK filter.
func (r *BucketLifecycleResource) buildLifecycleFilter(ctx context.Context, filterList types.List) (*s3types.LifecycleRuleFilter, diag.Diagnostics) {
	var diags diag.Diagnostics

	if filterList.IsNull() || filterList.IsUnknown() || len(filterList.Elements()) == 0 {
		// Empty filter means apply to all objects (empty prefix)
		return &s3types.LifecycleRuleFilter{
			Prefix: aws.String(""),
		}, diags
	}

	var filters []LifecycleFilterModel
	diags.Append(filterList.ElementsAs(ctx, &filters, false)...)
	if diags.HasError() || len(filters) == 0 {
		return nil, diags
	}

	filter := filters[0]
	s3Filter := &s3types.LifecycleRuleFilter{}

	// Check for AND condition first
	if !filter.And.IsNull() && !filter.And.IsUnknown() && len(filter.And.Elements()) > 0 {
		var ands []LifecycleFilterAndModel
		diags.Append(filter.And.ElementsAs(ctx, &ands, false)...)
		if len(ands) > 0 {
			and := ands[0]
			s3And := &s3types.LifecycleRuleAndOperator{}
			if !and.Prefix.IsNull() {
				s3And.Prefix = aws.String(and.Prefix.ValueString())
			}
			if !and.Tags.IsNull() && !and.Tags.IsUnknown() {
				var tags map[string]string
				diags.Append(and.Tags.ElementsAs(ctx, &tags, false)...)
				for k, v := range tags {
					s3And.Tags = append(s3And.Tags, s3types.Tag{
						Key:   aws.String(k),
						Value: aws.String(v),
					})
				}
			}
			s3Filter.And = s3And
			return s3Filter, diags
		}
	}

	// Check for tag
	if !filter.Tag.IsNull() && !filter.Tag.IsUnknown() && len(filter.Tag.Elements()) > 0 {
		var tags []LifecycleTagModel
		diags.Append(filter.Tag.ElementsAs(ctx, &tags, false)...)
		if len(tags) > 0 {
			tag := tags[0]
			s3Filter.Tag = &s3types.Tag{
				Key:   aws.String(tag.Key.ValueString()),
				Value: aws.String(tag.Value.ValueString()),
			}
			return s3Filter, diags
		}
	}

	// Default to prefix filter
	if !filter.Prefix.IsNull() {
		s3Filter.Prefix = aws.String(filter.Prefix.ValueString())
	} else {
		s3Filter.Prefix = aws.String("")
	}
	return s3Filter, diags
}

// extractRuleIDsFromList extracts rule IDs from a types.List of lifecycle rules.
// Returns an empty slice if the list is null, unknown, or empty.
func (r *BucketLifecycleResource) extractRuleIDsFromList(ctx context.Context, rulesList types.List) []string {
	if rulesList.IsNull() || rulesList.IsUnknown() {
		return nil
	}

	var rules []LifecycleRuleModel
	if diags := rulesList.ElementsAs(ctx, &rules, false); diags.HasError() {
		return nil
	}

	ids := make([]string, 0, len(rules))
	for _, rule := range rules {
		if !rule.ID.IsNull() && !rule.ID.IsUnknown() {
			ids = append(ids, rule.ID.ValueString())
		}
	}
	return ids
}

// flattenLifecycleRules converts AWS SDK rules to Terraform state.
// expectedOrder contains rule IDs in the expected order (from config/plan) to preserve order consistency.
func (r *BucketLifecycleResource) flattenLifecycleRules(ctx context.Context, s3Rules []s3types.LifecycleRule, expectedOrder []string) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	if len(s3Rules) == 0 {
		return types.ListNull(lifecycleRuleObjectType()), diags
	}

	// Build a map for quick lookup by ID
	rulesByID := make(map[string]s3types.LifecycleRule)
	for _, rule := range s3Rules {
		rulesByID[aws.ToString(rule.ID)] = rule
	}

	// Reorder rules to match expected order, then append any new rules not in expected order
	var orderedRules []s3types.LifecycleRule
	seenIDs := make(map[string]bool)

	// First, add rules in expected order
	for _, id := range expectedOrder {
		if rule, exists := rulesByID[id]; exists {
			orderedRules = append(orderedRules, rule)
			seenIDs[id] = true
		}
	}

	// Then, add any rules not in expected order (sorted by ID for consistency)
	var newRules []s3types.LifecycleRule
	for _, rule := range s3Rules {
		id := aws.ToString(rule.ID)
		if !seenIDs[id] {
			newRules = append(newRules, rule)
		}
	}
	sort.Slice(newRules, func(i, j int) bool {
		return aws.ToString(newRules[i].ID) < aws.ToString(newRules[j].ID)
	})
	orderedRules = append(orderedRules, newRules...)

	s3Rules = orderedRules

	var rules []attr.Value
	for _, s3Rule := range s3Rules {
		rule := map[string]attr.Value{
			"id":     types.StringValue(aws.ToString(s3Rule.ID)),
			"status": types.StringValue(string(s3Rule.Status)),
		}

		// Flatten filter
		rule["filter"] = r.flattenLifecycleFilter(ctx, s3Rule.Filter)

		// Flatten expiration
		if s3Rule.Expiration != nil {
			expValues := map[string]attr.Value{
				"days":                         types.Int64Null(),
				"expired_object_delete_marker": types.BoolNull(),
			}
			if s3Rule.Expiration.Days != nil && *s3Rule.Expiration.Days > 0 {
				expValues["days"] = types.Int64Value(int64(*s3Rule.Expiration.Days))
			}
			if s3Rule.Expiration.ExpiredObjectDeleteMarker != nil {
				expValues["expired_object_delete_marker"] = types.BoolValue(*s3Rule.Expiration.ExpiredObjectDeleteMarker)
			}
			expObj, _ := types.ObjectValue(lifecycleExpirationAttrTypes(), expValues)
			rule["expiration"], _ = types.ListValue(types.ObjectType{AttrTypes: lifecycleExpirationAttrTypes()}, []attr.Value{expObj})
		} else {
			rule["expiration"] = types.ListNull(types.ObjectType{AttrTypes: lifecycleExpirationAttrTypes()})
		}

		// Flatten transitions
		if len(s3Rule.Transitions) > 0 {
			var transitions []attr.Value
			for _, t := range s3Rule.Transitions {
				tValues := map[string]attr.Value{
					"days":          types.Int64Value(int64(aws.ToInt32(t.Days))),
					"storage_class": types.StringValue(string(t.StorageClass)),
				}
				tObj, _ := types.ObjectValue(lifecycleTransitionAttrTypes(), tValues)
				transitions = append(transitions, tObj)
			}
			rule["transition"], _ = types.ListValue(types.ObjectType{AttrTypes: lifecycleTransitionAttrTypes()}, transitions)
		} else {
			rule["transition"] = types.ListNull(types.ObjectType{AttrTypes: lifecycleTransitionAttrTypes()})
		}

		// Flatten noncurrent version expiration
		if s3Rule.NoncurrentVersionExpiration != nil {
			nveValues := map[string]attr.Value{
				"noncurrent_days":           types.Int64Value(int64(aws.ToInt32(s3Rule.NoncurrentVersionExpiration.NoncurrentDays))),
				"newer_noncurrent_versions": types.Int64Null(),
			}
			if s3Rule.NoncurrentVersionExpiration.NewerNoncurrentVersions != nil {
				nveValues["newer_noncurrent_versions"] = types.Int64Value(int64(*s3Rule.NoncurrentVersionExpiration.NewerNoncurrentVersions))
			}
			nveObj, _ := types.ObjectValue(lifecycleNoncurrentVersionExpirationAttrTypes(), nveValues)
			rule["noncurrent_version_expiration"], _ = types.ListValue(types.ObjectType{AttrTypes: lifecycleNoncurrentVersionExpirationAttrTypes()}, []attr.Value{nveObj})
		} else {
			rule["noncurrent_version_expiration"] = types.ListNull(types.ObjectType{AttrTypes: lifecycleNoncurrentVersionExpirationAttrTypes()})
		}

		// Flatten noncurrent version transitions
		if len(s3Rule.NoncurrentVersionTransitions) > 0 {
			var nvts []attr.Value
			for _, nvt := range s3Rule.NoncurrentVersionTransitions {
				nvtValues := map[string]attr.Value{
					"noncurrent_days":           types.Int64Value(int64(aws.ToInt32(nvt.NoncurrentDays))),
					"newer_noncurrent_versions": types.Int64Null(),
					"storage_class":             types.StringValue(string(nvt.StorageClass)),
				}
				if nvt.NewerNoncurrentVersions != nil {
					nvtValues["newer_noncurrent_versions"] = types.Int64Value(int64(*nvt.NewerNoncurrentVersions))
				}
				nvtObj, _ := types.ObjectValue(lifecycleNoncurrentVersionTransitionAttrTypes(), nvtValues)
				nvts = append(nvts, nvtObj)
			}
			rule["noncurrent_version_transition"], _ = types.ListValue(types.ObjectType{AttrTypes: lifecycleNoncurrentVersionTransitionAttrTypes()}, nvts)
		} else {
			rule["noncurrent_version_transition"] = types.ListNull(types.ObjectType{AttrTypes: lifecycleNoncurrentVersionTransitionAttrTypes()})
		}

		// Flatten abort incomplete multipart upload
		if s3Rule.AbortIncompleteMultipartUpload != nil {
			abortValues := map[string]attr.Value{
				"days_after_initiation": types.Int64Value(int64(aws.ToInt32(s3Rule.AbortIncompleteMultipartUpload.DaysAfterInitiation))),
			}
			abortObj, _ := types.ObjectValue(lifecycleAbortIncompleteMultipartUploadAttrTypes(), abortValues)
			rule["abort_incomplete_multipart_upload"], _ = types.ListValue(types.ObjectType{AttrTypes: lifecycleAbortIncompleteMultipartUploadAttrTypes()}, []attr.Value{abortObj})
		} else {
			rule["abort_incomplete_multipart_upload"] = types.ListNull(types.ObjectType{AttrTypes: lifecycleAbortIncompleteMultipartUploadAttrTypes()})
		}

		ruleObj, _ := types.ObjectValue(lifecycleRuleAttrTypes(), rule)
		rules = append(rules, ruleObj)
	}

	result, d := types.ListValue(lifecycleRuleObjectType(), rules)
	diags.Append(d...)
	return result, diags
}

// flattenLifecycleFilter converts AWS SDK filter to Terraform state.
func (r *BucketLifecycleResource) flattenLifecycleFilter(ctx context.Context, filter *s3types.LifecycleRuleFilter) types.List {
	if filter == nil {
		return types.ListNull(types.ObjectType{AttrTypes: lifecycleFilterAttrTypes()})
	}

	filterValues := map[string]attr.Value{
		"prefix": types.StringNull(),
		"tag":    types.ListNull(types.ObjectType{AttrTypes: lifecycleTagAttrTypes()}),
		"and":    types.ListNull(types.ObjectType{AttrTypes: lifecycleFilterAndAttrTypes()}),
	}

	// Check for AND condition
	if filter.And != nil {
		andValues := map[string]attr.Value{
			"prefix": types.StringNull(),
			"tags":   types.MapNull(types.StringType),
		}
		if filter.And.Prefix != nil {
			andValues["prefix"] = types.StringValue(*filter.And.Prefix)
		}
		if len(filter.And.Tags) > 0 {
			tags := make(map[string]attr.Value)
			for _, tag := range filter.And.Tags {
				tags[aws.ToString(tag.Key)] = types.StringValue(aws.ToString(tag.Value))
			}
			andValues["tags"], _ = types.MapValue(types.StringType, tags)
		}
		andObj, _ := types.ObjectValue(lifecycleFilterAndAttrTypes(), andValues)
		filterValues["and"], _ = types.ListValue(types.ObjectType{AttrTypes: lifecycleFilterAndAttrTypes()}, []attr.Value{andObj})
	} else if filter.Tag != nil {
		// Check for tag
		tagValues := map[string]attr.Value{
			"key":   types.StringValue(aws.ToString(filter.Tag.Key)),
			"value": types.StringValue(aws.ToString(filter.Tag.Value)),
		}
		tagObj, _ := types.ObjectValue(lifecycleTagAttrTypes(), tagValues)
		filterValues["tag"], _ = types.ListValue(types.ObjectType{AttrTypes: lifecycleTagAttrTypes()}, []attr.Value{tagObj})
	} else if filter.Prefix != nil && *filter.Prefix != "" {
		// Check for prefix
		filterValues["prefix"] = types.StringValue(*filter.Prefix)
	}

	filterObj, _ := types.ObjectValue(lifecycleFilterAttrTypes(), filterValues)
	result, _ := types.ListValue(types.ObjectType{AttrTypes: lifecycleFilterAttrTypes()}, []attr.Value{filterObj})
	return result
}

// Attribute type helpers
func lifecycleRuleObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: lifecycleRuleAttrTypes()}
}

func lifecycleRuleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                                types.StringType,
		"status":                            types.StringType,
		"filter":                            types.ListType{ElemType: types.ObjectType{AttrTypes: lifecycleFilterAttrTypes()}},
		"expiration":                        types.ListType{ElemType: types.ObjectType{AttrTypes: lifecycleExpirationAttrTypes()}},
		"transition":                        types.ListType{ElemType: types.ObjectType{AttrTypes: lifecycleTransitionAttrTypes()}},
		"noncurrent_version_expiration":     types.ListType{ElemType: types.ObjectType{AttrTypes: lifecycleNoncurrentVersionExpirationAttrTypes()}},
		"noncurrent_version_transition":     types.ListType{ElemType: types.ObjectType{AttrTypes: lifecycleNoncurrentVersionTransitionAttrTypes()}},
		"abort_incomplete_multipart_upload": types.ListType{ElemType: types.ObjectType{AttrTypes: lifecycleAbortIncompleteMultipartUploadAttrTypes()}},
	}
}

func lifecycleFilterAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"prefix": types.StringType,
		"tag":    types.ListType{ElemType: types.ObjectType{AttrTypes: lifecycleTagAttrTypes()}},
		"and":    types.ListType{ElemType: types.ObjectType{AttrTypes: lifecycleFilterAndAttrTypes()}},
	}
}

func lifecycleFilterAndAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"prefix": types.StringType,
		"tags":   types.MapType{ElemType: types.StringType},
	}
}

func lifecycleTagAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"key":   types.StringType,
		"value": types.StringType,
	}
}

func lifecycleExpirationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"days":                         types.Int64Type,
		"expired_object_delete_marker": types.BoolType,
	}
}

func lifecycleTransitionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"days":          types.Int64Type,
		"storage_class": types.StringType,
	}
}

func lifecycleNoncurrentVersionExpirationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"noncurrent_days":           types.Int64Type,
		"newer_noncurrent_versions": types.Int64Type,
	}
}

func lifecycleNoncurrentVersionTransitionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"noncurrent_days":           types.Int64Type,
		"newer_noncurrent_versions": types.Int64Type,
		"storage_class":             types.StringType,
	}
}

func lifecycleAbortIncompleteMultipartUploadAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"days_after_initiation": types.Int64Type,
	}
}
