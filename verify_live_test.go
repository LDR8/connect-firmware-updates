package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

var verifyModels = []string{"CoreOne", "CoreOne_INDX", "CoreOneL", "CoreOneL_INDX", "IX", "MINI", "MK3.5", "MK3.5S", "MK3.9", "MK3.9S", "MK4", "MK4S", "XL"}

func TestVerifyLive(t *testing.T) {
	if os.Getenv("VERIFY") == "" {
		t.Skip()
	}
	list, err := fetchReleases(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	c := NewCatalog("")
	c.setReleases(list)

	type row struct {
		Release, Model string
		Matches        []string
	}
	var rows []row
	var unmatched []string
	for _, rel := range c.releases {
		used := map[string]bool{}
		for _, m := range verifyModels {
			var hits []string
			for _, a := range rel.Assets {
				if assetFits(a.Name, m, "german") {
					hits = append(hits, a.Name)
					used[a.Name] = true
				}
			}
			rows = append(rows, row{rel.Tag, m, hits})
			if len(hits) > 1 {
				t.Errorf("%s %s: %d assets fit: %v", rel.Tag, m, len(hits), hits)
			}
		}
		for _, a := range rel.Assets {
			if !used[a.Name] {
				unmatched = append(unmatched, rel.Tag+" "+a.Name)
			}
		}
	}
	offers := map[string]map[string]*Offer{}
	urls := map[string]string{}
	for _, m := range verifyModels {
		s, b := c.Offers(m, "german")
		offers[m] = map[string]*Offer{"stable": s, "beta": b}
		for _, o := range []*Offer{s, b} {
			if o != nil {
				_, a, _ := c.Find(o.Tag, m, "german")
				urls[a.Name] = a.URL
			}
		}
	}
	out, _ := json.MarshalIndent(map[string]any{"releases": len(c.releases), "rows": rows, "unmatched": unmatched, "offers": offers, "urls": urls}, "", " ")
	_ = os.WriteFile(os.Getenv("VERIFY"), out, 0o644)
}
