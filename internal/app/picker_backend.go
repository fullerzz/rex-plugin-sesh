package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/fullerzz/rex-plugin-sesh/internal/config"
	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	pickerpkg "github.com/fullerzz/rex-plugin-sesh/internal/picker"
	"github.com/fullerzz/rex-plugin-sesh/internal/settings"
)

// The native picker switches through the backend, so a refusal keeps it open.
var _ pickerpkg.Switcher = (*pickerBackend)(nil)

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
	target           switchTarget

	// lastStatuses is the previous RefreshAgentStatuses result.
	statusMu     sync.Mutex
	lastStatuses map[string]string
}

// Switch asks the Rex app to show s. The picker calls it on Enter and stays
// open with the error when it fails.
func (b *pickerBackend) Switch(ctx context.Context, s model.Session) error {
	return b.app.switchTo(ctx, b.target, s)
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

// RefreshAgentStatuses maps each Rex session ID, on this machine and every
// host, to the most urgent state its programs report. The picker calls it
// every second, and clears the status of a session missing from the map. So
// while any host fails to answer, sessions missing from this refresh keep
// their last status.
func (b *pickerBackend) RefreshAgentStatuses() (map[string]string, error) {
	src := b.app.rexSource("")
	sessions, _ := src.List(b.ctx)
	if src.err != nil {
		return nil, src.err
	}
	statuses := make(map[string]string, len(sessions.Directory))
	for _, s := range sessions.Ordered() {
		statuses[s.SessionID] = s.AgentStatus
	}
	b.statusMu.Lock()
	defer b.statusMu.Unlock()
	if len(src.hostWarnings) > 0 {
		for id, status := range b.lastStatuses {
			if _, ok := statuses[id]; !ok {
				statuses[id] = status
			}
		}
	}
	b.lastStatuses = statuses
	return statuses, nil
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
