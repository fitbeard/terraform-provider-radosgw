package provider

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
)

// Shared IAM (account data-plane) helpers used by the radosgw_iam_account_*
// resources and data sources. All operate through the hand-rolled IAMClient.

// iamAccessKeyMetadataXML is one entry of a ListAccessKeys response.
type iamAccessKeyMetadataXML struct {
	UserName    string `xml:"UserName"`
	AccessKeyId string `xml:"AccessKeyId"`
	Status      string `xml:"Status"`
	CreateDate  string `xml:"CreateDate"`
}

type listAccessKeysResponseXML struct {
	XMLName xml.Name `xml:"ListAccessKeysResponse"`
	Result  struct {
		Members []iamAccessKeyMetadataXML `xml:"AccessKeyMetadata>member"`
	} `xml:"ListAccessKeysResult"`
}

// iamListAccessKeys returns the access key metadata for a user via ListAccessKeys.
func iamListAccessKeys(ctx context.Context, c *IAMClient, userName string) ([]iamAccessKeyMetadataXML, error) {
	params := url.Values{}
	params.Set("Action", "ListAccessKeys")
	params.Set("UserName", userName)

	body, err := c.DoRequest(ctx, params, "iam")
	if err != nil {
		return nil, err
	}
	var response listAccessKeysResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("could not parse ListAccessKeys response: %w", err)
	}
	return response.Result.Members, nil
}

// iamListAccessKeyIDs returns just the access key IDs for a user.
func iamListAccessKeyIDs(ctx context.Context, c *IAMClient, userName string) ([]string, error) {
	keys, err := iamListAccessKeys(ctx, c, userName)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(keys))
	for _, k := range keys {
		ids = append(ids, k.AccessKeyId)
	}
	return ids, nil
}

type listUserPoliciesResponseXML struct {
	XMLName xml.Name `xml:"ListUserPoliciesResponse"`
	Result  struct {
		PolicyNames []string `xml:"PolicyNames>member"`
	} `xml:"ListUserPoliciesResult"`
}

// iamListUserInlinePolicies returns the inline policy names for a user.
func iamListUserInlinePolicies(ctx context.Context, c *IAMClient, userName string) ([]string, error) {
	params := url.Values{}
	params.Set("Action", "ListUserPolicies")
	params.Set("UserName", userName)

	body, err := c.DoRequest(ctx, params, "iam")
	if err != nil {
		return nil, err
	}
	var response listUserPoliciesResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("could not parse ListUserPolicies response: %w", err)
	}
	return response.Result.PolicyNames, nil
}

type listAttachedUserPoliciesResponseXML struct {
	XMLName xml.Name `xml:"ListAttachedUserPoliciesResponse"`
	Result  struct {
		PolicyARNs []string `xml:"AttachedPolicies>member>PolicyArn"`
	} `xml:"ListAttachedUserPoliciesResult"`
}

// iamListAttachedUserPolicyARNs returns the attached (managed) policy ARNs for a user.
func iamListAttachedUserPolicyARNs(ctx context.Context, c *IAMClient, userName string) ([]string, error) {
	params := url.Values{}
	params.Set("Action", "ListAttachedUserPolicies")
	params.Set("UserName", userName)

	body, err := c.DoRequest(ctx, params, "iam")
	if err != nil {
		return nil, err
	}
	var response listAttachedUserPoliciesResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("could not parse ListAttachedUserPolicies response: %w", err)
	}
	return response.Result.PolicyARNs, nil
}

// iamGetUser fetches a single IAM user via GetUser.
func iamGetUser(ctx context.Context, c *IAMClient, userName string) (iamUserXML, error) {
	params := url.Values{}
	params.Set("Action", "GetUser")
	params.Set("UserName", userName)

	body, err := c.DoRequest(ctx, params, "iam")
	if err != nil {
		return iamUserXML{}, err
	}
	var response getUserResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		return iamUserXML{}, fmt.Errorf("could not parse GetUser response: %w", err)
	}
	return response.Result.User, nil
}

// iamGetGroup fetches a single IAM group via GetGroup.
func iamGetGroup(ctx context.Context, c *IAMClient, name string) (iamGroupXML, error) {
	params := url.Values{}
	params.Set("Action", "GetGroup")
	params.Set("GroupName", name)

	body, err := c.DoRequest(ctx, params, "iam")
	if err != nil {
		return iamGroupXML{}, err
	}
	var response getGroupResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		return iamGroupXML{}, fmt.Errorf("could not parse GetGroup response: %w", err)
	}
	return response.Result.Group, nil
}

