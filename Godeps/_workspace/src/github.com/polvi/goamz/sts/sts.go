// The sts package provides types and functions for interaction with the AWS
// Security Token Service (STS) service.
package sts

import (
	"encoding/xml"
	"github.com/polvi/coreup/Godeps/_workspace/src/github.com/polvi/goamz/aws"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The STS type encapsulates operations operations with the STS endpoint.
type STS struct {
	aws.Region
	httpClient *http.Client
}

// New creates a new STS instance.
func New(region aws.Region) *STS {
	return NewWithClient(region, aws.RetryingClient)
}

func NewWithClient(region aws.Region, httpClient *http.Client) *STS {
	return &STS{region, httpClient}
}

func (sts *STS) query(params map[string]string, resp interface{}) error {
	params["Version"] = "2011-06-15"
	params["Timestamp"] = time.Now().In(time.UTC).Format(time.RFC3339)
	endpoint, err := url.Parse(sts.STSEndpoint)
	if err != nil {
		return err
	}
	endpoint.RawQuery = multimap(params).Encode()
	r, err := sts.httpClient.Get(endpoint.String())
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode > 200 {
		return buildError(r)
	}
	return xml.NewDecoder(r.Body).Decode(resp)
}

func (sts *STS) postQuery(params map[string]string, resp interface{}) error {
	endpoint, err := url.Parse(sts.STSEndpoint)
	if err != nil {
		return err
	}
	params["Version"] = "2010-05-08"
	params["Timestamp"] = time.Now().In(time.UTC).Format(time.RFC3339)
	encoded := multimap(params).Encode()
	body := strings.NewReader(encoded)
	req, err := http.NewRequest("POST", endpoint.String(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Host", endpoint.Host)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Content-Length", strconv.Itoa(len(encoded)))
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode > 200 {
		return buildError(r)
	}
	return xml.NewDecoder(r.Body).Decode(resp)
}

func buildError(r *http.Response) error {
	var (
		err    Error
		errors xmlErrors
	)
	xml.NewDecoder(r.Body).Decode(&errors)
	if len(errors.Errors) > 0 {
		err = errors.Errors[0]
	}
	err.StatusCode = r.StatusCode
	if err.Message == "" {
		err.Message = r.Status
	}
	return &err
}

func multimap(p map[string]string) url.Values {
	q := make(url.Values, len(p))
	for k, v := range p {
		q[k] = []string{v}
	}
	return q
}

// Response to a AssumeRoleWithWebIdentity request.
//
// See http://goo.gl/fdXrLn for more details.
type AssumeRoleWithWebIdentityResp struct {
	RequestId                   string          `xml:"ResponseMetadata>RequestId"`
	AssumedRoleUser             AssumedRoleUser `xml:"AssumeRoleWithWebIdentityResult>AssumedRoleUser"`
	Credentials                 Credentials     `xml:"AssumeRoleWithWebIdentityResult>Credentials"`
	SubjectFromWebIdentityToken string          `xml:"AssumeRoleWithWebIdentityResult>SubjectFromWebIdentityToken"`
}

// AssumedRoleUser is the role that is assumed by the request to STS.
//
// See http://goo.gl/UB42VS for more details.
type AssumedRoleUser struct {
	Arn           string
	AssumedRoleId string `xml:"AssumedRoleId"`
}

// Credentials are the temporary AWS credentials returned by STS.
//
// See http://goo.gl/ZQn9Bt for more details.
type Credentials struct {
	AccessKeyId     string
	Expiration      string
	SecretAccessKey string
	SessionToken    string
}

//  AssumeRoleWithWebIdentity in STS.
//
// See http://goo.gl/fdXrLn for more details.
func (sts *STS) AssumeRoleWithWebIdentity(duration int, policy string, provider_id string, role_arn string, role_session_name string, web_identity_token string) (*AssumeRoleWithWebIdentityResp, error) {
	params := map[string]string{
		"Action":           "AssumeRoleWithWebIdentity",
		"DurationSeconds":  strconv.Itoa(duration),
		"RoleArn":          role_arn,
		"RoleSessionName":  role_session_name,
		"WebIdentityToken": web_identity_token,
	}
	if policy != "" {
		params["Policy"] = policy
	}
	if provider_id != "" {
		params["ProviderId"] = provider_id
	}
	resp := new(AssumeRoleWithWebIdentityResp)
	if err := sts.query(params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

type SimpleResp struct {
	RequestId string `xml:"ResponseMetadata>RequestId"`
}

type xmlErrors struct {
	Errors []Error `xml:"Error"`
}

// Error encapsulates an STS error.
type Error struct {
	// HTTP status code of the error.
	StatusCode int

	// AWS code of the error.
	Code string

	// Message explaining the error.
	Message string
}

func (e *Error) Error() string {
	var prefix string
	if e.Code != "" {
		prefix = e.Code + ": "
	}
	if prefix == "" && e.StatusCode > 0 {
		prefix = strconv.Itoa(e.StatusCode) + ": "
	}
	return prefix + e.Message
}
