package cmd

import "testing"

func TestValidateImportFlags(t *testing.T) {
	tests := []struct {
		name        string
		tag         string
		forceTag    bool
		expectError bool
	}{
		{name: "no tag, no force-tag", tag: "", forceTag: false, expectError: false},
		{name: "tag without force-tag", tag: "test:latest", forceTag: false, expectError: false},
		{name: "tag with force-tag", tag: "test:latest", forceTag: true, expectError: false},
		{name: "force-tag without tag", tag: "", forceTag: true, expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImportFlags(tt.tag, tt.forceTag)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}
