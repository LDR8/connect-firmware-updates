package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Channels a printer can follow.
const (
	ChannelStable = "stable"
	ChannelBeta   = "beta"
)

// minRefreshGap keeps a person pressing "check now" from spending GitHub's
// rate limit of 60 requests an hour.
const minRefreshGap = 30 * time.Second

// Offer is the newest firmware for one model on one channel.
type Offer struct {
	Version    string    `json:"version"`
	Tag        string    `json:"tag"`
	Name       string    `json:"name"`
	Prerelease bool      `json:"prerelease"`
	Published  time.Time `json:"published"`
	Page       string    `json:"page"`
	Notes      string    `json:"notes"`
	Asset      string    `json:"asset"`
	Size       int64     `json:"size"`
}

// Catalog holds the releases read from GitHub and the channel of each
// printer. It is shared by the poll loop and the routes.
type Catalog struct {
	mu       sync.Mutex
	dir      string
	releases []Release
	checked  time.Time
	lastErr  string
	channels map[string]string
	fetching bool
}

type catalogFile struct {
	Releases []Release `json:"releases"`
	Checked  time.Time `json:"checked"`
}

// NewCatalog reads what an earlier run kept in dir, which may be empty.
func NewCatalog(dir string) *Catalog {
	c := &Catalog{dir: dir, channels: map[string]string{}}
	if dir == "" {
		return c
	}

	if raw, err := os.ReadFile(filepath.Join(dir, "channels.json")); err == nil {
		_ = json.Unmarshal(raw, &c.channels)
	}

	if raw, err := os.ReadFile(filepath.Join(dir, "catalog.json")); err == nil {
		var held catalogFile
		if json.Unmarshal(raw, &held) == nil {
			c.setReleases(held.Releases)
			c.checked = held.Checked
		}
	}

	return c
}

func (c *Catalog) setReleases(list []Release) {
	for i := range list {
		list[i].parsed, _ = ParseVersion(list[i].Tag)
	}

	sort.SliceStable(list, func(i, j int) bool { return list[i].parsed.Compare(list[j].parsed) > 0 })
	c.releases = list
}

// Refresh reads the releases from GitHub. force skips the wait between two
// reads only when the last one is older than minRefreshGap.
func (c *Catalog) Refresh(ctx context.Context, token string) error {
	c.mu.Lock()
	if c.fetching || time.Since(c.checked) < minRefreshGap {
		c.mu.Unlock()

		return nil
	}

	c.fetching = true
	c.mu.Unlock()

	list, err := fetchReleases(ctx, token)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.fetching = false

	if err != nil {
		c.lastErr = err.Error()

		return err
	}

	c.lastErr = ""
	c.checked = time.Now().UTC()
	c.setReleases(list)
	c.save("catalog.json", catalogFile{Releases: c.releases, Checked: c.checked})

	return nil
}

// Checked answers when GitHub was last read, and the last error.
func (c *Catalog) Checked() (time.Time, string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.checked, c.lastErr
}

// Offers answers the newest stable and the newest beta firmware for a model.
// beta is the newest of all releases, so it equals stable when no pre-release
// is newer.
func (c *Catalog) Offers(model, language string) (stable, beta *Offer) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, rel := range c.releases {
		asset, ok := fitting(rel, model, language)
		if !ok {
			continue
		}

		offer := offerOf(rel, asset)
		if beta == nil {
			beta = offer
		}

		if !rel.Prerelease {
			stable = offer

			break
		}
	}

	return stable, beta
}

// Find answers the release with this tag and its image for the model.
func (c *Catalog) Find(tag, model, language string) (Release, Asset, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, rel := range c.releases {
		if rel.Tag != tag {
			continue
		}

		asset, ok := fitting(rel, model, language)

		return rel, asset, ok
	}

	return Release{}, Asset{}, false
}

func fitting(rel Release, model, language string) (Asset, bool) {
	for _, a := range rel.Assets {
		if assetFits(a.Name, model, language) {
			return a, true
		}
	}

	return Asset{}, false
}

func offerOf(rel Release, a Asset) *Offer {
	return &Offer{
		Version: rel.Version, Tag: rel.Tag, Name: rel.Name, Prerelease: rel.Prerelease,
		Published: rel.Published, Page: rel.Page, Notes: rel.Notes, Asset: a.Name, Size: a.Size,
	}
}

// Channels answers a copy of the per-printer channels.
func (c *Catalog) Channels() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make(map[string]string, len(c.channels))
	for k, v := range c.channels {
		out[k] = v
	}

	return out
}

// SetChannel sets the channel of one printer. An empty channel goes back to
// the default.
func (c *Catalog) SetChannel(device, channel string) error {
	switch channel {
	case "", ChannelStable, ChannelBeta:
	default:
		return fmt.Errorf("channel is %q, expected stable, beta or empty", channel)
	}

	if device == "" || len(device) > 200 || strings.ContainsAny(device, "/\\") {
		return errors.New("the device id is not one")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if channel == "" {
		delete(c.channels, device)
	} else {
		c.channels[device] = channel
	}

	return c.save("channels.json", c.channels)
}

// save writes one file into the data directory; c.mu is held.
func (c *Catalog) save(file string, value any) error {
	if c.dir == "" {
		return nil
	}

	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	tmp := filepath.Join(c.dir, file+".tmp")
	if err = os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}

	return os.Rename(tmp, filepath.Join(c.dir, file))
}

// printerFileName is the 8.3 name the image gets on the printer's USB drive.
// M997 takes a short name, and refuses an S followed by a digit, so the name
// is FW and the version's digits: FW802.BBF, FW803B.BBF for a beta.
func printerFileName(version string) string {
	v, ok := ParseVersion(version)
	if !ok {
		return "FWUPDATE.BBF"
	}

	digits := fmt.Sprintf("%d%d%d", v.Major, v.Minor, v.Patch)

	suffix := ""

	switch v.Stage {
	case stageAlpha:
		suffix = "A"
	case stageBeta:
		suffix = "B"
	case stageRC:
		suffix = "R"
	}

	name := "FW" + digits + suffix
	if len(name) > 8 {
		name = name[:8]
	}

	return name + ".BBF"
}
