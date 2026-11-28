package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fullerzz/rex-plugin-sesh/internal/filelock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func local(id string) Visit { return Visit{SessionID: id} }

// recordNow records a switch that began with app's history as it is now, as
// a switch does when no other picker switches the app meanwhile.
func recordNow(dir, app string, from, to Visit) error {
	h, _ := LoadHistory(dir)
	return RecordSwitch(dir, app, from, to, h.Revision(app))
}

func visits(t *testing.T, dir, app string) []Visit {
	t.Helper()
	h, err := LoadHistory(dir)
	require.NoError(t, err)
	return h.Visits(app)
}

func TestLoadHistoryWithoutAFileIsEmpty(t *testing.T) {
	h, err := LoadHistory(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, h.Visits("label:mac"))
	h, err = LoadHistory("")
	require.NoError(t, err)
	assert.Empty(t, h.Visits("label:mac"))
}

func TestRecordSwitchPutsTheOriginSecond(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, recordNow(dir, "label:mac", local("A"), local("C")))
	assert.Equal(t, []Visit{local("C"), local("A")}, visits(t, dir, "label:mac"))
	require.NoError(t, recordNow(dir, "label:mac", local("C"), local("A")))
	assert.Equal(t, []Visit{local("A"), local("C")}, visits(t, dir, "label:mac"))
}

// Repeated visits move a session forward instead of listing it twice.
func TestRecordSwitchListsEachSessionOnce(t *testing.T) {
	dir := t.TempDir()
	for _, step := range [][2]string{{"A", "C"}, {"C", "B"}, {"B", "C"}, {"C", "A"}} {
		require.NoError(t, recordNow(dir, "label:mac", local(step[0]), local(step[1])))
	}
	assert.Equal(t, []Visit{local("A"), local("C"), local("B")}, visits(t, dir, "label:mac"))
}

// After a switch rex-sesh did not see, such as one made with Change Session…,
// the session the picker opened in is recorded as the origin even when the
// destination already heads the history.
func TestRecordSwitchRecordsAnOriginItDidNotSwitchTo(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, recordNow(dir, "label:mac", local("A"), local("C")))
	require.NoError(t, recordNow(dir, "label:mac", local("B"), local("C")))
	assert.Equal(t, []Visit{local("C"), local("B"), local("A")}, visits(t, dir, "label:mac"))
}

// B heads the history, and the app moves to A outside rex-sesh. Two pickers
// in A take Enter at once: one picks B, the other C, after waiting for the
// first. The first left B at the head, but its switch still advanced the
// revision, so the second knows the app had left A.
func TestRecordSwitchDropsAnOriginThatAnotherSwitchLeft(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, recordNow(dir, "label:mac", local("X"), local("B")))
	h, err := LoadHistory(dir)
	require.NoError(t, err)
	atEnter := h.Revision("label:mac")
	require.NoError(t, RecordSwitch(dir, "label:mac", local("A"), local("B"), atEnter))
	require.NoError(t, RecordSwitch(dir, "label:mac", local("A"), local("C"), atEnter))
	assert.Equal(t, []Visit{local("C"), local("B"), local("A"), local("X")}, visits(t, dir, "label:mac"))
	h, err = LoadHistory(dir)
	require.NoError(t, err)
	assert.Equal(t, atEnter+2, h.Revision("label:mac"))
	assert.Zero(t, h.Revision("label:ipad"), "each app has its own revision")
}

func TestRecordSwitchToTheCurrentSessionRecordsNothing(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, recordNow(dir, "label:mac", local("C"), local("C")))
	assert.NoFileExists(t, HistoryPath(dir))
	require.NoError(t, recordNow(dir, "label:mac", local("A"), local("C")))
	require.NoError(t, recordNow(dir, "label:mac", local("C"), local("C")))
	assert.Equal(t, []Visit{local("C"), local("A")}, visits(t, dir, "label:mac"))
}

func TestRecordSwitchWithoutAnOriginRecordsTheDestination(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, recordNow(dir, "label:mac", Visit{}, local("C")))
	assert.Equal(t, []Visit{local("C")}, visits(t, dir, "label:mac"))
}

