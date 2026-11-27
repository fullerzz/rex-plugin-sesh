package app

import (
	"context"
	"errors"

	"github.com/fullerzz/rex-plugin-sesh/internal/config"
	pickerpkg "github.com/fullerzz/rex-plugin-sesh/internal/picker"
	"github.com/fullerzz/rex-plugin-sesh/internal/settings"
)

// errCloseUnsupported reports that the picker cannot close sessions yet.
var errCloseUnsupported = errors.New("closing sessions is not supported yet")

// pickerBackend serves the native picker from config and zoxide sessions. It
// has no running sessions, so it neither closes them nor reports their status.
type pickerBackend struct {
	app *App
	// ctx outlives each reload; display options keep it for later previews.
	ctx              context.Context
	cfg              config.Config
	activeConfigPath string
}

func (b *pickerBackend) CloseWorkspace(context.Context, string) error {
	return errCloseUnsupported
}

func (b *pickerBackend) ReloadPicker(ctx context.Context) (pickerpkg.ReloadResult, error) {
	sessions, err := b.app.collect(ctx, b.cfg, "", true)
	return pickerpkg.ReloadResult{Sessions: sessions}, err
}

func (b *pickerBackend) RefreshAgentStatuses() (map[string]string, error) {
	return map[string]string{}, nil
}

func (b *pickerBackend) OpenSettings() (settings.Model, error) {
	return settings.Open(config.LoadOptions{Path: b.activeConfigPath, Warn: b.app.Err}, b.app.settingsSaved)
}

func (b *pickerBackend) ReloadSettings(ctx context.Context, saved settings.Result) (pickerpkg.DisplayOptions, pickerpkg.ReloadResult, error) {
	// The editor already saved this file, even if applying it fails.
	b.activeConfigPath = saved.Path
	cfg, _, err := config.Load(config.LoadOptions{Path: saved.Path, Warn: b.app.Err})
	if err != nil {
		return pickerpkg.DisplayOptions{}, pickerpkg.ReloadResult{}, err
	}
	sessions, err := b.app.collect(ctx, cfg, "", true)
	if err != nil {
		return pickerpkg.DisplayOptions{}, pickerpkg.ReloadResult{}, err
	}
	b.cfg = cfg
	return b.displayOptions(cfg), pickerpkg.ReloadResult{Sessions: sessions}, nil
}

// displayOptions binds the picker's long-lived context, not a reload's.
func (b *pickerBackend) displayOptions(cfg config.Config) pickerpkg.DisplayOptions {
	return pickerOptionsFromConfig(b.ctx, b.app.Out, cfg)
}
