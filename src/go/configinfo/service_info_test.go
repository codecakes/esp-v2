// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package configinfo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/esp-v2/src/go/options"
	"github.com/GoogleCloudPlatform/esp-v2/src/go/util"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/mux"

	commonpb "github.com/GoogleCloudPlatform/esp-v2/src/go/proto/api/envoy/v8/http/common"
	scpb "github.com/GoogleCloudPlatform/esp-v2/src/go/proto/api/envoy/v8/http/service_control"
	annotationspb "google.golang.org/genproto/googleapis/api/annotations"
	confpb "google.golang.org/genproto/googleapis/api/serviceconfig"
	apipb "google.golang.org/genproto/protobuf/api"
	ptypepb "google.golang.org/genproto/protobuf/ptype"
)

var (
	testProjectName = "bookstore.endpoints.project123.cloud.goog"
	testApiName     = "endpoints.examples.bookstore.Bookstore"
	testConfigID    = "2019-03-02r0"
)

func TestProcessEndpoints(t *testing.T) {
	testData := []struct {
		desc              string
		fakeServiceConfig *confpb.Service
		wantedAllowCors   bool
	}{
		{
			desc: "Return true for endpoint name matching service name",
			fakeServiceConfig: &confpb.Service{
				Name: testProjectName,
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
				Endpoints: []*confpb.Endpoint{
					{
						Name:      testProjectName,
						AllowCors: true,
					},
				},
			},
			wantedAllowCors: true,
		},
		{
			desc: "Return false for not setting allow_cors",
			fakeServiceConfig: &confpb.Service{
				Name: testProjectName,
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
				Endpoints: []*confpb.Endpoint{
					{
						Name: testProjectName,
					},
				},
			},
			wantedAllowCors: false,
		},
		{
			desc: "Return false for endpoint name not matching service name",
			fakeServiceConfig: &confpb.Service{
				Name: testProjectName,
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
				Endpoints: []*confpb.Endpoint{
					{
						Name:      "echo.endpoints.project123.cloud.goog",
						AllowCors: true,
					},
				},
			},
			wantedAllowCors: false,
		},
		{
			desc: "Return false for empty endpoint field",
			fakeServiceConfig: &confpb.Service{
				Name: testProjectName,
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
			},
			wantedAllowCors: false,
		},
	}

	for i, tc := range testData {
		opts := options.DefaultConfigGeneratorOptions()
		serviceInfo, err := NewServiceInfoFromServiceConfig(tc.fakeServiceConfig, testConfigID, opts)
		if err != nil {
			t.Fatal(err)
		}

		if serviceInfo.AllowCors != tc.wantedAllowCors {
			t.Errorf("Test Desc(%d): %s, allow CORS flag got: %v, want: %v", i, tc.desc, serviceInfo.AllowCors, tc.wantedAllowCors)
		}
	}
}

func TestProcessApiKeyLocations(t *testing.T) {
	testData := []struct {
		desc                                   string
		fakeServiceConfig                      *confpb.Service
		wantedSystemParameters                 map[string][]*confpb.SystemParameter
		wantedAllTranscodingIgnoredQueryParams map[string]bool
		wantMethods                            map[string]*methodInfo
	}{
		{
			desc: "Succeed, only header",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
						Methods: []*apipb.Method{
							{
								Name: "echo",
							},
						},
					},
				},
				SystemParameters: &confpb.SystemParameters{
					Rules: []*confpb.SystemParameterRule{
						{
							Selector: "1.echo_api_endpoints_cloudesf_testing_cloud_goog.echo",
							Parameters: []*confpb.SystemParameter{
								{
									Name:       "api_key",
									HttpHeader: "header_name",
								},
							},
						},
					},
				},
			},
			wantedAllTranscodingIgnoredQueryParams: map[string]bool{},
			wantMethods: map[string]*methodInfo{
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.echo": &methodInfo{
					ShortName: "echo",
					ApiName:   "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/1.echo_api_endpoints_cloudesf_testing_cloud_goog/echo",
							HttpMethod:  util.POST,
						},
					},
					ApiKeyLocations: []*scpb.ApiKeyLocation{
						{
							Key: &scpb.ApiKeyLocation_Header{
								Header: "header_name",
							},
						},
					},
				},
			},
		},
		{
			desc: "Succeed, only url query",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
						Methods: []*apipb.Method{
							{
								Name: "echo",
							},
						},
					},
				},
				SystemParameters: &confpb.SystemParameters{
					Rules: []*confpb.SystemParameterRule{
						{
							Selector: "1.echo_api_endpoints_cloudesf_testing_cloud_goog.echo",
							Parameters: []*confpb.SystemParameter{
								{
									Name:              "api_key",
									UrlQueryParameter: "query_name",
								},
							},
						},
					},
				},
			},
			wantedAllTranscodingIgnoredQueryParams: map[string]bool{
				"query_name": true,
			},
			wantMethods: map[string]*methodInfo{
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.echo": &methodInfo{
					ShortName: "echo",
					ApiName:   "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/1.echo_api_endpoints_cloudesf_testing_cloud_goog/echo",
							HttpMethod:  util.POST,
						},
					},
					ApiKeyLocations: []*scpb.ApiKeyLocation{
						{
							Key: &scpb.ApiKeyLocation_Query{
								Query: "query_name",
							},
						},
					},
				},
			},
		},
		{
			desc: "Succeed, url query plus header",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
						Methods: []*apipb.Method{
							{
								Name: "echo",
							},
						},
					},
				},
				SystemParameters: &confpb.SystemParameters{
					Rules: []*confpb.SystemParameterRule{
						{
							Selector: "1.echo_api_endpoints_cloudesf_testing_cloud_goog.echo",
							Parameters: []*confpb.SystemParameter{
								{
									Name:              "api_key",
									HttpHeader:        "header_name_1",
									UrlQueryParameter: "query_name_1",
								},
								{
									Name:              "api_key",
									HttpHeader:        "header_name_2",
									UrlQueryParameter: "query_name_2",
								},
							},
						},
					},
				},
			},
			wantedAllTranscodingIgnoredQueryParams: map[string]bool{
				"query_name_1": true,
				"query_name_2": true,
			},
			wantMethods: map[string]*methodInfo{
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.echo": &methodInfo{
					ShortName: "echo",
					ApiName:   "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/1.echo_api_endpoints_cloudesf_testing_cloud_goog/echo",
							HttpMethod:  util.POST,
						},
					},
					ApiKeyLocations: []*scpb.ApiKeyLocation{
						{
							Key: &scpb.ApiKeyLocation_Query{
								Query: "query_name_1",
							},
						},
						{
							Key: &scpb.ApiKeyLocation_Query{
								Query: "query_name_2",
							},
						},
						{
							Key: &scpb.ApiKeyLocation_Header{
								Header: "header_name_1",
							},
						},
						{
							Key: &scpb.ApiKeyLocation_Header{
								Header: "header_name_2",
							},
						},
					},
				},
			},
		},

		{
			desc: "Succeed, url query plus header for multiple apis with one using default ApiKeyLocation",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
						Methods: []*apipb.Method{
							{
								Name: "foo",
							},
						},
					},
					{
						Name: "2.echo_api_endpoints_cloudesf_testing_cloud_goog",
						Methods: []*apipb.Method{
							{
								Name: "bar",
							},
						},
					},
					{
						Name: "3.echo_api_endpoints_cloudesf_testing_cloud_goog",
						Methods: []*apipb.Method{
							{
								Name: "baz",
							},
						},
					},
				},
				SystemParameters: &confpb.SystemParameters{
					Rules: []*confpb.SystemParameterRule{
						{
							Selector: "1.echo_api_endpoints_cloudesf_testing_cloud_goog.foo",
							Parameters: []*confpb.SystemParameter{
								{
									Name:              "api_key",
									HttpHeader:        "header_name_1",
									UrlQueryParameter: "query_name_1",
								},
								{
									Name:              "api_key",
									HttpHeader:        "header_name_2",
									UrlQueryParameter: "query_name_2",
								},
							},
						},
						{
							Selector: "2.echo_api_endpoints_cloudesf_testing_cloud_goog.bar",
							Parameters: []*confpb.SystemParameter{
								{
									Name:              "api_key",
									HttpHeader:        "header_name_1",
									UrlQueryParameter: "query_name_1",
								},
								{
									Name:              "api_key",
									HttpHeader:        "header_name_2",
									UrlQueryParameter: "query_name_2",
								},
							},
						},
					},
				},
			},
			wantedAllTranscodingIgnoredQueryParams: map[string]bool{
				"api_key":      true,
				"key":          true,
				"query_name_1": true,
				"query_name_2": true,
			},
			wantMethods: map[string]*methodInfo{
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.foo": {
					ShortName: "foo",
					ApiName:   "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/1.echo_api_endpoints_cloudesf_testing_cloud_goog/foo",
							HttpMethod:  util.POST,
						},
					},
					ApiKeyLocations: []*scpb.ApiKeyLocation{
						{
							Key: &scpb.ApiKeyLocation_Query{
								Query: "query_name_1",
							},
						},
						{
							Key: &scpb.ApiKeyLocation_Query{
								Query: "query_name_2",
							},
						},
						{
							Key: &scpb.ApiKeyLocation_Header{
								Header: "header_name_1",
							},
						},
						{
							Key: &scpb.ApiKeyLocation_Header{
								Header: "header_name_2",
							},
						},
					},
				},

				"2.echo_api_endpoints_cloudesf_testing_cloud_goog.bar": {
					ShortName: "bar",
					ApiName:   "2.echo_api_endpoints_cloudesf_testing_cloud_goog",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/2.echo_api_endpoints_cloudesf_testing_cloud_goog/bar",
							HttpMethod:  util.POST,
						},
					},
					ApiKeyLocations: []*scpb.ApiKeyLocation{
						{
							Key: &scpb.ApiKeyLocation_Query{
								Query: "query_name_1",
							},
						},
						{
							Key: &scpb.ApiKeyLocation_Query{
								Query: "query_name_2",
							},
						},
						{
							Key: &scpb.ApiKeyLocation_Header{
								Header: "header_name_1",
							},
						},
						{
							Key: &scpb.ApiKeyLocation_Header{
								Header: "header_name_2",
							},
						},
					},
				},
				"3.echo_api_endpoints_cloudesf_testing_cloud_goog.baz": {
					ShortName: "baz",
					ApiName:   "3.echo_api_endpoints_cloudesf_testing_cloud_goog",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/3.echo_api_endpoints_cloudesf_testing_cloud_goog/baz",
							HttpMethod:  util.POST,
						},
					},
				},
			},
		},
	}
	for i, tc := range testData {
		opts := options.DefaultConfigGeneratorOptions()
		opts.BackendAddress = "grpc://127.0.0.1:80"
		serviceInfo, err := NewServiceInfoFromServiceConfig(tc.fakeServiceConfig, testConfigID, opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(serviceInfo.Methods) != len(tc.wantMethods) {
			t.Errorf("Test Desc(%d): %s, got: %v, wanted: %v", i, tc.desc, serviceInfo.Methods, tc.wantMethods)
		}
		if !reflect.DeepEqual(serviceInfo.AllTranscodingIgnoredQueryParams, tc.wantedAllTranscodingIgnoredQueryParams) {
			t.Errorf("Test Desc(%d): %s, gotAllTranscodingIgnoredQueryParams: %v, wantedAllTranscodingIgnoredQueryParams: %v", i, tc.desc, serviceInfo.AllTranscodingIgnoredQueryParams, tc.wantedAllTranscodingIgnoredQueryParams)
		}

		for key, gotMethod := range serviceInfo.Methods {
			wantMethod := tc.wantMethods[key]
			if eq := cmp.Equal(gotMethod, wantMethod, cmp.Comparer(proto.Equal)); !eq {
				t.Errorf("Test Desc(%d): %s, \ngot: %v,\nwanted: %v", i, tc.desc, gotMethod, wantMethod)
			}
		}
	}
}

