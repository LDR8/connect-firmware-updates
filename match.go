package main

import (
	"regexp"
	"strings"
)

// The part of an asset name before "_firmware" or "-firmware" lists the
// printers it is for, separated by underscores:
//
//	MK4_MK4S_MK3.9_MK3.9S_firmware_8.0.2.bbf
//	COREONE_COREONE+GEN2_INDX_firmware_8.0.2.bbf
//	COREONE_L_COREONE_L+_firmware_7.0.0.bbf
//	COREONEL_COREONEL+_firmware_8.0.2.bbf
//	MINI_english-german_firmware_8.0.2.bbf
//
// Some releases had no "firmware" in the name, and the list ends where the
// version starts:
//
//	COREONE_COREONE+_INDX_6.6.3.bbf
var firmwareMarker = regexp.MustCompile(`(?i)[_-]firmware[_-]|[_-]v?\d+\.\d+\.\d+`)

// assetTokens answers the printer tokens of a firmware asset name, upper case,
// and the MINI language when the name carries one. ok is false for a file that
// is not a .bbf firmware image.
func assetTokens(asset string) (tokens []string, language string, ok bool) {
	if !strings.HasSuffix(strings.ToLower(asset), ".bbf") {
		return nil, "", false
	}

	loc := firmwareMarker.FindStringIndex(asset)
	if loc == nil {
		return nil, "", false
	}

	prefix := strings.ToUpper(asset[:loc[0]])
	// The CORE One L was spelled COREONE_L in some releases and COREONEL in
	// others.
	prefix = strings.ReplaceAll(prefix, "COREONE_L", "COREONEL")

	for _, token := range strings.Split(prefix, "_") {
		if token == "" {
			continue
		}

		if lang, found := strings.CutPrefix(token, "ENGLISH-"); found {
			language = strings.ToLower(lang)

			continue
		}

		tokens = append(tokens, token)
	}

	return tokens, language, len(tokens) > 0
}

// modelToken is the token a printer model is listed under, and whether the
// model is a tool changer (INDX), whose firmware is a separate image.
func modelToken(model string) (token string, indx bool) {
	upper := strings.ToUpper(strings.TrimSpace(model))
	if base, found := strings.CutSuffix(upper, "_INDX"); found {
		return base, true
	}

	return upper, false
}

// assetFits reports whether the asset is the firmware image for this model.
// language picks one of the MINI's language images.
func assetFits(asset, model, language string) bool {
	tokens, lang, ok := assetTokens(asset)
	if !ok {
		return false
	}

	want, indx := modelToken(model)
	if want == "" {
		return false
	}

	named, hasIndx := false, false

	for _, token := range tokens {
		switch token {
		case want:
			named = true
		case "INDX":
			hasIndx = true
		}
	}

	if !named || hasIndx != indx {
		return false
	}

	if lang != "" && !strings.EqualFold(lang, language) {
		return false
	}

	return true
}
