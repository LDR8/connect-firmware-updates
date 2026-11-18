// Command firmware-updates is a Prusa Connect module that watches the Buddy
// firmware releases on GitHub and says, for every printer, whether a newer
// firmware is out on the channel the printer follows: stable, or beta for a
// printer an operator chose to run pre-releases on.
//
// It never installs anything by itself. The browser half offers to copy the
// image onto the printer's USB drive and to start the flash, and both happen
// only when a person presses the button and confirms.
//
// # Routes
//
//	GET  /status?models=MK4S,XL   the offers for those models, the channels, when GitHub was read
//	PUT  /devices/{id}/channel    {"channel": "stable" | "beta" | ""}, "" is the default channel
//	POST /refresh                 read GitHub now
//	POST /stage                   {"model", "tag"}: download the image into a storage of this server
//
// Paths are relative to /api/v1/modules/com.github.aux.ldr8.connect-firmware-updates.
package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"time"

	"github.com/prusa3d/prusa-connect/sdk/extension"
	"github.com/prusa3d/prusa-connect/sdk/fault"
	"github.com/prusa3d/prusa-connect/sdk/routesapi"
	"github.com/prusa3d/prusa-connect/sdk/storageapi"
	"github.com/prusa3d/prusa-connect/sdk/wire"
	"github.com/prusa3d/prusa-connect/sdk/wirerpc"
)

const (
	name    = "firmware-updates"
	release = extension.Version("0.1.0")
)

// Settings is the module's configuration. Every key and its default is in
// module.yaml, and an operator edits them on the module's Settings tab.
type Settings struct {
	DefaultChannel string `json:"default_channel"`
	MiniLanguage   string `json:"mini_language"`
	CheckHours     int    `json:"check_hours"`
	Storage        string `json:"storage"`
	GithubToken    string `json:"github_token"`
}

func defaults() Settings {
	return Settings{DefaultChannel: ChannelStable, MiniLanguage: "german", CheckHours: 6, Storage: "local"}
}

//nolint:gochecknoglobals // set on plugin.configure, read by the routes.
var (
	held    atomic.Pointer[Settings]
	catalog atomic.Pointer[Catalog]
	wake    = make(chan struct{}, 1)
)

func settings() Settings {
	if s := held.Load(); s != nil {
		return *s
	}

	return defaults()
}

func cat() *Catalog {
	if c := catalog.Load(); c != nil {
		return c
	}

	c := NewCatalog("")
	catalog.CompareAndSwap(nil, c)

	return catalog.Load()
}

//go:embed openapi.json
var openAPI []byte

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	line, err := wire.Handshake{
		Protocol: wire.ProtocolMajor,
		MinMinor: wire.MinMinor,
		MaxMinor: wire.MaxMinor,
		Address:  wire.StdioAddress,
	}.Line()
	if err != nil {
		return fmt.Errorf("render the handshake: %w", err)
	}

	if _, err = os.Stdout.WriteString(line); err != nil {
		return fmt.Errorf("write the handshake: %w", err)
	}

	serves := wirerpc.Methods{}

	conn, err := wirerpc.NewConn(stdio{}, wirerpc.Options{Role: wirerpc.RoleChild, Methods: serves})
	if err != nil {
		return fmt.Errorf("build the session: %w", err)
	}

	files := storageapi.NewClient(conn)

	for method, handler := range methods(files) {
		serves[method] = handler
	}

	go poll(ctx)

	fmt.Fprintf(os.Stderr, "%s %s is serving on stdio as pid %d\n", name, release, os.Getpid())

	if err = conn.Serve(ctx); err != nil {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}

func methods(files *storageapi.Client) wirerpc.Methods {
	return wirerpc.Methods{
		wirerpc.MethodInfo: func(context.Context, wirerpc.Call) (any, error) {
			return routesapi.Describe(wirerpc.Info{
				Name: name, Version: release, Contract: wirerpc.Contract,
				Serves: []wirerpc.Method{wirerpc.MethodConfigure},
			}, openAPI)
		},
		wirerpc.MethodConfigure: wirerpc.ConfigureGranted(func(_ context.Context, sent Settings, grants wirerpc.Grants) error {
			sent.DefaultChannel = strings.ToLower(strings.TrimSpace(sent.DefaultChannel))
			if sent.DefaultChannel != ChannelStable && sent.DefaultChannel != ChannelBeta {
				return fmt.Errorf("default_channel is %q, expected stable or beta", sent.DefaultChannel)
			}

			if sent.CheckHours < 1 {
				sent.CheckHours = 1
			}

			if sent.Storage == "" {
				sent.Storage = "local"
			}

			sent.MiniLanguage = strings.ToLower(strings.TrimSpace(sent.MiniLanguage))
			held.Store(&sent)

			if catalog.Load() == nil || grants.Data != "" {
				catalog.Store(NewCatalog(grants.Data))
			}

			select {
			case wake <- struct{}{}:
			default:
			}

			return nil
		}),
		wirerpc.MethodShutdown: func(context.Context, wirerpc.Call) (any, error) {
			return nil, nil
		},
		routesapi.MethodServe: routesapi.Serve(routes(files)),
	}
}