func TestProcessTranscodingIgnoredQueryParams(t *testing.T) {
	testData := []struct {
		desc                                   string
		fakeServiceConfig                      *confpb.Service
		transcodingIgnoredQueryParamsFlag      string
		wantedAllTranscodingIgnoredQueryParams map[string]bool
		wantedErrorPrefix                      string
	}{
		{
			desc: "Success. Default jwt locations with transcoding_ignore_query_params flag",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
						Methods: []*apipb.Method{
							{
								Name: "echo",
							},
						},
					},
				},
				Authentication: &confpb.Authentication{
					Providers: []*confpb.AuthProvider{
						{
							Id:     "auth_provider",
							Issuer: "issuer-0",
						},
					},
				},
			},
			transcodingIgnoredQueryParamsFlag: "foo,bar",
			wantedAllTranscodingIgnoredQueryParams: map[string]bool{
				"access_token": true,
				"foo":          true,
				"bar":          true,
			},
		},
		{
			desc: "Failure. Wrong jwt locations setting Query with valuePrefix in the same time",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
						Methods: []*apipb.Method{
							{
								Name: "echo",
							},
						},
					},
				},
				Authentication: &confpb.Authentication{
					Providers: []*confpb.AuthProvider{
						{
							Id:     "auth_provider",
							Issuer: "issuer-0",
							JwtLocations: []*confpb.JwtLocation{
								{
									In: &confpb.JwtLocation_Query{
										Query: "jwt_query_param",
									},
									ValuePrefix: "jwt_query_header_prefix",
								},
							},
						},
					},
				},
			},
			wantedErrorPrefix: `JwtLocation_Query should be set without valuePrefix`,
		},
		{
			desc: "Success. Custom jwt locations with transcoding_ignore_query_params flag",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
						Methods: []*apipb.Method{
							{
								Name: "echo",
							},
						},
					},
				},
				Authentication: &confpb.Authentication{
					Providers: []*confpb.AuthProvider{
						{
							Id:     "auth_provider",
							Issuer: "issuer-0",
							JwtLocations: []*confpb.JwtLocation{
								{
									In: &confpb.JwtLocation_Header{
										Header: "jwt_query_header",
									},
									ValuePrefix: "jwt_query_header_prefix",
								},
								{
									In: &confpb.JwtLocation_Query{
										Query: "jwt_query_param",
									},
								},
							},
						},
					},
				},
			},
			transcodingIgnoredQueryParamsFlag: "foo,bar",
			wantedAllTranscodingIgnoredQueryParams: map[string]bool{
				"jwt_query_param": true,
				"foo":             true,
				"bar":             true,
			},
		},
	}
	for i, tc := range testData {

		opts := options.DefaultConfigGeneratorOptions()
		opts.TranscodingIgnoreQueryParameters = tc.transcodingIgnoredQueryParamsFlag
		serviceInfo := &ServiceInfo{
			serviceConfig:                    tc.fakeServiceConfig,
			Methods:                          make(map[string]*methodInfo),
			AllTranscodingIgnoredQueryParams: make(map[string]bool),
			Options:                          opts,
		}

		err := serviceInfo.processTranscodingIgnoredQueryParams()
		if err != nil {
			if !strings.HasPrefix(err.Error(), tc.wantedErrorPrefix) {
				// Error doesn't match with wantedError.
				t.Errorf("Test Desc(%d): %s, gotError: %v, wantedErrorPrefix: %v", i, tc.desc, err.Error(), tc.wantedErrorPrefix)
			}

		} else if tc.wantedErrorPrefix != "" {
			// Error is empty while wantedError is not.
			t.Errorf("Test Desc(%d): %s, gotError: %v, wantedErrorPrefix: %v", i, tc.desc, err.Error(), tc.wantedErrorPrefix)

		} else if !reflect.DeepEqual(serviceInfo.AllTranscodingIgnoredQueryParams, tc.wantedAllTranscodingIgnoredQueryParams) {
			// Generated TranscoderIgnoreApiKeyQueryParams is not expected.
			t.Errorf("Test Desc(%d): %s, gotAllTranscodingIgnoredQueryParams: %v, wantedAllTranscodingIgnoredQueryParams: %v", i, tc.desc, serviceInfo.AllTranscodingIgnoredQueryParams, tc.wantedAllTranscodingIgnoredQueryParams)
		}
	}
}

