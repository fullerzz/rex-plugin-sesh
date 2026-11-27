package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/fullerzz/rex-plugin-sesh/internal/config"
	pickerpkg "github.com/fullerzz/rex-plugin-sesh/internal/picker"
	"github.com/fullerzz/rex-plugin-sesh/internal/settings"
)

// errCloseUnsupported reports that the picker cannot close sessions yet.
var errCloseUnsupported = errors.New("closing sessions is not supported yet")

// pickerBackend serves the native picker. It cannot close Rex sessions or
// report their status yet.
type pickerBackend struct {
	app *App
	// ctx outlives each reload; display options keep it for later previews.
	ctx              context.Context
	cfg              config.Config
	activeConfigPath string
}

// collect lists the picker's rows. It reports an unavailable Rex as an error,
// which the picker shows, because the picker owns the screen.
func (b *pickerBackend) collect(ctx context.Context, cfg config.Config) (collection, error) {
	col, err := b.app.collectPicker(ctx, cfg)
	if err == nil && col.RexErr != nil {
		err = fmt.Errorf("list Rex sessions: %w", col.RexErr)
	}
	return col, err
}

func (b *pickerBackend) CloseWorkspace(context.Context, string) error {
	return errCloseUnsupported
}

func (b *pickerBackend) ReloadPicker(ctx context.Context) (pickerpkg.ReloadResult, error) {
	col, err := b.collect(ctx, b.cfg)
	if err != nil {
		return pickerpkg.ReloadResult{}, err
	}
	return pickerpkg.ReloadResult{Sessions: col.Sessions}, nil
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
	col, err := b.collect(ctx, cfg)
	if err != nil {
		return pickerpkg.DisplayOptions{}, pickerpkg.ReloadResult{}, err
	}
	b.cfg = cfg
	return b.displayOptions(cfg), pickerpkg.ReloadResult{Sessions: col.Sessions}, nil
}

// displayOptions binds the picker's long-lived context, not a reload's.
func (b *pickerBackend) displayOptions(cfg config.Config) pickerpkg.DisplayOptions {
	return pickerOptionsFromConfig(b.ctx, b.app.Out, cfg)
}
