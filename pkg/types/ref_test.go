package types

import (
	"reflect"
	"testing"
)

func TestParseResourceRef_Table(t *testing.T) {
	tests := []struct {
		in      string
		want    ResourceRef
		wantErr bool
	}{
		{
			in:   "rune://secret/default/db-credentials",
			want: ResourceRef{Type: ResourceTypeSecret, Namespace: "default", Name: "db-credentials"},
		},
		{
			in:   "rune://configmap/default/app-config/value",
			want: ResourceRef{Type: ResourceTypeConfigmap, Namespace: "default", Name: "app-config", Key: "value"},
		},
		{
			in:   "rune://service/prod/id/svc-123",
			want: ResourceRef{Type: ResourceTypeService, Namespace: "prod", ID: "svc-123"},
		},
		{
			in:   "rune://service/prod/id/svc-123/logs",
			want: ResourceRef{Type: ResourceTypeService, Namespace: "prod", ID: "svc-123", Key: "logs"},
		},
		{
			in:   "secret:db-credentials.default.rune",
			want: ResourceRef{Type: ResourceTypeSecret, Namespace: "default", Name: "db-credentials"},
		},
		{
			in:   "configmap:app.default.rune/value",
			want: ResourceRef{Type: ResourceTypeConfigmap, Namespace: "default", Name: "app", Key: "value"},
		},
		{
			in:   "secret:db-credentials",
			want: ResourceRef{Type: ResourceTypeSecret, Name: "db-credentials"},
		},
		{
			in:   "secret:db-credentials/username",
			want: ResourceRef{Type: ResourceTypeSecret, Name: "db-credentials", Key: "username"},
		},
		// invalids
		{in: "rune:///missing", wantErr: true},
		{in: ":no-type", wantErr: true},
		{in: "secret:", wantErr: true},
		{in: "rune://type/ns/id/", wantErr: true},
	}

	for _, tt := range tests {
		got, err := ParseResourceRef(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("ParseResourceRef(%q) expected error, got none", tt.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseResourceRef(%q) unexpected error: %v", tt.in, err)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("ParseResourceRef(%q)\n got:  %#v\n want: %#v", tt.in, got, tt.want)
		}
	}
}
