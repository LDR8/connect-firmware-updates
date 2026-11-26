package main

import (
	"testing"

	"github.com/prusa3d/prusa-connect/sdk/connectorapi"
)

func TestVersionsOrder(t *testing.T) {
	order := []string{"6.2.0-alpha2", "6.2.0-beta", "6.2.0-RC1", "6.2.0-RC2", "6.2.0", "6.2.1", "6.10.0", "8.0.3-beta", "v8.0.3+123"}
	for i := 1; i < len(order); i++ {
		a, ok1 := ParseVersion(order[i-1])
		b, ok2 := ParseVersion(order[i])
		if !ok1 || !ok2 || a.Compare(b) >= 0 {
			t.Errorf("%s should be below %s", order[i-1], order[i])
		}
	}
}

func TestAssetsFitModels(t *testing.T) {
	cases := []struct {
		asset, model string
		fits         bool
	}{
		{"MK4_MK4S_MK3.9_MK3.9S_firmware_8.0.2.bbf", "MK4S", true},
		{"MK4_MK4S_MK3.9_MK3.9S_firmware_8.0.2.bbf", "MK3.9", true},
		{"MK4_MK4S_MK3.9_MK3.9S_firmware_8.0.2.bbf", "MK3.5", false},
		{"MK3.5_MK3.5S_firmware_8.0.2.bbf", "MK3.5S", true},
		{"COREONE_COREONE+GEN2_firmware_8.0.2.bbf", "CoreOne", true},
		{"COREONE_COREONE+GEN2_firmware_8.0.2.bbf", "CoreOne_INDX", false},
		{"COREONE_COREONE+GEN2_INDX_firmware_8.0.2.bbf", "CoreOne_INDX", true},
		{"COREONE_COREONE+GEN2_INDX_firmware_8.0.2.bbf", "CoreOne", false},
		{"COREONE_COREONE+GEN2_INDX-firmware_7.0.1+16905.bbf", "CoreOne_INDX", true},
		{"COREONE_COREONE+GEN2_INDX_firmware_8.0.2.bbf", "COREONEINDX", true},
		{"COREONE_COREONE+GEN2_firmware_8.0.2.bbf", "COREONEINDX", false},
		{"COREONE_COREONE+GEN2_INDX_firmware_8.0.2.bbf", "Core One INDX", true},
		// Prusa has published no CORE One L INDX firmware yet. Its name is
		// assumed to follow the CORE One INDX's.
		{"COREONEL_COREONEL+_INDX_firmware_8.0.2.bbf", "COREONELINDX", true},
		{"COREONEL_COREONEL+_firmware_8.0.2.bbf", "COREONELINDX", false},
		{"COREONEL_COREONEL+_INDX_firmware_8.0.2.bbf", "COREONEL", false},
		{"COREONE_L_COREONE_L+_firmware_7.0.0.bbf", "CoreOneL", true},
		{"COREONEL_COREONEL+_firmware_8.0.2.bbf", "CoreOneL", true},
		{"COREONEL_COREONEL+_firmware_8.0.2.bbf", "CoreOne", false},
		{"XL_XL+_firmware_8.0.2.bbf", "XL", true},
		{"MINI_english-german_firmware_8.0.2.bbf", "MINI", true},
		{"MINI_english-czech_firmware_8.0.2.bbf", "MINI", false},
		{"PSA-2026-002.md", "MK4S", false},
	}
	for _, c := range cases {
		if got := assetFits(c.asset, c.model, "german"); got != c.fits {
			t.Errorf("assetFits(%s, %s) = %v", c.asset, c.model, got)
		}
	}
}