// poll reads GitHub at start and then every check_hours.
func poll(ctx context.Context) {
	// Wait for the settings, which name the data directory.
	select {
	case <-ctx.Done():
		return
	case <-wake:
	case <-time.After(10 * time.Second):
	}

	for {
		fetchCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		if err := cat().Refresh(fetchCtx, settings().GithubToken); err != nil {
			fmt.Fprintf(os.Stderr, "%s: reading the firmware releases failed: %v\n", name, err)
		}

		cancel()

		select {
		case <-ctx.Done():
			return
		case <-wake:
		case <-time.After(time.Duration(settings().CheckHours) * time.Hour):
		}
	}
}

// statusAnswer is GET /status.
type statusAnswer struct {
	DefaultChannel string                       `json:"default_channel"`
	Checked        *time.Time                   `json:"checked,omitempty"`
	Error          string                       `json:"error,omitempty"`
	Channels       map[string]string            `json:"channels"`
	Models         map[string]map[string]*Offer `json:"models"`
	Storage        string                       `json:"storage"`
}

func routes(files *storageapi.Client) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		s := settings()
		c := cat()
		checked, lastErr := c.Checked()

		out := statusAnswer{
			DefaultChannel: s.DefaultChannel,
			Error:          lastErr,
			Channels:       c.Channels(),
			Models:         map[string]map[string]*Offer{},
			Storage:        s.Storage,
		}

		if !checked.IsZero() {
			out.Checked = &checked
		}

		for _, model := range strings.Split(r.URL.Query().Get("models"), ",") {
			model = strings.TrimSpace(model)
			if model == "" || len(out.Models) >= 64 {
				continue
			}

			stable, beta := c.Offers(model, s.MiniLanguage)
			out.Models[model] = map[string]*Offer{ChannelStable: stable, ChannelBeta: beta}
		}

		answer(w, http.StatusOK, out)
	})

	mux.HandleFunc("PUT /devices/{id}/channel", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Channel string `json:"channel"`
		}

		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
			refuse(w, r, BadRequest.New("the body is not JSON: "+err.Error()))

			return
		}

		if err := cat().SetChannel(r.PathValue("id"), body.Channel); err != nil {
			refuse(w, r, BadRequest.New(err.Error()))

			return
		}

		answer(w, http.StatusOK, map[string]any{"device": r.PathValue("id"), "channel": body.Channel})
	})

	mux.HandleFunc("POST /refresh", func(w http.ResponseWriter, r *http.Request) {
		if err := cat().Refresh(r.Context(), settings().GithubToken); err != nil {
			refuse(w, r, GithubFailed.New(err.Error()))

			return
		}

		checked, _ := cat().Checked()
		answer(w, http.StatusOK, map[string]any{"checked": checked})
	})

	mux.HandleFunc("POST /stage", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
			Tag   string `json:"tag"`
		}

		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
			refuse(w, r, BadRequest.New("the body is not JSON: "+err.Error()))

			return
		}

		s := settings()

		rel, asset, ok := cat().Find(body.Tag, body.Model, s.MiniLanguage)
		if !ok {
			refuse(w, r, NoFirmware.New("release "+body.Tag+" holds no firmware for "+body.Model))

			return
		}

		image, err := downloadAsset(r.Context(), asset)
		if err != nil {
			refuse(w, r, GithubFailed.New(err.Error()))

			return
		}

		// The file's own header has the last word: it must name this
		// printer and the release's version, whatever the file is called.
		header, err := checkImage(image, body.Model, rel.Version)
		if err != nil {
			refuse(w, r, WrongFirmware.New("refused "+asset.Name+": "+err.Error()))

			return
		}

		path := "firmware-updates/" + asset.Name

		written, err := files.Write(r.Context(), storageapi.WriteParams{
			Principal: routesapi.PrincipalOf(r),
			Storage:   s.Storage,
			Path:      path,
			Size:      int64(len(image)),
		}, bytes.NewReader(image))
		if err != nil {
			refuse(w, r, StorageFailed.New("write "+path+" into the storage "+s.Storage+": "+err.Error()))

			return
		}

		answer(w, http.StatusOK, map[string]any{
			"storage":      s.Storage,
			"path":         path,
			"size":         written.Size,
			"sha256":       written.SHA256,
			"version":      rel.Version,
			"printer_file": printerFileName(rel.Version),
			"built_for":    header.Code(),
		})
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		refuse(w, r, RouteNotFound.
			New("this module serves no route at "+r.URL.Path).
			With(fault.StringParam("path", r.URL.Path)))
	})

	return mux
}

func answer(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(body)
}

type stdio struct{}

func (stdio) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdio) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdio) Close() error                { return os.Stdin.Close() }
