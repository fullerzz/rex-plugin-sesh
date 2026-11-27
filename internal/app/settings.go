package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/fullerzz/rex-plugin-sesh/internal/config"
	"github.com/fullerzz/rex-plugin-sesh/internal/settings"
	"github.com/fullerzz/rex-plugin-sesh/internal/state"
)

func (a *App) settingsSaved(_ string) error {
	dir := stateDir()
	if dir == "" {
		return nil
	}
	//nolint:gosec // rex-sesh-owned cache directory under the XDG state home.
	if err := os.Remove(state.SessionCachePath(dir)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale session cache: %w", err)
	}
	return nil
}
func (a *App) editSettings(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("config edit", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	path := fs.String("config", "", "config to edit (defaults to normal discovery)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("config edit accepts only --config <path>")
	}
	editor, err := settings.Open(config.LoadOptions{Path: *path, Warn: a.Err}, a.settingsSaved)
	if err != nil {
		return err
	}
	_, err = settings.RunModel(ctx, a.Out, editor)
	return err
}
