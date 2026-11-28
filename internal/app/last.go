package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/fullerzz/rex-plugin-sesh/internal/rex"
	"github.com/fullerzz/rex-plugin-sesh/internal/state"
)

// last switches directly to the previous open session of the selected app.
func (a *App) last(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("last", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	var target switchTarget
	var openerVisit state.Visit
	fs.StringVar(&target.client, "client", "", "the Rex app to switch, as a client ID or label")
	fs.StringVar(&target.opener, "opener", "", "the Rex app that opened the action, as a client ID")
	fs.StringVar(&openerVisit.SessionID, "opener-session", "", "the session shown to the app that opened the action")
	fs.StringVar(&openerVisit.Host, "opener-host", "", "the Rex host holding the opener session")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("last takes no positional arguments")
	}
	if openerVisit.Host != "" && openerVisit.SessionID == "" {
		return errors.New("--opener-host requires --opener-session")
	}
	if openerVisit.SessionID != "" && target.opener == "" {
		return errors.New("--opener-session requires --opener")
	}
	// Take the revision before observing the opener. Another switch can
	// change the app's session while Rex is listing sessions or we wait.
	before, err := state.LoadHistory(stateDir())
	if err != nil {
		return fmt.Errorf("read session history: %w", err)
	}
	src := a.rexSource(os.Getenv("REX_BLOCK"))
	_, _ = src.List(ctx)
	if src.err != nil {
		return fmt.Errorf("list Rex sessions: %w", src.err)
	}
	target.currentSessionID = src.current
	client := rex.NewClient()
	app, err := historyApp(ctx, client, target)
	if err != nil {
		return err
	}
	target.app, target.client = app, app.ID
	key := historyKey(app)
	// Serialize lookup, selection and recording with other switches for this app.
	unlock, err := state.LockSwitches(stateDir(), key, switchLockTimeout)
	if err != nil {
		return fmt.Errorf("lock session history: %w", err)
	}
	defer func() {
		if err := unlock(); err != nil {
			a.warnf("could not release the history lock: %v", err)
		}
	}()
	h, err := state.LoadHistory(stateDir())
	if err != nil {
		return fmt.Errorf("read session history: %w", err)
	}
	var origin state.Visit
	if h.Revision(key) == before.Revision(key) {
		origin.SessionID = target.shownSessionID()
		if openerVisit.SessionID != "" && target.opener == app.ID {
			origin = openerVisit
		}
	}
	// With no reliable opener, previousVisit takes the newest recorded
	// visit as current and RecordSwitch retains it behind the destination.
	stateOf := visitStates(src.rows, src.listedHosts)
	previous, status := previousVisit(h.Visits(key), origin, stateOf)
	switch status {
	case visitUnknown:
		return fmt.Errorf("previous session unavailable: host %q did not list its sessions", previous.Host)
	case visitClosed:
		return errors.New("no previous open session in this app's history")
	}
	if previous.Host != "" {
		host := rex.NewHostClient(previous.Host)
		remote, err := resolveHostClient(ctx, client, host, previous.Host, target)
		if err != nil {
			return err
		}
		local, err := client.AppClients(ctx, "")
		if err != nil {
			return err
		}
		if match, ok := uniqueLabel(local, remote.Label); !ok || match.ID != app.ID {
			return fmt.Errorf("previous session unavailable: cannot identify this app on host %q", previous.Host)
		}
		client, app = host, remote
	}
	if err := client.SelectSession(ctx, app.ID, previous.SessionID); err != nil {
		return err
	}
	a.recordSwitch(key, origin, previous, h.Revision(key))
	return nil
}
