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
package config

import (
	"bytes"
	"io"
	"os"
	"text/template"

	mf "github.com/manifestival/manifestival"
)

// PathPrefix is the file system path which template paths will be prefixed with.
// Default is no prefix, which causes paths to be read relative to process working dir
var PathPrefix string

func prefixedPath(p string) string {
	if PathPrefix != "" {
		return PathPrefix + "/" + p
	}
	return p
}

// A templating source read from a file
func PathTemplateSource(path string, context interface{}) mf.Source {
	f, err := os.Open(prefixedPath(path))
	if err != nil {
		panic(err)
	}
	return templateSource(f, context)
}

// A templating manifest source
func templateSource(r io.Reader, context interface{}) mf.Source {
	b, err := io.ReadAll(r)
	if err != nil {
		panic(err)
	}
	t, err := template.New("foo").Parse(string(b))
	if err != nil {
		panic(err)
	}
	var b2 bytes.Buffer
	err = t.Execute(&b2, context)
	if err != nil {
		panic(err)
	}
	return mf.Reader(&b2)
}
