package main

import (
	"errors"
	"fmt"
	"strings"
)

// The header of a Buddy firmware file (.bbf), as Prusa's utils/unpack_bbf.py
// reads it: a 64-byte signature, a 32-byte digest, then
//
//	firmware length   4 bytes, little endian
//	version           major, minor, patch: 1 byte each
//	build number      2 bytes
//	prerelease        5 bytes, text padded with zeros
//	board revision    1 byte
//	printer type      1 byte
//	then for bbf version 1:   printer version (=1), printer subversion
//	     for bbf version >=2: bbf version, printer subversion, printer version
const bbfHeaderLen = 64 + 32 + 4 + 3 + 2 + 5 + 1 + 1 + 3

// BBFHeader is what a firmware file says about itself.
type BBFHeader struct {
	Major, Minor, Patch int
	Prerelease          string
	Build               int
	PrinterType         int
	PrinterVersion      int
	PrinterSubversion   int
}

// Code is the printer the file is built for, as type.version.subversion, the
// spelling of the device type codes in Prusa Connect's device models.
func (h BBFHeader) Code() string {
	return fmt.Sprintf("%d.%d.%d", h.PrinterType, h.PrinterVersion, h.PrinterSubversion)
}

// Version is the firmware version the file carries, without the build.
func (h BBFHeader) Version() string {
	v := fmt.Sprintf("%d.%d.%d", h.Major, h.Minor, h.Patch)
	if h.Prerelease != "" {
		v += "-" + h.Prerelease
	}

	return v
}

func parseBBF(b []byte) (BBFHeader, error) {
	if len(b) < bbfHeaderLen {
		return BBFHeader{}, errors.New("the file is too short to be a Buddy firmware file")
	}

	o := 64 + 32
	length := int(b[o]) | int(b[o+1])<<8 | int(b[o+2])<<16 | int(b[o+3])<<24
	o += 4

	h := BBFHeader{Major: int(b[o]), Minor: int(b[o+1]), Patch: int(b[o+2])}
	o += 3
	h.Build = int(b[o]) | int(b[o+1])<<8
	o += 2
	h.Prerelease = strings.TrimRight(string(b[o:o+5]), "\x00")
	o += 5
	o++ // board revision
	h.PrinterType = int(b[o])
	o++

	if b[o] == 1 {
		h.PrinterVersion, h.PrinterSubversion = 1, int(b[o+1])
	} else {
		h.PrinterSubversion, h.PrinterVersion = int(b[o+1]), int(b[o+2])
	}

	if length <= 0 || length > len(b) {
		return BBFHeader{}, errors.New("the firmware length in the header does not fit the file")
	}

	return h, nil
}

// headerCodes is, per model as the device record spells it, the printer codes
// a firmware file for it may carry. A file serving several models is built
// for one of them: the MK4/MK4S/MK3.9/MK3.9S file says 1.4.0, the MK3.5/MK3.5S
// file 1.3.5. Each model's own code is accepted too, in case Prusa splits a
// file. A model not listed here is offered no installation at all.
//
//nolint:gochecknoglobals // a fixed table.
var headerCodes = map[string][]string{
	"MK4":           {"1.4.0"},
	"MK4S":          {"1.4.0", "1.4.1"},
	"MK3.9":         {"1.4.0", "1.3.9"},
	"MK3.9S":        {"1.4.0", "1.3.10"},
	"MK3.5":         {"1.3.5"},
	"MK3.5S":        {"1.3.5", "1.3.6"},
	"MINI":          {"2.1.0"},
	"XL":            {"3.1.0"},
	"IX":            {"4.1.0"},
	"CoreOne":       {"7.1.0"},
	"CoreOne_INDX":  {"7.10.0"},
	"CoreOneL":      {"8.1.0"},
	"CoreOneL_INDX": {"8.10.0"},
}

// checkImage refuses a firmware file whose header names another printer or
// another version than the release it was downloaded for.
func checkImage(image []byte, model, version string) (BBFHeader, error) {
	h, err := parseBBF(image)
	if err != nil {
		return h, err
	}

	allowed, known := headerCodes[model]
	if !known {
		return h, fmt.Errorf("no printer code is known for the model %s, so its firmware is not checked and not offered", model)
	}

	fits := false

	for _, code := range allowed {
		if code == h.Code() {
			fits = true
		}
	}

	if !fits {
		return h, fmt.Errorf("the file is built for printer %s, and a %s takes %s", h.Code(), model, strings.Join(allowed, " or "))
	}

	want, ok := ParseVersion(version)
	if !ok || want.Major != h.Major || want.Minor != h.Minor || want.Patch != h.Patch {
		return h, fmt.Errorf("the file carries firmware %s, not the release's %s", h.Version(), version)
	}

	return h, nil
}
