// Copyright 2021 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package upbound

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/pkg/errors"
)

func Test_GetAgentCerts(t *testing.T) {
	errBoom := errors.New("boom")

	endpoint := "https://foo.com"
	endpointToken := "platform-token"
	defaultResponse := map[string]string{
		keyNATSCA:       "test-ca",
		keyJWTPublicKey: "test-jwt-public-key",
	}

	type args struct {
		responderErr error
		responseCode int
		responseBody interface{}
	}
	type want struct {
		out PublicCerts
		err error
	}
	cases := map[string]struct {
		args
		want
	}{
		"Success": {
			args: args{
				responseCode: http.StatusOK,
				responseBody: defaultResponse,
			},
			want: want{
				out: PublicCerts{
					JWTPublicKey: "test-jwt-public-key",
					NATSCA:       "test-ca",
				},
				err: nil,
			},
		},
		"ServerError": {
			args: args{
				responseCode: http.StatusInternalServerError,
				responseBody: "some-error",
			},
			want: want{
				err: errors.New("agent certs request failed with 500 - \"some-error\""),
			},
		},
		"UnexpectedResponseBody": {
			args: args{
				responseCode: http.StatusOK,
				responseBody: "test-ca",
			},
			want: want{
				err: errors.WithStack(errors.New("failed to unmarshall agent certs response: json: cannot unmarshal string into Go value of type map[string]string")),
			},
		},
		"EmptyCerts": {
			args: args{
				responseCode: http.StatusOK,
				responseBody: map[string]string{},
			},
			want: want{
				err: errors.New("empty jwt public key received"),
			},
		},
		"RestyTransportErr": {
			args: args{
				responderErr: errBoom,
			},
			want: want{
				err: errors.Wrap(&url.Error{
					Op:  "Get",
					URL: "https://foo.com/v1/gw/certs",
					Err: errBoom,
				}, "failed to request agent certs"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rc := NewClient(endpoint, logging.NewNopLogger(), false, false)

			httpmock.ActivateNonDefault(rc.(*client).resty.GetClient())

			b, err := json.Marshal(tc.responseBody)
			if err != nil {
				t.Errorf("cannot unmarshal tc.responseBody %v", err)
			}

			var responder httpmock.Responder
			if tc.responderErr != nil {
				responder = httpmock.NewErrorResponder(tc.responderErr)
			} else {
				responder = httpmock.NewStringResponder(tc.responseCode, string(b))
			}

			httpmock.RegisterResponder(http.MethodGet, endpoint+gwCertsPath, responder)

			got, gotErr := rc.GetAgentCerts(endpointToken)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("GetAgentCerts(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.out, got); diff != "" {
				t.Errorf("GetAgentCerts(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func Test_fetchNewJWT(t *testing.T) {
	errBoom := errors.New("boom")

	endpoint := "https://foo.com"
	endpointToken := "platform-token"
	clusterID := uuid.New()
	defaultResponse := map[string]string{
		"token": "test-jwt",
	}

	type args struct {
		responderErr error
		responseCode int
		responseBody interface{}
	}
	type want struct {
		jwt string
		err error
	}
	cases := map[string]struct {
		args
		want
	}{
		"Success": {
			args: args{
				responseCode: http.StatusOK,
				responseBody: defaultResponse,
			},
			want: want{
				jwt: "test-jwt",
				err: nil,
			},
		},
		"ServerError": {
			args: args{
				responseCode: http.StatusInternalServerError,
				responseBody: "some-error",
			},
			want: want{
				err: errors.New("new token request failed with 500 - \"some-error\""),
			},
		},
		"UnexpectedResponseBody": {
			args: args{
				responseCode: http.StatusOK,
				responseBody: "test-ca",
			},
			want: want{
				err: errors.WithStack(errors.New("failed to unmarshall nats token response: json: cannot unmarshal string into Go value of type map[string]string")),
			},
		},
		"EmptyToken": {
			args: args{
				responseCode: http.StatusOK,
				responseBody: map[string]string{
					"token": "",
				},
			},
			want: want{
				err: errors.New("empty token received"),
			},
		},
		"RestyTransportErr": {
			args: args{
				responderErr: errBoom,
			},
			want: want{
				err: errors.Wrap(&url.Error{
					Op:  "Post",
					URL: "https://foo.com/v1/nats/token",
					Err: errBoom,
				}, "failed to request new token"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rc := NewClient(endpoint, logging.NewNopLogger(), false, false)

			httpmock.ActivateNonDefault(rc.(*client).resty.GetClient())

			b, err := json.Marshal(tc.responseBody)
			if err != nil {
				t.Errorf("cannot unmarshal tc.responseBody %v", err)
			}

			var responder httpmock.Responder
			if tc.responderErr != nil {
				responder = httpmock.NewErrorResponder(tc.responderErr)
			} else {
				responder = httpmock.NewStringResponder(tc.responseCode, string(b))
			}

			httpmock.RegisterResponder(http.MethodPost, endpoint+natsTokenPath, responder)

			got, gotErr := rc.FetchNewJWTToken(endpointToken, clusterID.String(), "some-public-key")
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("fetchNewJWTToken(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.jwt, got); diff != "" {
				t.Errorf("fetchNewJWTToken(...): -want result, +got result: %s", diff)
			}
		})
	}
}
