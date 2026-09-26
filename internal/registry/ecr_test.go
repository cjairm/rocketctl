package registry

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestParseTagList(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []string
	}{
		{"tab separated on one line", "1.0.0\t1.1.0\t1.2.0\n", []string{"1.0.0", "1.1.0", "1.2.0"}},
		{
			"split across lines by pagination",
			"1.0.0\t1.1.0\n1.2.0\n",
			[]string{"1.0.0", "1.1.0", "1.2.0"},
		},
		{"empty repository prints None", "None\n", nil},
		{"no output", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseTagList(tt.out); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTagList(%q) = %v, want %v", tt.out, got, tt.want)
			}
		})
	}
}

func TestImageIDBatches(t *testing.T) {
	var tags []string
	for i := range 101 {
		tags = append(tags, fmt.Sprintf("0.0.%d", i))
	}
	batches := imageIDBatches(tags)
	if len(batches) != 2 || len(batches[0]) != 100 || len(batches[1]) != 1 {
		t.Fatalf("imageIDBatches(101 tags) sizes = %d batches, want 100 + 1", len(batches))
	}
	if batches[0][0] != "imageTag=0.0.0" || batches[1][0] != "imageTag=0.0.100" {
		t.Errorf("imageIDBatches() = %q ... %q, want imageTag=<tag>", batches[0][0], batches[1][0])
	}
	if got := imageIDBatches(nil); got != nil {
		t.Errorf("imageIDBatches(nil) = %v, want nil", got)
	}
}

func TestDeleteFailures(t *testing.T) {
	if err := deleteFailures("myapp_api", "None\n"); err != nil {
		t.Errorf("deleteFailures(None) = %v, want nil", err)
	}
	if err := deleteFailures("myapp_api", ""); err != nil {
		t.Errorf("deleteFailures(\"\") = %v, want nil", err)
	}
	err := deleteFailures(
		"myapp_api",
		"1.0.0\tImageNotFound\n1.1.0\tImageReferencedByManifestList\n",
	)
	if err == nil {
		t.Fatal("deleteFailures(two failures) = nil, want error")
	}
	for _, want := range []string{"myapp_api", "1.0.0 (ImageNotFound)", "1.1.0 (ImageReferencedByManifestList)"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("deleteFailures() error = %q, want it to mention %q", err, want)
		}
	}
}