func TestMethods(t *testing.T) {
	testData := []struct {
		desc              string
		fakeServiceConfig *confpb.Service
		BackendAddress    string
		healthz           string
		wantMethods       map[string]*methodInfo
		wantError         string
	}{
		{
			desc: "Succeed for gRPC, no Http rule, with Healthz",
			fakeServiceConfig: &confpb.Service{
				Name: testProjectName,
				Apis: []*apipb.Api{
					{
						Name: testApiName,
						Methods: []*apipb.Method{
							{
								Name: "ListShelves",
							},
							{
								Name: "CreateShelf",
							},
						},
					},
				},
			},
			BackendAddress: "grpc://127.0.0.1:80",
			healthz:        "/",
			wantMethods: map[string]*methodInfo{
				fmt.Sprintf("%s.%s", testApiName, "ListShelves"): &methodInfo{
					ShortName: "ListShelves",
					ApiName:   testApiName,
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: fmt.Sprintf("/%s/%s", testApiName, "ListShelves"),
							HttpMethod:  util.POST,
						},
					},
				},
				fmt.Sprintf("%s.%s", testApiName, "CreateShelf"): &methodInfo{
					ShortName: "CreateShelf",
					ApiName:   testApiName,
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: fmt.Sprintf("/%s/%s", testApiName, "CreateShelf"),
							HttpMethod:  util.POST,
						},
					},
				},
				fmt.Sprintf("%s.%s", "espv2_deployment", "ESPv2_Autogenerated_HealthCheck"): &methodInfo{
					ShortName:          "ESPv2_Autogenerated_HealthCheck",
					ApiName:            "espv2_deployment",
					SkipServiceControl: true,
					IsGenerated:        true,
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/",
							HttpMethod:  util.GET,
						},
					},
				},
			},
		},
		{
			desc: "Succeed for HTTP, with Healthz",
			fakeServiceConfig: &confpb.Service{
				Name: testProjectName,
				Apis: []*apipb.Api{
					{
						Name: "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
						Methods: []*apipb.Method{
							{
								Name: "Echo",
							},
							{
								Name: "Echo_Auth_Jwt",
							},
						},
					},
				},
				Http: &annotationspb.Http{
					Rules: []*annotationspb.HttpRule{
						{
							Selector: "1.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo_Auth_Jwt",
							Pattern: &annotationspb.HttpRule_Get{
								Get: "/auth/info/googlejwt",
							},
						},
						{
							Selector: "1.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo",
							Pattern: &annotationspb.HttpRule_Post{
								Post: "/echo",
							},
							Body: "message",
						},
					},
				},
			},
			BackendAddress: "http://127.0.0.1:80",
			healthz:        "/",
			wantMethods: map[string]*methodInfo{
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo": &methodInfo{
					ShortName: "Echo",
					ApiName:   "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/echo",
							HttpMethod:  util.POST,
						},
					},
				},
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo_Auth_Jwt": &methodInfo{
					ShortName: "Echo_Auth_Jwt",
					ApiName:   "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/auth/info/googlejwt",
							HttpMethod:  util.GET,
						},
					},
				},
				"espv2_deployment.ESPv2_Autogenerated_HealthCheck": &methodInfo{
					ShortName:          "ESPv2_Autogenerated_HealthCheck",
					ApiName:            "espv2_deployment",
					SkipServiceControl: true,
					IsGenerated:        true,
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/",
							HttpMethod:  util.GET,
						},
					},
				},
			},
		},
		{
			desc: "Succeed for HTTP with multiple apis",
			fakeServiceConfig: &confpb.Service{
				Name: testProjectName,
				Apis: []*apipb.Api{
					{
						Name: "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
						Methods: []*apipb.Method{
							{
								Name: "Echo",
							},
							{
								Name: "Echo_Auth_Jwt",
							},
						},
					},
					{
						Name: "2.echo_api_endpoints_cloudesf_testing_cloud_goog",
						Methods: []*apipb.Method{
							{
								Name: "Echo",
							},
							{
								Name: "Echo_Auth_Jwt",
							},
						},
					},
				},
				Http: &annotationspb.Http{
					Rules: []*annotationspb.HttpRule{
						{
							Selector: "1.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo_Auth_Jwt",
							Pattern: &annotationspb.HttpRule_Get{
								Get: "/auth/info/googlejwt",
							},
						},
						{
							Selector: "1.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo",
							Pattern: &annotationspb.HttpRule_Post{
								Post: "/echo",
							},
							Body: "message",
						},
						{
							Selector: "2.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo_Auth_Jwt",
							Pattern: &annotationspb.HttpRule_Get{
								Get: "/auth/info/googlejwt",
							},
						},
						{
							Selector: "2.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo",
							Pattern: &annotationspb.HttpRule_Post{
								Post: "/echo",
							},
							Body: "message",
						},
					},
				},
			},
			BackendAddress: "http://127.0.0.1:80",
			wantMethods: map[string]*methodInfo{
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo": &methodInfo{
					ShortName: "Echo",
					ApiName:   "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/echo",
							HttpMethod:  util.POST,
						},
					},
				},
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo_Auth_Jwt": &methodInfo{
					ShortName: "Echo_Auth_Jwt",
					ApiName:   "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/auth/info/googlejwt",
							HttpMethod:  util.GET,
						},
					},
				},
				"2.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo": &methodInfo{
					ShortName: "Echo",
					ApiName:   "2.echo_api_endpoints_cloudesf_testing_cloud_goog",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/echo",
							HttpMethod:  util.POST,
						},
					},
				},
				"2.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo_Auth_Jwt": &methodInfo{
					ShortName: "Echo_Auth_Jwt",
					ApiName:   "2.echo_api_endpoints_cloudesf_testing_cloud_goog",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/auth/info/googlejwt",
							HttpMethod:  util.GET,
						},
					},
				},
			},
		},
		{
			desc: "Succeed for HTTP, with OPTIONS, and AllowCors, with Healthz",
			fakeServiceConfig: &confpb.Service{
				Name: testProjectName,
				Apis: []*apipb.Api{
					{
						Name:    "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
						Version: "1.2.3",
						Methods: []*apipb.Method{
							{
								Name: "Echo",
							},
							{
								Name: "Echo_Auth",
							},
							{
								Name: "Echo_Auth_Jwt",
							},
							{
								Name: "EchoCors",
							},
							{
								Name: "GetBook",
							},
							{
								Name: "UpdateBook",
							},
							{
								Name: "DeleteBook",
							},
						},
					},
				},
				Endpoints: []*confpb.Endpoint{
					{
						Name:      testProjectName,
						AllowCors: true,
					},
				},
				Http: &annotationspb.Http{
					Rules: []*annotationspb.HttpRule{
						{
							Selector: "1.echo_api_endpoints_cloudesf_testing_cloud_goog.EchoCors",
							Pattern: &annotationspb.HttpRule_Custom{
								Custom: &annotationspb.CustomHttpPattern{
									Kind: "OPTIONS",
									Path: "/echo",
								},
							},
						},
						{
							Selector: "1.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo",
							Pattern: &annotationspb.HttpRule_Post{
								Post: "/echo",
							},
							Body: "message",
						},
						{
							Selector: "1.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo_Auth_Jwt",
							Pattern: &annotationspb.HttpRule_Get{
								Get: "/auth/info/googlejwt",
							},
						},
						{
							Selector: "1.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo_Auth",
							Pattern: &annotationspb.HttpRule_Post{
								Post: "/auth/info/googlejwt",
							},
						},
						{
							// This will also have a CORS method generated for it.
							Selector: "1.echo_api_endpoints_cloudesf_testing_cloud_goog.GetBook",
							Pattern: &annotationspb.HttpRule_Get{
								Get: "/shelves/{shelf_id}/books/{book.id}",
							},
						},
						{
							// No CORS method generated due to an equivalent path.
							Selector: "1.echo_api_endpoints_cloudesf_testing_cloud_goog.UpdateBook",
							Pattern: &annotationspb.HttpRule_Patch{
								Patch: "/shelves/{shelf_id}/books/{book.id}",
							},
						},
						{
							// No CORS method generated due to a **semantically** equivalent path.
							Selector: "1.echo_api_endpoints_cloudesf_testing_cloud_goog.DeleteBook",
							Pattern: &annotationspb.HttpRule_Delete{
								Delete: "/shelves/{shelf_id_different_param}/books/{book.id}",
							},
						},
					},
				},
			},
			BackendAddress: "http://127.0.0.1:80",
			healthz:        "/healthz",
			wantMethods: map[string]*methodInfo{
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.EchoCors": &methodInfo{
					ShortName:  "EchoCors",
					ApiName:    "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					ApiVersion: "1.2.3",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/echo",
							HttpMethod:  util.OPTIONS,
						},
					},
				},
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo": &methodInfo{
					ShortName:  "Echo",
					ApiName:    "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					ApiVersion: "1.2.3",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/echo",
							HttpMethod:  util.POST,
						},
					},
				},
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.GetBook": &methodInfo{
					ShortName:  "GetBook",
					ApiName:    "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					ApiVersion: "1.2.3",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/shelves/{shelf_id}/books/{book.id}",
							HttpMethod:  util.GET,
						},
					},
				},
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.ESPv2_Autogenerated_CORS_shelves_shelf_id_books_book.id": &methodInfo{
					ShortName:  "ESPv2_Autogenerated_CORS_shelves_shelf_id_books_book.id",
					ApiVersion: "1.2.3",
					ApiName:    "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/shelves/{shelf_id}/books/{book.id}",
							HttpMethod:  util.OPTIONS,
						},
					},
					IsGenerated: true,
				},
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.UpdateBook": &methodInfo{
					ShortName:  "UpdateBook",
					ApiName:    "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					ApiVersion: "1.2.3",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/shelves/{shelf_id}/books/{book.id}",
							HttpMethod:  util.PATCH,
						},
					},
				},
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.DeleteBook": &methodInfo{
					ShortName:  "DeleteBook",
					ApiName:    "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					ApiVersion: "1.2.3",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/shelves/{shelf_id_different_param}/books/{book.id}",
							HttpMethod:  util.DELETE,
						},
					},
				},
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.ESPv2_Autogenerated_CORS_auth_info_googlejwt": &methodInfo{
					ShortName:  "ESPv2_Autogenerated_CORS_auth_info_googlejwt",
					ApiName:    "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					ApiVersion: "1.2.3",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/auth/info/googlejwt",
							HttpMethod:  util.OPTIONS,
						},
					},
					IsGenerated: true,
				},
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo_Auth_Jwt": &methodInfo{
					ShortName:  "Echo_Auth_Jwt",
					ApiName:    "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					ApiVersion: "1.2.3",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/auth/info/googlejwt",
							HttpMethod:  util.GET,
						},
					},
				},
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.Echo_Auth": &methodInfo{
					ShortName:  "Echo_Auth",
					ApiName:    "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					ApiVersion: "1.2.3",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/auth/info/googlejwt",
							HttpMethod:  util.POST,
						},
					},
				},
				"espv2_deployment.ESPv2_Autogenerated_HealthCheck": &methodInfo{
					ShortName:          "ESPv2_Autogenerated_HealthCheck",
					ApiName:            "espv2_deployment",
					SkipServiceControl: true,
					IsGenerated:        true,
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/healthz",
							HttpMethod:  util.GET,
						},
					},
				},
			},
		},
		{
			desc: "Succeed for HTTP with custom method and allow CORS",
			fakeServiceConfig: &confpb.Service{
				Name: testProjectName,
				Apis: []*apipb.Api{
					{
						Name: "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
						Methods: []*apipb.Method{
							{
								Name: "EchoCustom",
							},
						},
					},
				},
				Endpoints: []*confpb.Endpoint{
					{
						Name:      testProjectName,
						AllowCors: true,
					},
				},
				Http: &annotationspb.Http{
					Rules: []*annotationspb.HttpRule{
						{
							Selector: "1.echo_api_endpoints_cloudesf_testing_cloud_goog.EchoCustom",
							Pattern: &annotationspb.HttpRule_Custom{
								Custom: &annotationspb.CustomHttpPattern{
									Kind: "CustomMethod",
									Path: "/echo/test",
								},
							},
						},
					},
				},
			},
			BackendAddress: "http://127.0.0.1:80",
			wantMethods: map[string]*methodInfo{
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.EchoCustom": &methodInfo{
					ShortName: "EchoCustom",
					ApiName:   "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/echo/test",
							HttpMethod:  "CustomMethod",
						},
					},
				},
				// CORS method generated for the above path.
				"1.echo_api_endpoints_cloudesf_testing_cloud_goog.ESPv2_Autogenerated_CORS_echo_test": &methodInfo{
					ShortName: "ESPv2_Autogenerated_CORS_echo_test",
					ApiName:   "1.echo_api_endpoints_cloudesf_testing_cloud_goog",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/echo/test",
							HttpMethod:  util.OPTIONS,
						},
					},
					IsGenerated: true,
				},
			},
		},
		{
			desc: "Succeed for multiple url Pattern",
			fakeServiceConfig: &confpb.Service{
				Name: testProjectName,
				Apis: []*apipb.Api{
					{
						Name: "endpoints.examples.bookstore.Bookstore",
						Methods: []*apipb.Method{
							{
								Name:            "CreateBook",
								RequestTypeUrl:  "type.googleapis.com/endpoints.examples.bookstore.CreateBookRequest",
								ResponseTypeUrl: "type.googleapis.com/endpoints.examples.bookstore.Book",
							},
						},
					},
				},
				Http: &annotationspb.Http{
					Rules: []*annotationspb.HttpRule{
						{
							Selector: "endpoints.examples.bookstore.Bookstore.CreateBook",
							Pattern: &annotationspb.HttpRule_Post{
								Post: "/v1/shelves/{shelf}/books/{book.id}/{book.author}",
							},
							Body: "book.title",
						},
						{
							Selector: "endpoints.examples.bookstore.Bookstore.CreateBook",
							Pattern: &annotationspb.HttpRule_Post{
								Post: "/v1/shelves/{shelf}/books",
							},
							Body: "book",
						},
					},
				},
				Types: []*ptypepb.Type{
					{
						Name: "endpoints.examples.bookstore.CreateBookRequest",
					},
				},
			},
			BackendAddress: "grpc://127.0.0.1:80",
			wantMethods: map[string]*methodInfo{
				"endpoints.examples.bookstore.Bookstore.CreateBook": &methodInfo{
					ShortName:       "CreateBook",
					ApiName:         "endpoints.examples.bookstore.Bookstore",
					RequestTypeName: "endpoints.examples.bookstore.CreateBookRequest",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/v1/shelves/{shelf}/books/{book.id}/{book.author}",
							HttpMethod:  util.POST,
						},
						{
							UriTemplate: "/v1/shelves/{shelf}/books",
							HttpMethod:  util.POST,
						},
						{
							UriTemplate: "/endpoints.examples.bookstore.Bookstore/CreateBook",
							HttpMethod:  util.POST,
						},
					},
				},
			},
		},
		{
			desc: "Succeed for additional binding",
			fakeServiceConfig: &confpb.Service{
				Name: testProjectName,
				Apis: []*apipb.Api{
					{
						Name: "endpoints.examples.bookstore.Bookstore",
						Methods: []*apipb.Method{
							{
								Name:            "CreateBook",
								RequestTypeUrl:  "type.googleapis.com/endpoints.examples.bookstore.CreateBookRequest",
								ResponseTypeUrl: "type.googleapis.com/endpoints.examples.bookstore.Book",
							},
						},
					},
				},
				Http: &annotationspb.Http{
					Rules: []*annotationspb.HttpRule{
						{
							Selector: "endpoints.examples.bookstore.Bookstore.CreateBook",
							Pattern: &annotationspb.HttpRule_Post{
								Post: "/v1/shelves/{shelf}/books/{book.id}/{book.author}",
							},
							Body: "book.title",
							AdditionalBindings: []*annotationspb.HttpRule{
								{
									Pattern: &annotationspb.HttpRule_Post{
										Post: "/v1/shelves/{shelf}/books/foo",
									},
									Body: "book",
								},
								{
									Pattern: &annotationspb.HttpRule_Post{
										Post: "/v1/shelves/{shelf}/books/bar",
									},
									Body: "book",
								},
							},
						},
					},
				},
				Types: []*ptypepb.Type{
					{
						Name: "endpoints.examples.bookstore.CreateBookRequest",
					},
				},
			},
			BackendAddress: "grpc://127.0.0.1:80",
			wantMethods: map[string]*methodInfo{
				"endpoints.examples.bookstore.Bookstore.CreateBook": &methodInfo{
					ShortName:       "CreateBook",
					ApiName:         "endpoints.examples.bookstore.Bookstore",
					RequestTypeName: "endpoints.examples.bookstore.CreateBookRequest",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/v1/shelves/{shelf}/books/{book.id}/{book.author}",
							HttpMethod:  util.POST,
						},
						{
							UriTemplate: "/v1/shelves/{shelf}/books/foo",
							HttpMethod:  util.POST,
						},
						{
							UriTemplate: "/v1/shelves/{shelf}/books/bar",
							HttpMethod:  util.POST,
						},
						{
							UriTemplate: "/endpoints.examples.bookstore.Bookstore/CreateBook",
							HttpMethod:  util.POST,
						},
					},
				},
			},
		},
	}

	for _, tc := range testData {
		opts := options.DefaultConfigGeneratorOptions()
		opts.BackendAddress = tc.BackendAddress
		opts.Healthz = tc.healthz
		serviceInfo, err := NewServiceInfoFromServiceConfig(tc.fakeServiceConfig, testConfigID, opts)
		if tc.wantError != "" {
			if err == nil || err.Error() != tc.wantError {
				t.Errorf("Test(%v): got Errors : %v, want: %v", tc.desc, err, tc.wantError)
			}
			continue
		}
		if err != nil {
			t.Errorf("Test(%v): got err %v", tc.desc, err)
			continue
		}
		if len(serviceInfo.Methods) != len(tc.wantMethods) {
			t.Errorf("Test(%v): diff in number of Methods, got: %v, want: %v", tc.desc, len(serviceInfo.Methods), len(tc.wantMethods))
			continue
		}
		for key, gotMethod := range serviceInfo.Methods {
			wantMethod, ok := tc.wantMethods[key]
			if !ok {
				t.Errorf("Test(%v): \n cannot find key: %v\n got methods: %+v\nwant methods: %+v", tc.desc, key, serviceInfo.Methods, tc.wantMethods)
			}

			if eq := cmp.Equal(gotMethod, wantMethod, cmp.Comparer(proto.Equal)); !eq {
				t.Errorf("Test(%v): \n got method: %v, \nwant method: %v", tc.desc, gotMethod, wantMethod)
			}
		}
	}
}

