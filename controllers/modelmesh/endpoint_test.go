// Copyright 2021 IBM Corporation
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

package modelmesh

import (
	"reflect"
	"testing"
)

func TestParseEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		expected interface{}
	}{
		{
			"port-number",
			"port:1234",
			TCPEndpoint{
				Port: "1234",
			},
		},
		{
			"unix-uri",
			"unix:/var/file.sock",
			UnixEndpoint{
				Path:       "/var/file.sock",
				ParentPath: "/var",
			},
		},
		{
			"unix-url",
			"unix:///var/file.sock",
			UnixEndpoint{
				Path:       "/var/file.sock",
				ParentPath: "/var",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint, err := ParseEndpoint(tt.endpoint)
			if err != nil {
				t.Fatal("Unexpected error ", err)
			}
			if !reflect.DeepEqual(tt.expected, endpoint) {
				t.Fatalf("Expected %v but found %v", tt.expected, endpoint)
			}
		})
	}
}