// iamListUsersXML is used by the account users data source.
type listUsersResponseXML struct {
	XMLName xml.Name `xml:"ListUsersResponse"`
	Result  struct {
		Users []iamUserXML `xml:"Users>member"`
	} `xml:"ListUsersResult"`
}

// iamListUsers returns all IAM users in the account, optionally filtered by path
// prefix (RGW's ListUsers honors PathPrefix).
func iamListUsers(ctx context.Context, c *IAMClient, pathPrefix string) ([]iamUserXML, error) {
	params := url.Values{}
	params.Set("Action", "ListUsers")
	if pathPrefix != "" {
		params.Set("PathPrefix", pathPrefix)
	}

	body, err := c.DoRequest(ctx, params, "iam")
	if err != nil {
		return nil, err
	}
	var response listUsersResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("could not parse ListUsers response: %w", err)
	}
	return response.Result.Users, nil
}

// iamListRoleAttachedPolicyARNs returns the attached (managed) policy ARNs for a role.
func iamListRoleAttachedPolicyARNs(ctx context.Context, c *IAMClient, roleName string) ([]string, error) {
	params := url.Values{}
	params.Set("Action", "ListAttachedRolePolicies")
	params.Set("RoleName", roleName)
	body, err := c.DoRequest(ctx, params, "iam")
	if err != nil {
		return nil, err
	}
	var response struct {
		XMLName xml.Name `xml:"ListAttachedRolePoliciesResponse"`
		Result  struct {
			PolicyARNs []string `xml:"AttachedPolicies>member>PolicyArn"`
		} `xml:"ListAttachedRolePoliciesResult"`
	}
	if err := xml.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("could not parse ListAttachedRolePolicies response: %w", err)
	}
	return response.Result.PolicyARNs, nil
}

// iamListGroupAttachedPolicyARNs returns the attached (managed) policy ARNs for a group.
func iamListGroupAttachedPolicyARNs(ctx context.Context, c *IAMClient, groupName string) ([]string, error) {
	params := url.Values{}
	params.Set("Action", "ListAttachedGroupPolicies")
	params.Set("GroupName", groupName)
	body, err := c.DoRequest(ctx, params, "iam")
	if err != nil {
		return nil, err
	}
	var response struct {
		XMLName xml.Name `xml:"ListAttachedGroupPoliciesResponse"`
		Result  struct {
			PolicyARNs []string `xml:"AttachedPolicies>member>PolicyArn"`
		} `xml:"ListAttachedGroupPoliciesResult"`
	}
	if err := xml.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("could not parse ListAttachedGroupPolicies response: %w", err)
	}
	return response.Result.PolicyARNs, nil
}

// iamGetGroupMembers returns the user names that are members of a group (GetGroup).
func iamGetGroupMembers(ctx context.Context, c *IAMClient, groupName string) ([]string, error) {
	params := url.Values{}
	params.Set("Action", "GetGroup")
	params.Set("GroupName", groupName)
	body, err := c.DoRequest(ctx, params, "iam")
	if err != nil {
		return nil, err
	}
	var response struct {
		XMLName xml.Name `xml:"GetGroupResponse"`
		Result  struct {
			Users []string `xml:"Users>member>UserName"`
		} `xml:"GetGroupResult"`
	}
	if err := xml.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("could not parse GetGroup (members) response: %w", err)
	}
	return response.Result.Users, nil
}

// iamAttachPolicy attaches a managed policy to an entity (Attach{User,Role,Group}Policy).
func iamAttachPolicy(ctx context.Context, c *IAMClient, action, entityKey, entityName, policyARN string) error {
	params := url.Values{}
	params.Set("Action", action)
	params.Set(entityKey, entityName)
	params.Set("PolicyArn", policyARN)
	_, err := c.DoRequest(ctx, params, "iam")
	return err
}

// iamDetachPolicy detaches a managed policy from an entity (Detach{User,Role,Group}Policy).
func iamDetachPolicy(ctx context.Context, c *IAMClient, action, entityKey, entityName, policyARN string) error {
	params := url.Values{}
	params.Set("Action", action)
	params.Set(entityKey, entityName)
	params.Set("PolicyArn", policyARN)
	_, err := c.DoRequest(ctx, params, "iam")
	return err
}

// newAccountIAMClient builds an IAMClient from the provider client (using the
// configured endpoint/credentials — typically an account root user).
func newAccountIAMClient(client *RadosgwClient) *IAMClient {
	return NewIAMClient(
		client.Admin.Endpoint,
		client.Admin.AccessKey,
		client.Admin.SecretKey,
		client.Admin.HTTPClient,
	)
}