func TestProcessBackendRuleForDeadline(t *testing.T) {
	testData := []struct {
		desc              string
		fakeServiceConfig *confpb.Service
		// Map of selector to the expected deadline for the corresponding route.
		wantedMethodDeadlines map[string]time.Duration
	}{
		{
			desc: "Mixed deadlines across multiple backend rules",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
				Backend: &confpb.Backend{
					Rules: []*confpb.BackendRule{
						{
							Address:  "grpc://abc.com/api/",
							Selector: "abc.com.api",
							Deadline: 10.5,
						},
						{
							Address:  "grpc://cnn.com/api/",
							Selector: "cnn.com.api",
							Deadline: 20,
						},
					},
				},
			},
			wantedMethodDeadlines: map[string]time.Duration{
				"abc.com.api": 10*time.Second + 500*time.Millisecond,
				"cnn.com.api": 20 * time.Second,
			},
		},
		{
			desc: "Deadline with high precision is rounded to milliseconds",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
				Backend: &confpb.Backend{
					Rules: []*confpb.BackendRule{
						{
							Address:  "grpc://abc.com/api/",
							Selector: "abc.com.api",
							Deadline: 30.0009, // 30s 0.9ms
						},
					},
				},
			},
			wantedMethodDeadlines: map[string]time.Duration{
				"abc.com.api": 30*time.Second + 1*time.Millisecond,
			},
		},
		{
			desc: "Deadline that is non-positive is overridden to default",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
				Backend: &confpb.Backend{
					Rules: []*confpb.BackendRule{
						{
							Address:  "grpc://abc.com/api/",
							Selector: "abc.com.api",
							Deadline: -10.5,
						},
					},
				},
			},
			wantedMethodDeadlines: map[string]time.Duration{
				"abc.com.api": util.DefaultResponseDeadline,
			},
		},
	}

	for i, tc := range testData {
		opts := options.DefaultConfigGeneratorOptions()
		s, err := NewServiceInfoFromServiceConfig(tc.fakeServiceConfig, testConfigID, opts)

		if err != nil {
			t.Errorf("Test Desc(%d): %s, TestProcessBackendRuleForDeadline error not expected, got: %v", i, tc.desc, err)
			return
		}

		for _, rule := range tc.fakeServiceConfig.Backend.Rules {
			gotDeadline := s.Methods[rule.Selector].BackendInfo.Deadline
			wantDeadline := tc.wantedMethodDeadlines[rule.Selector]

			if wantDeadline != gotDeadline {
				t.Errorf("Test Desc(%d): %s, TestProcessBackendRuleForDeadline, Deadline not expected, got: %v, want: %v", i, tc.desc, gotDeadline, wantDeadline)
			}
		}
	}
}