func TestPrinterFileNameIsShort(t *testing.T) {
	for in, want := range map[string]string{"8.0.2": "FW802.BBF", "8.0.3-beta": "FW803B.BBF", "6.5.0-RC1": "FW650R.BBF"} {
		if got := printerFileName(in); got != want {
			t.Errorf("printerFileName(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestOffersPickNewestPerChannel(t *testing.T) {
	c := NewCatalog("")
	c.setReleases([]Release{
		{Tag: "v8.0.2", Assets: []Asset{{Name: "XL_XL+_firmware_8.0.2.bbf"}}},
		{Tag: "v8.0.3-beta", Prerelease: true, Assets: []Asset{{Name: "COREONE_COREONE+GEN2_firmware_8.0.3_beta.bbf"}}},
		{Tag: "v8.1.0-RC1", Prerelease: true, Assets: []Asset{{Name: "XL_XL+_firmware_8.1.0-RC1.bbf"}}},
		{Tag: "v8.0.1", Assets: []Asset{{Name: "XL_XL+_firmware_8.0.1.bbf"}}},
	})
	stable, beta := c.Offers("XL", "german")
	if stable == nil || stable.Tag != "v8.0.2" || beta == nil || beta.Tag != "v8.1.0-RC1" {
		t.Fatalf("stable %+v beta %+v", stable, beta)
	}
	stable, beta = c.Offers("CoreOne", "german")
	if stable != nil || beta == nil || beta.Tag != "v8.0.3-beta" {
		t.Fatalf("CoreOne stable %+v beta %+v", stable, beta)
	}
}

func TestOldNamesWithoutFirmwareWord(t *testing.T) {
	if !assetFits("COREONE_COREONE+_INDX_6.6.3.bbf", "CoreOne_INDX", "german") ||
		assetFits("COREONE_COREONE+_INDX_6.6.3.bbf", "CoreOne", "german") ||
		!assetFits("COREONE_COREONE+_6.5.3-RC.bbf", "CoreOne", "german") {
		t.Fatal("old CORE One names are not matched")
	}
}

// fakeBBF builds a header as Prusa's pack_fw.py writes it (bbf version 2).
func fakeBBF(maj, min, pat, ptype, pver, psub int) []byte {
	b := make([]byte, 600)
	o := 96
	b[o] = 100 // firmware length 100
	o += 4
	b[o], b[o+1], b[o+2] = byte(maj), byte(min), byte(pat)
	o += 3 + 2 + 5 + 1
	b[o] = byte(ptype)
	b[o+1], b[o+2], b[o+3] = 2, byte(psub), byte(pver)
	return b
}

func TestImageCheck(t *testing.T) {
	if _, err := checkImage(fakeBBF(8, 0, 2, 1, 4, 0), "MK4S", "8.0.2"); err != nil {
		t.Errorf("MK4 family file refused for MK4S: %v", err)
	}
	if _, err := checkImage(fakeBBF(8, 0, 2, 1, 3, 5), "MK4S", "8.0.2"); err == nil {
		t.Error("MK3.5 file accepted for MK4S")
	}
	if _, err := checkImage(fakeBBF(8, 0, 2, 7, 1, 0), "COREONE", "8.0.2"); err != nil {
		t.Errorf("CoreOne file refused for a printer reporting COREONE: %v", err)
	}
	if _, err := checkImage(fakeBBF(8, 0, 2, 7, 10, 0), "COREONEINDX", "8.0.2"); err != nil {
		t.Errorf("INDX file refused for a printer reporting COREONEINDX: %v", err)
	}
	if _, err := checkImage(fakeBBF(8, 0, 2, 7, 1, 0), "COREONEINDX", "8.0.2"); err == nil {
		t.Error("CoreOne file accepted for COREONEINDX")
	}
	if _, err := checkImage(fakeBBF(8, 0, 2, 8, 10, 0), "COREONELINDX", "8.0.2"); err != nil {
		t.Errorf("CoreOneL INDX file refused for a printer reporting COREONELINDX: %v", err)
	}
	folded := map[string]string{}
	for name := range headerCodes {
		if other, taken := folded[connectorapi.FoldModel(name)]; taken {
			t.Errorf("%s and %s fold to one model", name, other)
		}
		folded[connectorapi.FoldModel(name)] = name
	}
	if _, err := checkImage(fakeBBF(8, 0, 2, 7, 10, 0), "CoreOne", "8.0.2"); err == nil {
		t.Error("INDX file accepted for CoreOne")
	}
	if _, err := checkImage(fakeBBF(8, 0, 1, 7, 1, 0), "CoreOne", "8.0.2"); err == nil {
		t.Error("wrong version accepted")
	}
	if _, err := checkImage(fakeBBF(8, 0, 2, 7, 1, 0), "Unknown", "8.0.2"); err == nil {
		t.Error("unknown model accepted")
	}
}
