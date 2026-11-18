package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// releasesURL lists the Buddy firmware releases, newest first.
const releasesURL = "https://api.github.com/repos/prusa3d/Prusa-Firmware-Buddy/releases?per_page=60"

// maxNotes is how much of a release's notes the catalogue keeps.
const maxNotes = 20000

// maxFirmwareBytes refuses an asset larger than any Buddy image could be.
const maxFirmwareBytes = 32 << 20

// Asset is one firmware image of a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
	// Digest is what GitHub publishes for the file, "sha256:<hex>".
	Digest string `json:"digest,omitempty"`
}

// Release is one GitHub release of the Buddy firmware.
type Release struct {
	Tag        string    `json:"tag"`
	Version    string    `json:"version"`
	Name       string    `json:"name"`
	Prerelease bool      `json:"prerelease"`
	Published  time.Time `json:"published"`
	Page       string    `json:"page"`
	Notes      string    `json:"notes"`
	Assets     []Asset   `json:"assets"`

	parsed Version
}

type githubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
		Digest             string `json:"digest"`
	} `json:"assets"`
}

// httpClient is the one client this module reaches GitHub with.
//
//nolint:gochecknoglobals // shared, safe for concurrent use.
var httpClient = &http.Client{Timeout: 2 * time.Minute}

const userAgent = "prusa-connect-firmware-updates/" + string(release)

// fetchReleases reads the release list from GitHub.
func fetchReleases(ctx context.Context, token string) ([]Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub is unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))

		return nil, fmt.Errorf("GitHub answered %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var raw []githubRelease
	if err = json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&raw); err != nil {
		return nil, fmt.Errorf("read the GitHub answer: %w", err)
	}

	out := make([]Release, 0, len(raw))

	for _, r := range raw {
		if r.Draft {
			continue
		}

		v, ok := ParseVersion(r.TagName)
		if !ok {
			continue
		}

		rel := Release{
			Tag:        r.TagName,
			Version:    v.Raw,
			Name:       r.Name,
			Prerelease: r.Prerelease || v.Prerelease(),
			Published:  r.PublishedAt,
			Page:       r.HTMLURL,
			Notes:      truncate(r.Body, maxNotes),
			parsed:     v,
		}

		for _, a := range r.Assets {
			if _, _, isFirmware := assetTokens(a.Name); isFirmware {
				rel.Assets = append(rel.Assets, Asset{Name: a.Name, URL: a.BrowserDownloadURL, Size: a.Size, Digest: a.Digest})
			}
		}

		if len(rel.Assets) > 0 {
			out = append(out, rel)
		}
	}

	return out, nil
}

// downloadAsset fetches one firmware image into memory and holds it to the
// size and the SHA-256 GitHub published for it.
func downloadAsset(ctx context.Context, asset Asset) ([]byte, error) {
	url := asset.URL
	if !strings.HasPrefix(url, "https://github.com/prusa3d/Prusa-Firmware-Buddy/releases/download/") {
		return nil, errors.New("refusing to download from " + url)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub is unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub answered %s for %s", resp.Status, url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFirmwareBytes+1))
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}

	if len(body) > maxFirmwareBytes {
		return nil, errors.New("the firmware image is larger than any Buddy image can be")
	}

	if asset.Size > 0 && int64(len(body)) != asset.Size {
		return nil, fmt.Errorf("the download has %d bytes, GitHub lists %d", len(body), asset.Size)
	}

	if want, ok := strings.CutPrefix(asset.Digest, "sha256:"); ok {
		sum := sha256.Sum256(body)
		if got := hex.EncodeToString(sum[:]); !strings.EqualFold(got, want) {
			return nil, fmt.Errorf("the download's SHA-256 is %s, GitHub lists %s", got, want)
		}
	}

	return body, nil
}

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}

	return text[:limit] + "\n\n…"
}