func TestProcessBackendRuleForProtocol(t *testing.T) {
	testData := []struct {
		desc              string
		fakeServiceConfig *confpb.Service
		// Map of cluster name to the expected backend protocol for the backend routing cluster.
		wantedClusterProtocols map[string]util.BackendProtocol
	}{
		{
			desc: "Mixed protocols across multiple backend rules",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
				Backend: &confpb.Backend{
					Rules: []*confpb.BackendRule{
						{
							Address:  "https://abc.com/api/",
							Selector: "abc.com.api",
							Protocol: "http/1.1",
						},
						{
							Address:  "https://cnn.com/api/",
							Selector: "cnn.com.api",
							Protocol: "h2",
						},
					},
				},
			},
			wantedClusterProtocols: map[string]util.BackendProtocol{
				"abc.com:443": util.HTTP1,
				"cnn.com:443": util.HTTP2,
			},
		},
		{
			// This case is not supported in practice, but we shouldn't break ordering if a user does it.
			desc: "When multiple backend rules with the same address have different protocols, only first one is used",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
				Backend: &confpb.Backend{
					Rules: []*confpb.BackendRule{
						{
							Address:  "https://abc.com/api/",
							Selector: "api.test.1",
							Protocol: "http/1.1",
						},
						{
							Address:  "https://abc.com/api/",
							Selector: "api.test.2",
							Protocol: "h2",
						},
					},
				},
			},
			wantedClusterProtocols: map[string]util.BackendProtocol{
				"abc.com:443": util.HTTP1,
			},
		},
	}

	for _, tc := range testData {
		opts := options.DefaultConfigGeneratorOptions()
		s, err := NewServiceInfoFromServiceConfig(tc.fakeServiceConfig, testConfigID, opts)

		if err != nil {
			t.Errorf("Test Desc(%s): error not expected, got: %v", tc.desc, err)
			return
		}

		for _, gotBackendRoutingCluster := range s.BackendRoutingClusters {
			gotProtocol := gotBackendRoutingCluster.Protocol
			wantProtocol, ok := tc.wantedClusterProtocols[gotBackendRoutingCluster.ClusterName]

			if !ok {
				t.Errorf("Test Desc(%s): Unknown backend routing cluster generated: %+v", tc.desc, gotBackendRoutingCluster)
				continue
			}

			if wantProtocol != gotProtocol {
				t.Errorf("Test Desc(%s): Protocol not expected, got: %v, want: %v", tc.desc, gotProtocol, wantProtocol)
			}
		}
	}
}

func TestProcessBackendRuleForClusterName(t *testing.T) {
	testData := []struct {
		desc        string
		Address     string
		ClusterName string
	}{
		{
			desc:        "Domain name with default http port",
			Address:     "http://abc.com/api/",
			ClusterName: "abc.com:80",
		},
		{
			desc:        "Domain name with default https port",
			Address:     "https://abc.com/api/",
			ClusterName: "abc.com:443",
		},
		{
			desc:        "Domain name with default grpc port",
			Address:     "grpc://abc.com/api/",
			ClusterName: "abc.com:80",
		},
		{
			desc:        "Domain name with default grpcs port",
			Address:     "grpcs://abc.com/api/",
			ClusterName: "abc.com:443",
		},
		{
			desc:        "Domain name with custom http port",
			Address:     "http://abc.com:8080/api/",
			ClusterName: "abc.com:8080",
		},
		{
			desc:        "Domain name with custom https port",
			Address:     "https://abc.com:8080/api/",
			ClusterName: "abc.com:8080",
		},
		{
			desc:        "Domain name with custom grpc port",
			Address:     "grpc://abc.com:8080/api/",
			ClusterName: "abc.com:8080",
		},
		{
			desc:        "Domain name with custom grpcs port",
			Address:     "grpcs://abc.com:8080/api/",
			ClusterName: "abc.com:8080",
		},
		{
			desc:        "IP with default http port",
			Address:     "http://127.0.0.1/api/",
			ClusterName: "127.0.0.1:80",
		},
		{
			desc:        "IP with default https port",
			Address:     "https://127.0.0.1/api/",
			ClusterName: "127.0.0.1:443",
		},
		{
			desc:        "IP with default grpc port",
			Address:     "grpc://127.0.0.1/api/",
			ClusterName: "127.0.0.1:80",
		},
		{
			desc:        "IP with default grpcs port",
			Address:     "grpcs://127.0.0.1/api/",
			ClusterName: "127.0.0.1:443",
		},
		{
			desc:        "IP with custom http port",
			Address:     "http://127.0.0.1:8080/api/",
			ClusterName: "127.0.0.1:8080",
		},
		{
			desc:        "IP with custom https port",
			Address:     "https://127.0.0.1:8080/api/",
			ClusterName: "127.0.0.1:8080",
		},
		{
			desc:        "IP with custom grpc port",
			Address:     "grpc://127.0.0.1:8080/api/",
			ClusterName: "127.0.0.1:8080",
		},
		{
			desc:        "IP with custom grpcs port",
			Address:     "grpcs://127.0.0.1:8080/api/",
			ClusterName: "127.0.0.1:8080",
		},
	}

	for _, tc := range testData {
		fakeServiceConfig := &confpb.Service{
			Apis: []*apipb.Api{
				{
					Name: testApiName,
				},
			},
			Backend: &confpb.Backend{
				Rules: []*confpb.BackendRule{
					{
						Address:  tc.Address,
						Selector: "http.abc.com.api",
					},
				},
			},
		}
		opts := options.DefaultConfigGeneratorOptions()
		s, err := NewServiceInfoFromServiceConfig(fakeServiceConfig, testConfigID, opts)

		if err != nil {
			t.Errorf("Test Desc(%s): error not expected, got: %v", tc.desc, err)
			return
		}

		if len(s.BackendRoutingClusters) != 1 {
			t.Errorf("Test Desc(%s): generated number of clusters is not 1", tc.desc)
			return
		}
		if tc.ClusterName != s.BackendRoutingClusters[0].ClusterName {
			t.Errorf("Test Desc(%s): cluster name is different, want: %s, got %s", tc.desc, tc.ClusterName, s.BackendRoutingClusters[0].ClusterName)
		}
	}
}

