package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseAcceptHeader(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		want    map[string]string
	}{
		{
			name:    "with group and version",
			headers: []string{"application/json;as=Table;g=meta.k8s.io;v=v1"},
			want: map[string]string{
				"as":               "Table",
				"g":                "meta.k8s.io",
				"v":                "v1",
				"application/json": "",
			},
		},
		{
			name: "empty",
			want: map[string]string{},
		},
		{
			name: "with multiple header values",
			headers: []string{
				"application/json;as=Table;g=meta.k8s.io;v=v1beta1",
				"g=meta.k8s.io;v=v1",
			},
			want: map[string]string{
				"as":               "Table",
				"g":                "meta.k8s.io",
				"v":                "v1beta1",
				"application/json": "",
			},
		},
		{
			name: "with duplicate values in same header",
			headers: []string{
				"application/json;as=Table;v=v1beta1;g=meta.k8s.io, application/json",
			},
			want: map[string]string{
				"as":               "Table",
				"g":                "meta.k8s.io",
				"v":                "v1beta1",
				"application/json": "",
			},
		},
		{
			name: "just a space",
			headers: []string{
				"   ",
			},
			want: map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAcceptHeader(tt.headers)
			assert.Equalf(t, got, tt.want, "parseAcceptHeader() = %v, want %v", got, tt.want)
		})
	}
}
