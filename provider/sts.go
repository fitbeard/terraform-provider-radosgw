package provider

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"strconv"
)

// stsCredentials holds temporary credentials returned by an STS assume-role call.
type stsCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expiration      string
	AssumedRoleARN  string
}

// assumeRoleParams are the inputs for an STS AssumeRole call.
//
// Note: RadosGW does not support an external ID (a role whose trust policy has an
// sts:ExternalId condition cannot be assumed even with the matching value), so
// there is deliberately no ExternalID field here.
type assumeRoleParams struct {
	RoleARN         string
	RoleSessionName string
	Policy          string
	DurationSeconds int
}

// assumeRole calls STS AssumeRole against the RadosGW endpoint, signing the
// request with the client's (base) credentials, and returns the temporary
// credentials RadosGW issues for the role. client must carry the long-term
// credentials allowed to assume the role (AssumeRole is a signed call).
func assumeRole(ctx context.Context, client *IAMClient, p assumeRoleParams) (stsCredentials, error) {
	params := url.Values{}
	params.Set("Action", "AssumeRole")
	params.Set("Version", "2011-06-15")
	params.Set("RoleArn", p.RoleARN)
	params.Set("RoleSessionName", p.RoleSessionName)
	if p.DurationSeconds > 0 {
		params.Set("DurationSeconds", strconv.Itoa(p.DurationSeconds))
	}
	if p.Policy != "" {
		params.Set("Policy", p.Policy)
	}

	body, err := client.DoPostRequest(ctx, params, "sts")
	if err != nil {
		return stsCredentials{}, err
	}

	var resp struct {
		Result struct {
			Credentials struct {
				AccessKeyID     string `xml:"AccessKeyId"`
				SecretAccessKey string `xml:"SecretAccessKey"`
				SessionToken    string `xml:"SessionToken"`
				Expiration      string `xml:"Expiration"`
			} `xml:"Credentials"`
			AssumedRoleUser struct {
				Arn string `xml:"Arn"`
			} `xml:"AssumedRoleUser"`
		} `xml:"AssumeRoleResult"`
	}
	if err := xml.Unmarshal(body, &resp); err != nil {
		return stsCredentials{}, fmt.Errorf("could not parse AssumeRole response: %w (body: %s)", err, string(body))
	}

	creds := stsCredentials{
		AccessKeyID:     resp.Result.Credentials.AccessKeyID,
		SecretAccessKey: resp.Result.Credentials.SecretAccessKey,
		SessionToken:    resp.Result.Credentials.SessionToken,
		Expiration:      resp.Result.Credentials.Expiration,
		AssumedRoleARN:  resp.Result.AssumedRoleUser.Arn,
	}
	if creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
		return stsCredentials{}, fmt.Errorf("AssumeRole returned no credentials (body: %s)", string(body))
	}
	return creds, nil
}