func TestProcessBackendRuleForJwtAudience(t *testing.T) {
	testData := []struct {
		desc              string
		fakeServiceConfig *confpb.Service

		wantedJwtAudience map[string]string
	}{

		{
			desc: "DisableAuth is set to true",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
				Backend: &confpb.Backend{
					Rules: []*confpb.BackendRule{

						{
							Address:        "grpc://abc.com/api",
							Selector:       "abc.com.api",
							Deadline:       10.5,
							Authentication: &confpb.BackendRule_DisableAuth{DisableAuth: true},
						},
					},
				},
			},
			wantedJwtAudience: map[string]string{
				"abc.com.api": "",
			},
		},
		{
			desc: "DisableAuth is set to false",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
				Backend: &confpb.Backend{
					Rules: []*confpb.BackendRule{

						{
							Address:        "grpc://abc.com/api",
							Selector:       "abc.com.api",
							Deadline:       10.5,
							Authentication: &confpb.BackendRule_DisableAuth{DisableAuth: false},
						},
					},
				},
			},
			wantedJwtAudience: map[string]string{
				"abc.com.api": "http://abc.com",
			},
		},
		{
			desc: "Authentication field is empty and grpc scheme is changed to http",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
				Backend: &confpb.Backend{
					Rules: []*confpb.BackendRule{

						{
							Address:  "grpc://abc.com/api",
							Selector: "abc.com.api",
							Deadline: 10.5,
						},
					},
				},
			},
			wantedJwtAudience: map[string]string{
				"abc.com.api": "http://abc.com",
			},
		},
		{
			desc: "Authentication field is empty and grpcs scheme is changed to https",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
				Backend: &confpb.Backend{
					Rules: []*confpb.BackendRule{

						{
							Address:  "grpcs://abc.com/api",
							Selector: "abc.com.api",
							Deadline: 10.5,
						},
					},
				},
			},
			wantedJwtAudience: map[string]string{
				"abc.com.api": "https://abc.com",
			},
		},
		{
			desc: "JwtAudience is set",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
				Backend: &confpb.Backend{
					Rules: []*confpb.BackendRule{
						{
							Address:        "grpc://abc.com/api",
							Selector:       "abc.com.api",
							Deadline:       10.5,
							Authentication: &confpb.BackendRule_JwtAudience{JwtAudience: "audience-foo"},
						},
					},
				},
			},
			wantedJwtAudience: map[string]string{
				"abc.com.api": "audience-foo",
			},
		},
		{
			desc: "Mix all Authentication cases",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
				Backend: &confpb.Backend{
					Rules: []*confpb.BackendRule{
						{
							Address:        "grpc://abc.com/api",
							Selector:       "abc.com.api",
							Deadline:       10.5,
							Authentication: &confpb.BackendRule_JwtAudience{JwtAudience: "audience-foo"},
						},
						{
							Address:        "grpc://def.com/api",
							Selector:       "def.com.api",
							Deadline:       10.5,
							Authentication: &confpb.BackendRule_JwtAudience{JwtAudience: "audience-bar"},
						},
						{
							Address:        "grpc://ghi.com/api",
							Selector:       "ghi.com.api",
							Deadline:       10.5,
							Authentication: &confpb.BackendRule_DisableAuth{DisableAuth: false},
						},
						{
							Address:        "grpc://jkl.com/api",
							Selector:       "jkl.com.api",
							Deadline:       10.5,
							Authentication: &confpb.BackendRule_DisableAuth{DisableAuth: true},
						},
						{
							Address:  "grpcs://mno.com/api",
							Selector: "mno.com.api",
							Deadline: 10.5,
						},
					},
				},
			},
			wantedJwtAudience: map[string]string{
				"abc.com.api": "audience-foo",
				"def.com.api": "audience-bar",
				"ghi.com.api": "http://ghi.com",
				"jkl.com.api": "",
				"mno.com.api": "https://mno.com",
			},
		},
	}

	for i, tc := range testData {
		opts := options.DefaultConfigGeneratorOptions()
		s, err := NewServiceInfoFromServiceConfig(tc.fakeServiceConfig, testConfigID, opts)

		if err != nil {
			t.Errorf("Test Desc(%d): %s, error not expected, got: %v", i, tc.desc, err)
			return
		}

		for _, rule := range tc.fakeServiceConfig.Backend.Rules {
			gotJwtAudience := s.Methods[rule.Selector].BackendInfo.JwtAudience
			wantedJwtAudience := tc.wantedJwtAudience[rule.Selector]

			if wantedJwtAudience != gotJwtAudience {
				t.Errorf("Test Desc(%d): %s, JwtAudience not expected, got: %v, want: %v", i, tc.desc, gotJwtAudience, wantedJwtAudience)
			}
		}
	}
}

func TestProcessQuota(t *testing.T) {
	testData := []struct {
		desc              string
		fakeServiceConfig *confpb.Service
		wantMethods       map[string]*methodInfo
	}{
		{
			desc: "Succeed, simple case",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: testApiName,
						Methods: []*apipb.Method{
							{
								Name: "ListShelves",
							},
						},
					},
				},
				Quota: &confpb.Quota{
					MetricRules: []*confpb.MetricRule{
						{
							Selector: "endpoints.examples.bookstore.Bookstore.ListShelves",
							MetricCosts: map[string]int64{
								"metric_a": 2,
								"metric_b": 1,
							},
						},
					},
				},
			},
			wantMethods: map[string]*methodInfo{
				fmt.Sprintf("%s.%s", testApiName, "ListShelves"): &methodInfo{
					ShortName: "ListShelves",
					ApiName:   testApiName,
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: fmt.Sprintf("/%s/%s", testApiName, "ListShelves"),
							HttpMethod:  util.POST,
						},
					},
					MetricCosts: []*scpb.MetricCost{
						{
							Name: "metric_a",
							Cost: 2,
						},
						{
							Name: "metric_b",
							Cost: 1,
						},
					},
				},
			},
		},
		{
			desc: "Succeed, two metric cost items",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: testApiName,
						Methods: []*apipb.Method{
							{
								Name: "ListShelves",
							},
						},
					},
				},
				Quota: &confpb.Quota{
					MetricRules: []*confpb.MetricRule{
						{
							Selector: "endpoints.examples.bookstore.Bookstore.ListShelves",
							MetricCosts: map[string]int64{
								"metric_c": 2,
								"metric_a": 3,
							},
						},
					},
				},
			},
			wantMethods: map[string]*methodInfo{
				fmt.Sprintf("%s.%s", testApiName, "ListShelves"): &methodInfo{
					ShortName: "ListShelves",
					ApiName:   testApiName,
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: fmt.Sprintf("/%s/%s", testApiName, "ListShelves"),
							HttpMethod:  util.POST,
						},
					},
					MetricCosts: []*scpb.MetricCost{
						{
							Name: "metric_a",
							Cost: 3,
						},
						{
							Name: "metric_c",
							Cost: 2,
						},
					},
				},
			},
		},
	}

	for i, tc := range testData {
		opts := options.DefaultConfigGeneratorOptions()
		opts.BackendAddress = "grpc://127.0.0.1:80"
		serviceInfo, _ := NewServiceInfoFromServiceConfig(tc.fakeServiceConfig, testConfigID, opts)

		for key, gotMethod := range serviceInfo.Methods {
			wantMethod := tc.wantMethods[key]

			sort.Slice(gotMethod.MetricCosts, func(i, j int) bool { return gotMethod.MetricCosts[i].Name < gotMethod.MetricCosts[j].Name })
			if eq := cmp.Equal(gotMethod, wantMethod, cmp.Comparer(proto.Equal)); !eq {
				t.Errorf("Test Desc(%d): %s,\ngot Method: %v,\nwant Method: %v", i, tc.desc, gotMethod, wantMethod)
			}
		}
	}
}

