package main

import (
	"regexp"
	"strconv"
	"strings"
)

// Version is a Buddy firmware version: 6.4.0, 6.5.0-RC1, 8.0.3-beta,
// 6.2.0-alpha2. Build metadata after a "+" is ignored.
type Version struct {
	Major, Minor, Patch int
	// Stage orders the pre-releases: alpha < beta < rc < final.
	Stage int
	// StageNum is the number after the stage, such as the 2 of RC2.
	StageNum int
	// Raw is the text it was read from, without a leading v.
	Raw string
}

const (
	stageAlpha = iota
	stageBeta
	stageRC
	stageFinal
)

var versionPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)(?:\.(\d+))?(?:[-_.]?([A-Za-z]+)[-_.]?(\d*))?`)

// ParseVersion reads a version, and answers false for text that is not one.
func ParseVersion(text string) (Version, bool) {
	text = strings.TrimSpace(text)
	if base, _, found := strings.Cut(text, "+"); found {
		text = base
	}

	m := versionPattern.FindStringSubmatch(text)
	if m == nil {
		return Version{}, false
	}

	v := Version{Stage: stageFinal, Raw: strings.TrimPrefix(text, "v")}
	v.Major, _ = strconv.Atoi(m[1])
	v.Minor, _ = strconv.Atoi(m[2])
	v.Patch, _ = strconv.Atoi(m[3])

	switch strings.ToLower(m[4]) {
	case "":
		v.Stage = stageFinal
	case "alpha", "a":
		v.Stage = stageAlpha
	case "beta", "b":
		v.Stage = stageBeta
	case "rc":
		v.Stage = stageRC
	default:
		// An unknown suffix is a pre-release of some kind, below a final.
		v.Stage = stageAlpha
	}

	v.StageNum, _ = strconv.Atoi(m[5])

	return v, true
}

// Prerelease is true for an alpha, a beta or a release candidate.
func (v Version) Prerelease() bool { return v.Stage != stageFinal }

// Compare answers -1, 0 or 1.
func (v Version) Compare(o Version) int {
	for _, pair := range [][2]int{
		{v.Major, o.Major}, {v.Minor, o.Minor}, {v.Patch, o.Patch},
		{v.Stage, o.Stage}, {v.StageNum, o.StageNum},
	} {
		switch {
		case pair[0] < pair[1]:
			return -1
		case pair[0] > pair[1]:
			return 1
		}
	}

	return 0
}
