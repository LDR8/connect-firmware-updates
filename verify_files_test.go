package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRealFiles checks the downloaded GitHub files against every model.
func TestRealFiles(t *testing.T) {
	dir := os.Getenv("BBFDIR")
	if dir == "" {
		t.Skip()
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.bbf"))
	for _, f := range files {
		b, _ := os.ReadFile(f)
		h, err := parseBBF(b)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		var accepted []string
		for _, m := range verifyModels {
			if _, err := checkImage(b, m, h.Version()); err == nil {
				accepted = append(accepted, m)
			}
			// the name rule and the header rule must agree
			if assetFits(filepath.Base(f), m, "german") != (func() bool { _, e := checkImage(b, m, h.Version()); return e == nil })() {
				t.Errorf("%s: name and header disagree for %s", filepath.Base(f), m)
			}
		}
		t.Logf("%-50s %s accepted for %v", filepath.Base(f), h.Code(), accepted)
	}
}
