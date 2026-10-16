package provider

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

// assumeRoleWithWebIdentityParams are the inputs for an STS
// AssumeRoleWithWebIdentity call.
type assumeRoleWithWebIdentityParams struct {
	RoleARN          string
	RoleSessionName  string
	WebIdentityToken string
	Policy           string
	DurationSeconds  int
}

// assumeRoleWithWebIdentity calls STS AssumeRoleWithWebIdentity against the
// RadosGW endpoint and returns temporary credentials. This call is anonymous —
// the web identity token (a JWT from the OIDC provider) is the credential, so the
// request is NOT SigV4-signed. httpClient carries the TLS configuration.
func assumeRoleWithWebIdentity(ctx context.Context, endpoint string, httpClient HTTPClient, p assumeRoleWithWebIdentityParams) (stsCredentials, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	form := url.Values{}
	form.Set("Action", "AssumeRoleWithWebIdentity")
	form.Set("Version", "2011-06-15")
	form.Set("RoleArn", p.RoleARN)
	form.Set("RoleSessionName", p.RoleSessionName)
	form.Set("WebIdentityToken", p.WebIdentityToken)
	if p.DurationSeconds > 0 {
		form.Set("DurationSeconds", strconv.Itoa(p.DurationSeconds))
	}
	if p.Policy != "" {
		form.Set("Policy", p.Policy)
	}

	reqURL := strings.TrimRight(endpoint, "/") + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return stsCredentials{}, fmt.Errorf("could not build AssumeRoleWithWebIdentity request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return stsCredentials{}, fmt.Errorf("AssumeRoleWithWebIdentity request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return stsCredentials{}, fmt.Errorf("could not read AssumeRoleWithWebIdentity response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return stsCredentials{}, fmt.Errorf("AssumeRoleWithWebIdentity returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var out struct {
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
		} `xml:"AssumeRoleWithWebIdentityResult"`
	}
	if err := xml.Unmarshal(body, &out); err != nil {
		return stsCredentials{}, fmt.Errorf("could not parse AssumeRoleWithWebIdentity response: %w (body: %s)", err, string(body))
	}

	creds := stsCredentials{
		AccessKeyID:     out.Result.Credentials.AccessKeyID,
		SecretAccessKey: out.Result.Credentials.SecretAccessKey,
		SessionToken:    out.Result.Credentials.SessionToken,
		Expiration:      out.Result.Credentials.Expiration,
		AssumedRoleARN:  out.Result.AssumedRoleUser.Arn,
	}
	if creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
		return stsCredentials{}, fmt.Errorf("AssumeRoleWithWebIdentity returned no credentials (body: %s)", string(body))
	}
	return creds, nil
}
