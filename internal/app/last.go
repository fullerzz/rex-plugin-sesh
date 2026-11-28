package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/fullerzz/rex-plugin-sesh/internal/rex"
	"github.com/fullerzz/rex-plugin-sesh/internal/state"
)

// last switches directly to the previous open session of the selected app.
func (a *App) last(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("last", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	var target switchTarget
	var actionSession state.Visit
	fs.StringVar(&actionSession.SessionID, "context-session", "", "the invoking terminal's session for app selection")
	fs.StringVar(&actionSession.Host, "context-host", "", "the Rex host holding the invoking terminal's session")
	var openerVisit state.Visit
	captured := fs.Bool("opener-history-stdin", false, "read the history captured with the action context from stdin")
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
	var before state.History
	if *captured {
		if a.In == nil {
			return errors.New("missing captured session history")
		}
		data, err := io.ReadAll(a.In)
		if err == nil {
			err = json.Unmarshal(data, &before)
		}
		if err != nil {
			return fmt.Errorf("read captured session history: %w", err)
		}
	} else {
		// CLI callers and older modules observe history before listing Rex sessions.
		var err error
		before, err = state.LoadHistory(stateDir())
		if err != nil {
			return fmt.Errorf("read session history: %w", err)
		}
	}
	src := a.rexSource(os.Getenv("REX_BLOCK"))
	_, _ = src.List(ctx)
	if src.err != nil {
		return fmt.Errorf("list Rex sessions: %w", src.err)
	}
	target.currentSessionID = src.current
	client := rex.NewClient()
	app, err := lastActionApp(ctx, client, target, actionSession)
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

// A CLI action identifies a terminal session, but its client is the command
// itself. Use the session only to select an app, never as a history origin.
func lastActionApp(ctx context.Context, client *rex.Client, target switchTarget, session state.Visit) (rex.AppClient, error) {
	if session.Host != "" && session.SessionID == "" {
		return rex.AppClient{}, errors.New("--context-host requires --context-session")
	}
	if session.SessionID != "" {
		target.currentSessionID = session.SessionID
		if session.Host != "" && target.client == "" && target.opener == "" && os.Getenv("REX_CLIENT") == "" {
			// Resolve on the session's own server, then match its app on
			// the config server by device label. IDs are server-local.
			apps, err := client.AppClients(ctx, "")
			if err != nil {
				return rex.AppClient{}, err
			}
			return matchHostApp(ctx, rex.NewHostClient(session.Host), apps, "the config server", target)
		}
	}
	return historyApp(ctx, client, target)
}
