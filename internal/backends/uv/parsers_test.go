//go:build linux || darwin
// +build linux darwin

package uv

import (
	"reflect"
	"testing"

	"github.com/Rotlerxd/pipnest/internal/backends"
)

func TestParseListOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []backends.Package
	}{
		{
			name:  "json output",
			input: `[{"name":"requests","version":"2.31.0"},{"name":"httpx","version":"0.27.0"}]`,
			want: []backends.Package{
				{Name: "requests", Version: "2.31.0"},
				{Name: "httpx", Version: "0.27.0"},
			},
		},
		{
			name:  "table fallback output",
			input: "Package    Version\n---------- -------\nrequests   2.31.0\nhttpx      0.27.0\n",
			want: []backends.Package{
				{Name: "requests", Version: "2.31.0"},
				{Name: "httpx", Version: "0.27.0"},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			input := tc.input

			// Act
			got, err := parseListOutput(input)

			// Assert
			if err != nil {
				t.Fatalf("parseListOutput returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unexpected parsed list\nwant: %#v\ngot:  %#v", tc.want, got)
			}
		})
	}
}

func TestParseShowOutput(t *testing.T) {
	t.Parallel()

	// Arrange
	input := `
Name: requests
VERSION: 2.31.0
Summary: HTTP for Humans
Home-Page: https://requests.readthedocs.io
Author: Kenneth Reitz
License: Apache-2.0
Requires: urllib3
`

	want := backends.PackageDetails{
		Name:     "requests",
		Version:  "2.31.0",
		Summary:  "HTTP for Humans",
		HomePage: "https://requests.readthedocs.io",
		Author:   "Kenneth Reitz",
		License:  "Apache-2.0",
		Raw:      "Name: requests\nVERSION: 2.31.0\nSummary: HTTP for Humans\nHome-Page: https://requests.readthedocs.io\nAuthor: Kenneth Reitz\nLicense: Apache-2.0\nRequires: urllib3",
	}

	// Act
	got := parseShowOutput(input)

	// Assert
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected parsed details\nwant: %#v\ngot:  %#v", want, got)
	}
}