func TestRecordSwitchKeepsEachAppsHistory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, recordNow(dir, "label:mac", local("A"), local("B")))
	require.NoError(t, recordNow(dir, "label:ipad", local("C"), local("A")))
	assert.Equal(t, []Visit{local("B"), local("A")}, visits(t, dir, "label:mac"))
	assert.Equal(t, []Visit{local("A"), local("C")}, visits(t, dir, "label:ipad"))
}

// A session ID is unique only on its own server, so the same ID on two
// hosts is two sessions.
func TestRecordSwitchTellsHostsApart(t *testing.T) {
	dir := t.TempDir()
	devbox := Visit{Host: "devbox", SessionID: "session:1"}
	require.NoError(t, recordNow(dir, "label:mac", local("session:1"), devbox))
	assert.Equal(t, []Visit{devbox, local("session:1")}, visits(t, dir, "label:mac"))
}

func TestRecordSwitchKeepsTheNewestVisits(t *testing.T) {
	dir := t.TempDir()
	for i := range maxVisits + 5 {
		require.NoError(t, recordNow(dir, "label:mac", Visit{}, local(fmt.Sprint(i))))
	}
	got := visits(t, dir, "label:mac")
	require.Len(t, got, maxVisits)
	assert.Equal(t, local(fmt.Sprint(maxVisits+4)), got[0])
	assert.Equal(t, local("5"), got[maxVisits-1])
}

func TestRecordSwitchNeedsADirAndAnApp(t *testing.T) {
	require.NoError(t, recordNow("", "label:mac", local("A"), local("B")))
	dir := t.TempDir()
	require.NoError(t, recordNow(dir, "", local("A"), local("B")))
	require.NoError(t, recordNow(dir, "label:mac", local("A"), Visit{}))
	assert.NoFileExists(t, HistoryPath(dir))
}

func TestRecordSwitchReplacesHistoryThatDoesNotDecode(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(HistoryPath(dir), []byte("{"), 0o600))
	_, err := LoadHistory(dir)
	require.Error(t, err)
	require.NoError(t, recordNow(dir, "label:mac", local("A"), local("B")))
	assert.Equal(t, []Visit{local("B"), local("A")}, visits(t, dir, "label:mac"))
}

// Pickers in several apps may switch at once; the lock keeps every write.
func TestRecordSwitchKeepsConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	const apps = 8
	errs := make(chan error, apps)
	var wg sync.WaitGroup
	for i := range apps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- recordNow(dir, fmt.Sprintf("label:app%d", i), local("A"), local("B"))
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	h, err := LoadHistory(dir)
	require.NoError(t, err)
	assert.Len(t, h.Apps, apps)
}

// A second rex-sesh switching the same app waits for the first; another app
// does not.
func TestLockSwitchesWaitsOnlyForTheSameApp(t *testing.T) {
	dir := t.TempDir()
	unlock, err := LockSwitches(dir, "label:mac", time.Second)
	require.NoError(t, err)
	_, err = LockSwitches(dir, "label:mac", 50*time.Millisecond)
	require.ErrorIs(t, err, ErrHistoryLockTimeout)
	other, err := LockSwitches(dir, "label:ipad", 50*time.Millisecond)
	require.NoError(t, err)
	require.NoError(t, other())
	require.NoError(t, unlock())
	again, err := LockSwitches(dir, "label:mac", 50*time.Millisecond)
	require.NoError(t, err)
	require.NoError(t, again())

	none, err := LockSwitches(dir, "", time.Second)
	require.NoError(t, err)
	require.NoError(t, none())
}

func TestRecordSwitchTimesOutWaitingForTheLock(t *testing.T) {
	dir := t.TempDir()
	lock, err := filelock.Open(filepath.Join(dir, "history.lock"))
	require.NoError(t, err)
	defer func() { _ = lock.Close() }()
	require.NoError(t, filelock.TryLock(lock))
	defer func() { _ = filelock.Unlock(lock) }()

	done := make(chan error, 1)
	go func() { done <- recordNow(dir, "label:mac", local("A"), local("B")) }()
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrHistoryLockTimeout)
	case <-time.After(2 * time.Second):
		require.FailNow(t, "RecordSwitch stayed blocked for two seconds")
	}
	assert.NoFileExists(t, HistoryPath(dir))
}
