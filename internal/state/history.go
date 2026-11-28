package state

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fullerzz/rex-plugin-sesh/internal/filelock"
)

// Visit names a Rex session that an app switched to: the host whose server
// holds it, by the label "rex hosts" gives it on this machine ("" for this
// machine's own server), and its session ID there. A session ID is unique
// only on its own server, so a visit needs both.
type Visit struct {
	Host      string `json:"host,omitempty"`
	SessionID string `json:"session_id"`
}

// History holds, for each Rex app, the sessions rex-sesh switched it to,
// newest first. Rex reports neither which session an app shows nor the order
// it showed them, so a switch made in the app itself is not recorded.
type History struct {
	Apps map[string]AppHistory `json:"apps,omitempty"`
}

// AppHistory is one app's visits, newest first, and its revision, which
// advances with every switch recorded for the app.
type AppHistory struct {
	Visits   []Visit `json:"visits"`
	Revision uint64  `json:"revision"`
}

var ErrHistoryLockTimeout = errors.New("timed out waiting for history lock")

const (
	maxVisits           = 50
	historyLockTimeout  = 250 * time.Millisecond
	historyLockInterval = 10 * time.Millisecond
)

// HistoryPath is the history file in dir. Its name differs from the
// herdr-sesh history.json, whose format this one does not read.
func HistoryPath(dir string) string { return filepath.Join(dir, "session-history.json") }

// LoadHistory reads the history in dir. A missing file, or dir "", is an
// empty history.
func LoadHistory(dir string) (History, error) {
	var h History
	if dir == "" {
		return h, nil
	}
	b, err := os.ReadFile(HistoryPath(dir))
	if os.IsNotExist(err) {
		return h, nil
	}
	if err != nil {
		return h, err
	}
	return h, json.Unmarshal(b, &h)
}

// Visits returns the sessions app visited, newest first.
func (h History) Visits(app string) []Visit { return h.Apps[app].Visits }

// Revision is the revision of app's history; see RecordSwitch.
func (h History) Revision(app string) uint64 { return h.Apps[app].Revision }

// RecordSwitch records that app switched from one session to another: to
// becomes the newest visit and from, when set, the one before it. from is
// the session app showed when the switch began, with app's history at
// revision. A switch recorded since moved the app away from from first, so
// from is then left out. Switching to the session already shown records
// nothing, so the previous session stays the one before it. History that
// does not decode is replaced.
func RecordSwitch(dir, app string, from, to Visit, revision uint64) error {
	if dir == "" || app == "" || to.SessionID == "" {
		return nil
	}
	return withHistoryLock(dir, func() error {
		h, err := LoadHistory(dir)
		if err != nil {
			if !isJSONDecodeError(err) {
				return err
			}
			h = History{}
		}
		a := h.Apps[app]
		if a.Revision != revision {
			from = Visit{}
		}
		if from == to {
			return nil
		}
		a.Visits = dedupeVisits([]Visit{to, from}, a.Visits)
		a.Revision++
		if h.Apps == nil {
			h.Apps = map[string]AppHistory{}
		}
		h.Apps[app] = a
		return writeJSONFile(HistoryPath(dir), h)
	})
}

// LockSwitches holds app's switch lock until unlock is called, waiting up to
// timeout while another rex-sesh switches the same app. A switch made and
// recorded under the lock reaches the history in the order the app accepted
// it. Without dir or app it locks nothing.
func LockSwitches(dir, app string, timeout time.Duration) (unlock func() error, err error) {
	if dir == "" || app == "" {
		return func() error { return nil }, nil
	}
	sum := sha256.Sum256([]byte(app))
	return lockFile(dir, fmt.Sprintf("switch-%x.lock", sum[:8]), timeout)
}

func withHistoryLock(dir string, fn func() error) error {
	unlock, err := lockFile(dir, "history.lock", historyLockTimeout)
	if err != nil {
		return err
	}
	return errors.Join(fn(), unlock())
}

// lockFile takes an exclusive lock on the file name in dir, waiting up to
// timeout.
func lockFile(dir, name string, timeout time.Duration) (unlock func() error, err error) {
	//nolint:gosec // dir is the trusted rex-sesh state directory supplied to this API.
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	lock, err := filelock.Open(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	for {
		err := filelock.TryLock(lock)
		if err == nil {
			break
		}
		if !errors.Is(err, filelock.ErrWouldBlock) {
			return nil, errors.Join(err, lock.Close())
		}
		if time.Now().Add(historyLockInterval).After(deadline) {
			return nil, errors.Join(fmt.Errorf("%w after %s", ErrHistoryLockTimeout, timeout), lock.Close())
		}
		time.Sleep(historyLockInterval)
	}
	return func() error {
		return errors.Join(filelock.Unlock(lock), lock.Close())
	}, nil
}

// dedupeVisits puts front before rest, keeping each session's first place,
// and keeps at most maxVisits.
func dedupeVisits(front, rest []Visit) []Visit {
	seen := map[Visit]bool{}
	out := make([]Visit, 0, len(front)+len(rest))
	for _, v := range append(front, rest...) {
		if v.SessionID == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
		if len(out) == maxVisits {
			break
		}
	}
	return out
}

func isJSONDecodeError(err error) bool {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	return errors.As(err, &syntaxErr) || errors.As(err, &typeErr)
}