func TestProcessEmptyJwksUriByOpenID(t *testing.T) {
	r := mux.NewRouter()
	jwksUriEntry, _ := json.Marshal(map[string]string{"jwks_uri": "this-is-jwksUri"})
	r.Path(util.OpenIDDiscoveryCfgURLSuffix).Methods("GET").Handler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(jwksUriEntry)
		}))
	openIDServer := httptest.NewServer(r)

	testData := []struct {
		desc              string
		fakeServiceConfig *confpb.Service
		wantedJwksUri     string
		wantErr           bool
	}{
		{
			desc: "Empty jwksUri, use jwksUri acquired by openID",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
				Authentication: &confpb.Authentication{
					Providers: []*confpb.AuthProvider{
						{
							Id:     "auth_provider",
							Issuer: openIDServer.URL,
						},
					},
				},
			},
			wantedJwksUri: "this-is-jwksUri",
		},
		{
			desc: "Empty jwksUri and Open ID Connect Discovery failed",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: testApiName,
					},
				},
				Authentication: &confpb.Authentication{
					Providers: []*confpb.AuthProvider{
						{
							Id:     "auth_provider",
							Issuer: "aaaaa.bbbbbb.ccccc/inaccessible_uri/",
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for i, tc := range testData {
		opts := options.DefaultConfigGeneratorOptions()
		serviceInfo, err := NewServiceInfoFromServiceConfig(tc.fakeServiceConfig, testConfigID, opts)

		if tc.wantErr {
			if err == nil {
				t.Errorf("Test Desc(%d): %s, process jwksUri got: no err, but expected err", i, tc.desc)
			}
		} else if err != nil {
			t.Errorf("Test Desc(%d): %s, process jwksUri got: %v, but expected no err", i, tc.desc, err)
		} else if jwksUri := serviceInfo.serviceConfig.Authentication.Providers[0].JwksUri; jwksUri != tc.wantedJwksUri {
			t.Errorf("Test Desc(%d): %s, process jwksUri got: %v, want: %v", i, tc.desc, jwksUri, tc.wantedJwksUri)
		}
	}
}

func TestProcessApis(t *testing.T) {
	testData := []struct {
		desc              string
		fakeServiceConfig *confpb.Service
		wantMethods       map[string]*methodInfo
		wantApiNames      []string
	}{
		{
			desc: "Succeed, process multiple apis",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: "api-1",
						Methods: []*apipb.Method{
							{
								Name:           "foo",
								RequestTypeUrl: "type.googleapis.com/google.protobuf.Empty",
							},
							{
								Name:           "bar",
								RequestTypeUrl: "type.googleapis.com/CreateShelfRequest",
							},
						},
					},
					{
						Name: "api-2",
						Methods: []*apipb.Method{
							{
								Name:           "foo",
								RequestTypeUrl: "type.googleapis.com/CreateBookRequest",
							},
							{
								Name:           "bar",
								RequestTypeUrl: "type.googleapis.com/CreateShelfRequest",
							},
						},
					},
					{
						Name:    "api-3",
						Methods: []*apipb.Method{},
					},
					{
						Name: "api-4",
						Methods: []*apipb.Method{
							{
								Name: "bar",
								// Missing request type.
							},
							{
								Name:           "baz",
								RequestTypeUrl: "types.MALFORMED.com/google.protobuf.Empty",
							},
						},
					},
				},
			},
			wantMethods: map[string]*methodInfo{
				"api-1.foo": {
					ShortName:       "foo",
					ApiName:         "api-1",
					RequestTypeName: "google.protobuf.Empty",
				},
				"api-1.bar": {
					ShortName:       "bar",
					ApiName:         "api-1",
					RequestTypeName: "CreateShelfRequest",
				},
				"api-2.foo": {
					ShortName:       "foo",
					ApiName:         "api-2",
					RequestTypeName: "CreateBookRequest",
				},
				"api-2.bar": {
					ShortName:       "bar",
					ApiName:         "api-2",
					RequestTypeName: "CreateShelfRequest",
				},
				"api-4.bar": {
					ShortName: "bar",
					ApiName:   "api-4",
				},
				"api-4.baz": {
					ShortName: "baz",
					ApiName:   "api-4",
				},
			},
			wantApiNames: []string{
				"api-1",
				"api-2",
				"api-3",
				"api-4",
			},
		},
	}

	for _, tc := range testData {

		serviceInfo := &ServiceInfo{
			serviceConfig: tc.fakeServiceConfig,
			Methods:       make(map[string]*methodInfo),
		}
		serviceInfo.processApis()

		for key, gotMethod := range serviceInfo.Methods {
			wantMethod := tc.wantMethods[key]
			if eq := cmp.Equal(gotMethod, wantMethod, cmp.Comparer(proto.Equal)); !eq {
				t.Errorf("Test(%v) failed: \n got Method: %v,\nwant Method: %v", tc.desc, gotMethod, wantMethod)
			}
		}
		for idx, gotApiName := range serviceInfo.ApiNames {
			wantApiName := tc.wantApiNames[idx]
			if gotApiName != wantApiName {
				t.Errorf("Test(%v) failed: \n got ApiName: %v,\nwant Apiname: %v", tc.desc, gotApiName, wantApiName)
			}
		}
	}
}

func TestProcessApisForGrpc(t *testing.T) {
	testData := []struct {
		desc              string
		fakeServiceConfig *confpb.Service
		wantMethods       map[string]*methodInfo
		wantApiNames      []string
	}{
		{
			desc: "Process API with unary and streaming gRPC methods",
			fakeServiceConfig: &confpb.Service{
				Apis: []*apipb.Api{
					{
						Name: "api-streaming-test",
						Methods: []*apipb.Method{
							{
								Name: "unary",
							},
							{
								Name:             "streaming_request",
								RequestStreaming: true,
							},
							{
								Name:              "streaming_response",
								ResponseStreaming: true,
							},
						},
					},
				},
			},
			wantMethods: map[string]*methodInfo{
				"api-streaming-test.unary": {
					ShortName: "unary",
					ApiName:   "api-streaming-test",
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/api-streaming-test/unary",
							HttpMethod:  util.POST,
						},
					},
				},
				"api-streaming-test.streaming_request": {
					ShortName:   "streaming_request",
					ApiName:     "api-streaming-test",
					IsStreaming: true,
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/api-streaming-test/streaming_request",
							HttpMethod:  util.POST,
						},
					},
				},
				"api-streaming-test.streaming_response": {
					ShortName:   "streaming_response",
					ApiName:     "api-streaming-test",
					IsStreaming: true,
					HttpRule: []*commonpb.Pattern{
						{
							UriTemplate: "/api-streaming-test/streaming_response",
							HttpMethod:  util.POST,
						},
					},
				},
			},
			wantApiNames: []string{
				"api-streaming-test",
			},
		},
	}

	for i, tc := range testData {

		serviceInfo := &ServiceInfo{
			serviceConfig:       tc.fakeServiceConfig,
			GrpcSupportRequired: true,
			Methods:             make(map[string]*methodInfo),
		}
		serviceInfo.processApis()
		serviceInfo.addGrpcHttpRules()

		for key, gotMethod := range serviceInfo.Methods {
			wantMethod := tc.wantMethods[key]
			if eq := cmp.Equal(gotMethod, wantMethod, cmp.Comparer(proto.Equal)); !eq {
				t.Errorf("Test Desc(%d): %s,\ngot Method: %v,\nwant Method: %v", i, tc.desc, gotMethod, wantMethod)
			}
		}
		for idx, gotApiName := range serviceInfo.ApiNames {
			wantApiName := tc.wantApiNames[idx]
			if gotApiName != wantApiName {
				t.Errorf("Test Desc(%d): %s,\ngot ApiName: %v,\nwant Apiname: %v", i, tc.desc, gotApiName, wantApiName)
			}
		}
	}
}

