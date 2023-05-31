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
	//"fmt"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
)

func ParseEndpoint(s string) (Endpoint, error) {
	if strings.HasPrefix(s, "port:") {
		portSpl := strings.SplitAfter(s, "port:")
		numberStr := portSpl[1]
		return TCPEndpoint{Port: numberStr}, nil
	}
	fspath := strings.Replace(s, "unix://", "", 1)
	fspath = strings.Replace(fspath, "unix:", "", 1)
	fsdir := filepath.Dir(fspath)
	return UnixEndpoint{Path: fspath, ParentPath: fsdir}, nil
}

func ValidateEndpoint(s string) (string, error) {
	match, _ := regexp.MatchString("^port:[0-9]+$", s)
	if match || strings.HasPrefix(s, "unix:") {
		return s, nil
	} else {
		return "", errors.New("Invaid Endpoint: " + s)
	}
}

type Endpoint interface {
}

type TCPEndpoint struct {
	Endpoint
	Port string
}

type UnixEndpoint struct {
	Endpoint

	// The file system path which may be a file
	Path string

	// The file system directory which contains the
	// file.
	ParentPath string
}
