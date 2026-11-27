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

// pickerBackend serves the native picker. It cannot close Rex sessions or
// report their status yet.
type pickerBackend struct {
	app *App
	// ctx outlives each reload; display options keep it for later previews.
	ctx              context.Context
	cfg              config.Config
	activeConfigPath string
	// warnings wait for the picker to leave the screen; see flushWarnings.
	warnings []string
}

func (b *pickerBackend) noteRexErr(err error) {
	if warning := rexWarning(err); warning != "" {
		b.warnings = append(b.warnings, warning)
	}
}

// flushWarnings prints the warnings once the picker no longer owns the screen.
func (b *pickerBackend) flushWarnings() {
	for _, warning := range b.warnings {
		b.app.warnf("%s", warning)
	}
	b.warnings = nil
}

func (b *pickerBackend) CloseWorkspace(context.Context, string) error {
	return errCloseUnsupported
}

func (b *pickerBackend) ReloadPicker(ctx context.Context) (pickerpkg.ReloadResult, error) {
	col, err := b.app.collect(ctx, b.cfg, "", true)
	if err != nil {
		return pickerpkg.ReloadResult{}, err
	}
	b.noteRexErr(col.RexErr)
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
	col, err := b.app.collect(ctx, cfg, "", true)
	if err != nil {
		return pickerpkg.DisplayOptions{}, pickerpkg.ReloadResult{}, err
	}
	b.noteRexErr(col.RexErr)
	b.cfg = cfg
	return b.displayOptions(cfg), pickerpkg.ReloadResult{Sessions: col.Sessions}, nil
}

// displayOptions binds the picker's long-lived context, not a reload's.
func (b *pickerBackend) displayOptions(cfg config.Config) pickerpkg.DisplayOptions {
	return pickerOptionsFromConfig(b.ctx, b.app.Out, cfg)
}