func TestProcessTypes(t *testing.T) {
	testData := []struct {
		desc                            string
		fakeServiceConfig               *confpb.Service
		fakeRequestTypeNamesByOperation map[string]string
		wantSegmentsByOperation         map[string]SnakeToJsonSegments
		wantErr                         error
	}{
		{
			desc: "Success for single type with multiple distinct fields",
			fakeServiceConfig: &confpb.Service{
				Types: []*ptypepb.Type{
					{
						Name: "CreateShelvesRequest",
						Fields: []*ptypepb.Field{
							{
								Name:     "foo_bar",
								JsonName: "fooBar",
							},
							{
								Name:     "x_y",
								JsonName: "xY",
							},
						},
					},
				},
			},
			fakeRequestTypeNamesByOperation: map[string]string{
				"api-1.operation-1": "CreateShelvesRequest",
			},
			wantSegmentsByOperation: map[string]SnakeToJsonSegments{
				"api-1.operation-1": {
					"foo_bar": "fooBar",
					"x_y":     "xY",
				},
			},
		},
		{
			desc: "Success for single type with multiple duplicated fields, which can be de-duped",
			fakeServiceConfig: &confpb.Service{
				Types: []*ptypepb.Type{
					{
						Name: "CreateShelvesRequest",
						Fields: []*ptypepb.Field{
							{
								Name:     "foo_bar",
								JsonName: "fooBar",
							},
							{
								Name:     "foo_bar",
								JsonName: "fooBar",
							},
						},
					},
				},
			},
			fakeRequestTypeNamesByOperation: map[string]string{
				"api-1.operation-1": "CreateShelvesRequest",
			},
			wantSegmentsByOperation: map[string]SnakeToJsonSegments{
				"api-1.operation-1": {
					"foo_bar": "fooBar",
				},
			},
		},
		{
			desc: "Success for single type with multiple non-conflicting fields",
			fakeServiceConfig: &confpb.Service{
				Types: []*ptypepb.Type{
					{
						Name: "CreateShelvesRequest",
						Fields: []*ptypepb.Field{
							{
								// This one is ignored, its doesn't map anything.
								Name:     "fooBar",
								JsonName: "fooBar",
							},
							{
								Name:     "foo_bar",
								JsonName: "fooBar",
							},
						},
					},
				},
			},
			fakeRequestTypeNamesByOperation: map[string]string{
				"api-1.operation-1": "CreateShelvesRequest",
			},
			wantSegmentsByOperation: map[string]SnakeToJsonSegments{
				"api-1.operation-1": {
					"foo_bar": "fooBar",
				},
			},
		},
		{
			desc: "Failure for single type with multiple conflicting fields",
			fakeServiceConfig: &confpb.Service{
				Types: []*ptypepb.Type{
					{
						Name: "CreateShelvesRequest",
						Fields: []*ptypepb.Field{
							{
								Name:     "foo_bar",
								JsonName: "foo-bar",
							},
							{
								Name:     "foo_bar",
								JsonName: "fooBar",
							},
						},
					},
				},
			},
			fakeRequestTypeNamesByOperation: map[string]string{
				"api-1.operation-1": "CreateShelvesRequest",
			},
			wantErr: fmt.Errorf("for operation (api-1.operation-1): detected two types with same snake_name (foo_bar) but mistmatching json_name"),
		},
		{
			desc: "Success for multiple types with distinct fields",
			fakeServiceConfig: &confpb.Service{
				Types: []*ptypepb.Type{
					{
						Name: "CreateShelvesRequest",
						Fields: []*ptypepb.Field{
							{
								Name:     "foo_bar",
								JsonName: "fooBar",
							},
							{
								Name:     "x_y",
								JsonName: "xY",
							},
						},
					},
					{
						Name: "CreateBookRequest",
						Fields: []*ptypepb.Field{
							{
								// This one will be ignored, the names match.
								Name:     "baz",
								JsonName: "baz",
							},
							{
								Name:     "a_b",
								JsonName: "aB",
							},
						},
					},
					{
						Name: "google.protobuf.Empty",
						// This will be ignored, no fields at all.
						Fields: []*ptypepb.Field{},
					},
					{
						// This will be ignored, it's not directly any operation's request type.
						Name: "Library",
						Fields: []*ptypepb.Field{
							{
								Name:     "lib_name",
								JsonName: "libName",
							},
						},
					},
				},
			},
			fakeRequestTypeNamesByOperation: map[string]string{
				"api-1.operation-1": "CreateShelvesRequest",
				"api-1.operation-2": "CreateBookRequest",
				"api-2.operation-1": "google.protobuf.Empty",
				// This will be ignored, it doesn't exist in types.
				"api-3.operation-1": "NonExistingType",
				// This will be ignored, it is empty.
				"api-3.operation-2": "",
			},
			wantSegmentsByOperation: map[string]SnakeToJsonSegments{
				"api-1.operation-1": {
					"foo_bar": "fooBar",
					"x_y":     "xY",
				},
				"api-1.operation-2": {
					"a_b": "aB",
				},
			},
		},
		{
			desc: "Success for multiple types with conflicting fields across types, but no conflicts within a single type",
			fakeServiceConfig: &confpb.Service{
				Types: []*ptypepb.Type{
					{
						Name: "CreateShelvesRequest",
						Fields: []*ptypepb.Field{
							{
								Name:     "foo_bar",
								JsonName: "foo-bar",
							},
						},
					},
					{
						Name: "CreateBookRequest",
						Fields: []*ptypepb.Field{
							{
								Name:     "foo_bar",
								JsonName: "fooBar",
							},
						},
					},
				},
			},
			fakeRequestTypeNamesByOperation: map[string]string{
				"api-1.operation-1": "CreateShelvesRequest",
				"api-1.operation-2": "CreateBookRequest",
			},
			wantSegmentsByOperation: map[string]SnakeToJsonSegments{
				"api-1.operation-1": {
					"foo_bar": "foo-bar",
				},
				"api-1.operation-2": {
					"foo_bar": "fooBar",
				},
			},
		},
	}

	for _, tc := range testData {
		serviceInfo := &ServiceInfo{
			serviceConfig: tc.fakeServiceConfig,
			Methods:       make(map[string]*methodInfo),
		}
		// Emulate a part of processApis().
		for operation, requestType := range tc.fakeRequestTypeNamesByOperation {
			mi := &methodInfo{
				RequestTypeName: requestType,
			}
			serviceInfo.Methods[operation] = mi
		}

		err := serviceInfo.processTypes()

		if err != nil {
			if tc.wantErr == nil || !strings.Contains(err.Error(), tc.wantErr.Error()) {
				t.Errorf("Test(%v): Expected err (%v), got err (%v)", tc.desc, tc.wantErr, err)
			}
			continue
		}

		if tc.wantErr != nil {
			t.Errorf("Test(%v): Expected err (%v), got no err", tc.desc, tc.wantErr)
		}

		for operation, wantMapping := range tc.wantSegmentsByOperation {
			gotMapping := serviceInfo.Methods[operation].SegmentMappings
			if !reflect.DeepEqual(wantMapping, gotMapping) {
				t.Errorf("Test(%v): For operation (%v), expected segment mapping (%v), got segments (%v)", tc.desc, operation, wantMapping, gotMapping)
			}
		}
	}
}

func TestProcessAccessToken(t *testing.T) {
	fakeServiceConfig := &confpb.Service{
		Apis: []*apipb.Api{
			{
				Name: testApiName,
			},
		},
	}
	testCases := []struct {
		desc              string
		serviceAccountKey string
		wantAccessToken   *commonpb.AccessToken
	}{
		{
			desc: "get access token from imds",
			wantAccessToken: &commonpb.AccessToken{
				TokenType: &commonpb.AccessToken_RemoteToken{
					RemoteToken: &commonpb.HttpUri{
						Uri:     "http://169.254.169.254/computeMetadata/v1/instance/service-accounts/default/token",
						Cluster: "metadata-cluster",
						Timeout: ptypes.DurationProto(30 * time.Second),
					},
				},
			},
		},
		{
			desc:              "get access token from lmds",
			serviceAccountKey: "this-is-service-account-key",
			wantAccessToken: &commonpb.AccessToken{
				TokenType: &commonpb.AccessToken_RemoteToken{
					RemoteToken: &commonpb.HttpUri{
						Uri:     "http://127.0.0.1:8791/local/access_token",
						Cluster: "token-agent-cluster",
						Timeout: ptypes.DurationProto(30 * time.Second),
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		opts := options.DefaultConfigGeneratorOptions()
		opts.ServiceAccountKey = tc.serviceAccountKey
		serviceInfo, err := NewServiceInfoFromServiceConfig(fakeServiceConfig, "ConfigID", opts)
		if err != nil {
			t.Fatal(err)
		}

		serviceInfo.processAccessToken()
		if !reflect.DeepEqual(serviceInfo.AccessToken, tc.wantAccessToken) {
			t.Errorf("fail(%s): expect accessToken: %v, get accessToken: %v", tc.desc, tc.wantAccessToken, serviceInfo.AccessToken)
		}
	}

}
