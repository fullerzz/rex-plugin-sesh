package app

import (
	"context"
	"fmt"
	"time"

	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	pickerpkg "github.com/fullerzz/rex-plugin-sesh/internal/picker"
	"github.com/fullerzz/rex-plugin-sesh/internal/rex"
	"github.com/fullerzz/rex-plugin-sesh/internal/state"
)

// Rex reports neither which session an app shows nor the order it showed
// them, so rex-sesh keeps its own history: each switch the picker makes,
// native or fzf, records the session the picker opened in and the session it
// switched to, for the app it switched. History lives in the state directory
// of the machine that runs the picker.

// historyApp finds the Rex app that opened the picker, the one resolveClient
// picks, with its label.
func historyApp(ctx context.Context, apps appLister, target switchTarget) (rex.AppClient, error) {
	app, listed, err := resolveApp(ctx, apps, target)
	if err != nil || listed {
		return app, err
	}
	// An app named by a flag or REX_CLIENT, which may be a label, comes
	// without its label.
	all, err := apps.AppClients(ctx, "")
	if err != nil {
		return rex.AppClient{}, err
	}
	for _, listed := range all {
		if listed.ID == app.ID || listed.Label == app.ID {
			return listed, nil
		}
	}
	return rex.AppClient{}, fmt.Errorf("no Rex app %s is connected", app.ID)
}

// historyKey names app's history: its device label, which the app keeps on
// every server, or its client ID when it has no label, or "" for no app.
// Apps that share a label share a history.
func historyKey(app rex.AppClient) string {
	switch {
	case app.Label != "":
		return "label:" + app.Label
	case app.ID != "":
		return "id:" + app.ID
	default:
		return ""
	}
}

// switchLockTimeout outlasts the longest switch, a client action that rex
// gives 15 seconds, so a picker waits out another picker's switch of the
// same app.
const switchLockTimeout = 20 * time.Second

// recording names the history that a switch accepted by the app accepted
// goes in, or "" for none, and the switch's origin there. The origin is the
// session the app showed before, when that is known; see shownSessionID.
// Another app, such as a host's only app or one found when the picker opened
// without one, has an unknown origin. A label names it on this machine, and
// so does its ID when idIsKey: a listing of this machine's apps found it.
func (t switchTarget) recording(accepted rex.AppClient, idIsKey bool) (key string, from state.Visit) {
	if !sameApp(t.app, accepted) {
		if accepted.Label == "" && !idIsKey {
			return "", state.Visit{}
		}
		return historyKey(accepted), state.Visit{}
	}
	return historyKey(t.app), state.Visit{SessionID: t.shownSessionID()}
}

// lockSwitches holds the switch lock of the history named key and returns
// its release. Without the lock the switch still happens, and only its place
// in the history may be wrong, so a failure is only a warning.
func (a *App) lockSwitches(key string) (unlock func()) {
	release, err := state.LockSwitches(stateDir(), key, switchLockTimeout)
	if err != nil {
		a.warnf("switching without the history lock: %v", err)
		return func() {}
	}
	return func() {
		if err := release(); err != nil {
			a.warnf("could not release the history lock: %v", err)
		}
	}
}

// recordSwitch records a switch from from to to in the history named key,
// whose revision was revision when Enter was pressed. A switch recorded
// since, such as another picker's that this one waited for, moved the app
// away from from first, so RecordSwitch leaves from out. The switch already
// happened, so failing to record it is only a warning.
func (a *App) recordSwitch(key string, from, to state.Visit, revision uint64) {
	if err := state.RecordSwitch(stateDir(), key, from, to, revision); err != nil {
		a.warnf("could not record session history: %v", err)
	}
}

// sameApp reports whether accepted is opener. An app on this machine named
// by --client carries only that name, an ID or a label; one on a host has
// another ID and the same label.
func sameApp(opener, accepted rex.AppClient) bool {
	if opener.ID == "" {
		return false
	}
	if accepted.ID == opener.ID || (opener.Label != "" && accepted.ID == opener.Label) {
		return true
	}
	return opener.Label != "" && accepted.Label == opener.Label
}

// visitState says whether the picker can switch to a visited session.
type visitState int

const (
	// visitClosed means the session's server answered without listing it.
	visitClosed visitState = iota
	visitOpen
	// visitUnknown means the session's host did not answer, so the session
	// may still be open.
	visitUnknown
)

// pickerHistory is the history the picker shows for app, judged against the
// sessions col lists: the recent order, starting with current, the session
// the app shows, and the previous session. Nothing is written, so opening or
// cancelling the picker keeps the previous session. Without an app, or with
// a history that cannot be read, the previous session is unknown.
func pickerHistory(app, current string, col collection) pickerpkg.ReloadResult {
	open := map[string]bool{}
	var rexSessions []model.Session
	for _, s := range col.Sessions {
		if s.Source == "rex" && s.SessionID != "" {
			open[s.Ref()] = true
			rexSessions = append(rexSessions, s)
		}
	}
	stateOf := func(v state.Visit) visitState {
		if open[visitRef(v)] {
			return visitOpen
		}
		if v.Host != "" && (col.Unlisted[""] || col.Unlisted[v.Host]) {
			return visitUnknown
		}
		return visitClosed
	}

	shown := state.Visit{SessionID: current}
	result := pickerpkg.ReloadResult{RecentSessionIDs: []string{}, RexSessions: rexSessions}
	if shown.SessionID != "" {
		result.RecentSessionIDs = append(result.RecentSessionIDs, visitRef(shown))
	}
	h, err := state.LoadHistory(stateDir())
	if app == "" || err != nil {
		result.LastSessionUnknown = true
		return result
	}
	visits := h.Visits(app)
	for _, v := range visits {
		if v != shown && stateOf(v) == visitOpen {
			result.RecentSessionIDs = append(result.RecentSessionIDs, visitRef(v))
		}
	}
	switch previous, st := previousVisit(visits, shown, stateOf); st {
	case visitOpen:
		result.LastSessionID = visitRef(previous)
	case visitUnknown:
		result.LastSessionUnknown = true
	case visitClosed:
	}
	return result
}

// previousVisit finds the newest visit other than current that is not
// closed. A visit whose host did not answer may be the previous session, so
// the search stops there and reports it unknown. Without a current session,
// the newest visit is taken as current. With no such visit the state is
// visitClosed.
func previousVisit(visits []state.Visit, current state.Visit, stateOf func(state.Visit) visitState) (state.Visit, visitState) {
	if current.SessionID == "" && len(visits) > 0 {
		current = visits[0]
	}
	for _, v := range visits {
		if v == current {
			continue
		}
		if st := stateOf(v); st != visitClosed {
			return v, st
		}
	}
	return state.Visit{}, visitClosed
}

// visitRef is the picker's Session.Ref for a visit.
func visitRef(v state.Visit) string {
	return model.Session{Machine: v.Host, SessionID: v.SessionID}.Ref()
}
