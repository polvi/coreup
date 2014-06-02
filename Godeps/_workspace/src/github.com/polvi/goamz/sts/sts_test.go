package sts_test

import (
	"github.com/polvi/coreup/Godeps/_workspace/src/github.com/polvi/goamz/aws"
	"github.com/polvi/coreup/Godeps/_workspace/src/github.com/polvi/goamz/sts"
	"github.com/polvi/goamz/testutil"
	. "github.com/motain/gocheck"
	"testing"
)

func Test(t *testing.T) {
	TestingT(t)
}

type S struct {
	sts *sts.STS
}

var _ = Suite(&S{})

var testServer = testutil.NewHTTPServer()

func (s *S) SetUpSuite(c *C) {
	testServer.Start()
	auth := aws.Auth{"abc", "123", ""}
	s.sts = sts.NewWithClient(auth, aws.Region{STSEndpoint: testServer.URL}, testutil.DefaultClient)
}

func (s *S) TearDownTest(c *C) {
	testServer.Flush()
}

func (s *S) TestAssumeRoleWithWebIdentity(c *C) {
	testServer.Response(200, nil, AssumeRoleWithWebIdentityExample)
	resp, err := s.sts.AssumeRoleWithWebIdentity(3600, "{}", "accounts.google.com", "arn", "role_session_name", "token")
	values := testServer.WaitRequest().URL.Query()
	c.Assert(values.Get("Action"), Equals, "AssumeRoleWithWebIdentity")
	c.Assert(values.Get("DurationSeconds"), Equals, "3600")
	c.Assert(values.Get("Policy"), Equals, "{}")
	c.Assert(values.Get("ProviderId"), Equals, "accounts.google.com")
	c.Assert(values.Get("RoleArn"), Equals, "arn")
	c.Assert(values.Get("RoleSessionName"), Equals, "role_session_name")
	c.Assert(values.Get("WebIdentityToken"), Equals, "token")
	c.Assert(err, IsNil)
	c.Assert(resp.RequestId, Equals, "ad4156e9-bce1-11e2-82e6-6b6ef249e618")
	c.Assert(resp.SubjectFromWebIdentityToken, Equals, "amzn1.account.AF6RHO7KZU5XRVQJGXK6HB56KR2A")
	expected := sts.AssumedRoleUser{
		Arn:           "arn:aws:sts::000240903217:assumed-role/FederatedWebIdentityRole/app1",
		AssumedRoleId: "AROACLKWSDQRAOFQC3IDI:app1",
	}
	c.Assert(resp.AssumedRoleUser, DeepEquals, expected)
	expected2 := sts.Credentials{
		AccessKeyId:     "AKIAIOSFODNN7EXAMPLE",
		Expiration:      "2013-05-14T23:00:23Z",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYzEXAMPLEKEY",
		SessionToken:    "AQoDYXdzEE0a8ANXXXXXXXXNO1ewxE5TijQyp+IPfnyowF",
	}
	c.Assert(resp.Credentials, DeepEquals, expected2)
}
