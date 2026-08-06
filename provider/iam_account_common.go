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
