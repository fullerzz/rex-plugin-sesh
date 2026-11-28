package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fullerzz/rex-plugin-sesh/internal/config"
	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	pickerpkg "github.com/fullerzz/rex-plugin-sesh/internal/picker"
	"github.com/fullerzz/rex-plugin-sesh/internal/rex"
	"github.com/fullerzz/rex-plugin-sesh/internal/settings"
	"github.com/fullerzz/rex-plugin-sesh/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useHistoryDir gives the test a state directory of its own and returns it.
func useHistoryDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("REX_SESH_STATE_DIR", dir)
	return dir
}

func recordSwitches(t *testing.T, app string, steps ...[2]state.Visit) {
	t.Helper()
	for _, step := range steps {
		h, err := state.LoadHistory(stateDir())
		require.NoError(t, err)
		require.NoError(t, state.RecordSwitch(stateDir(), app, step[0], step[1], h.Revision(app)))
	}
}

func localVisit(id string) state.Visit { return state.Visit{SessionID: id} }

func historyOf(t *testing.T, app string) []state.Visit {
	t.Helper()
	h, err := state.LoadHistory(stateDir())
	require.NoError(t, err)
	return h.Visits(app)
}

func TestHistoryAppResolvesLabelsToClientIDs(t *testing.T) {
	apps := &fakeApps{all: []string{"client:mac", "client:ipad", "client:bare"}, labels: map[string]string{"client:mac": "Studio Mac", "client:ipad": "iPad"}}
	for client, want := range map[string]string{
		"client:mac":  "id:client:mac",
		"iPad":        "id:client:ipad",
		"client:bare": "id:client:bare",
	} {
		got, err := historyApp(t.Context(), apps, switchTarget{client: client})
		require.NoError(t, err)
		assert.Equal(t, want, historyKey(got), client)
	}
	_, err := historyApp(t.Context(), apps, switchTarget{client: "client:gone"})
	require.EqualError(t, err, "no unique Rex app matches client:gone")
	_, err = historyApp(t.Context(), apps, switchTarget{})
	require.EqualError(t, err, "3 Rex apps are connected; choose one with --client")
	assert.Empty(t, historyKey(rex.AppClient{}))
}

// The listing that finds the only app also gives its label, so the picker
// lists apps once before it opens.
func TestHistoryAppListsAppsOnce(t *testing.T) {
	apps := &fakeApps{all: []string{"client:mac"}, labels: map[string]string{"client:mac": "Studio Mac"}}
	got, err := historyApp(t.Context(), apps, switchTarget{})
	require.NoError(t, err)
	assert.Equal(t, rex.AppClient{ID: "client:mac", Label: "Studio Mac"}, got)
	assert.Equal(t, []string{""}, apps.calls)
}

// abc lists sessions A, B and C on this machine, in that order, with C
// holding the picker, and a session on devbox that shares A's ID.
func abc(current string) collection {
	sessions := []model.Session{
		{Source: "rex", Name: "A", SessionID: "session:a"},
		{Source: "rex", Name: "B", SessionID: "session:b"},
		{Source: "rex", Name: "C", SessionID: "session:c"},
		{Source: "rex", Machine: "devbox", Name: "train", SessionID: "session:a"},
		{Source: "zoxide", Name: "src", Path: "/src"},
	}
	for i := range sessions {
		sessions[i].Attached = sessions[i].Machine == "" && sessions[i].Name == current
	}
	return collection{RexSessions: sessions[:4], ListedHosts: map[string]bool{"devbox": true}, Sessions: sessions, CurrentSessionID: currentOf(sessions)}
}

// shownHistory is pickerHistory for an app known to show the picker's
// session, as one that opened it from a key does.
func shownHistory(app string, col collection) pickerpkg.ReloadResult {
	return pickerHistory(app, col.CurrentSessionID, col)
}

func currentOf(sessions []model.Session) string {
	for _, s := range sessions {
		if s.Attached {
			return s.SessionID
		}
	}
	return ""
}

func TestPickerHistoryWithNoHistory(t *testing.T) {
	useHistoryDir(t)
	got := shownHistory("label:mac", abc("C"))
	assert.Equal(t, []string{"session:c"}, got.RecentSessionIDs)
	assert.Empty(t, got.LastSessionID)
	assert.False(t, got.LastSessionUnknown, "no history is not unknown history")
	assert.Len(t, got.RexSessions, 4)
}

// A → C, then the picker opens in C: A is the previous session, even though
// B sits between them in Rex order.
func TestPickerHistoryNamesTheSessionBeforeTheCurrentOne(t *testing.T) {
	useHistoryDir(t)
	recordSwitches(t, "label:mac", [2]state.Visit{localVisit("session:a"), localVisit("session:c")})
	got := shownHistory("label:mac", abc("C"))
	assert.Equal(t, "session:a", got.LastSessionID)
	assert.Equal(t, []string{"session:c", "session:a"}, got.RecentSessionIDs)

	// C → A → C: repeated visits keep one place each.
	recordSwitches(t, "label:mac",
		[2]state.Visit{localVisit("session:c"), localVisit("session:a")},
		[2]state.Visit{localVisit("session:a"), localVisit("session:c")})
	got = shownHistory("label:mac", abc("C"))
	assert.Equal(t, "session:a", got.LastSessionID)
	assert.Equal(t, []string{"session:c", "session:a"}, got.RecentSessionIDs)
	assert.Equal(t, []state.Visit{localVisit("session:c"), localVisit("session:a")}, historyOf(t, "label:mac"))
}

// rex-sesh cannot see a switch made with the app's own commands. After A → C
// through the picker and C → B with Change Session…, the picker in B names C, the
// last session it recorded.
func TestPickerHistoryAfterASwitchItDidNotSee(t *testing.T) {
	useHistoryDir(t)
	recordSwitches(t, "label:mac", [2]state.Visit{localVisit("session:a"), localVisit("session:c")})
	got := shownHistory("label:mac", abc("B"))
	assert.Equal(t, "session:c", got.LastSessionID)
	assert.Equal(t, []string{"session:b", "session:c", "session:a"}, got.RecentSessionIDs)
}

func TestPickerHistoryKeepsEachAppsHistory(t *testing.T) {
	useHistoryDir(t)
	recordSwitches(t, "label:mac", [2]state.Visit{localVisit("session:a"), localVisit("session:c")})
	recordSwitches(t, "label:ipad", [2]state.Visit{localVisit("session:b"), localVisit("session:c")})
	assert.Equal(t, "session:a", shownHistory("label:mac", abc("C")).LastSessionID)
	assert.Equal(t, "session:b", shownHistory("label:ipad", abc("C")).LastSessionID)
}

// devbox's session shares A's ID, so only the host tells them apart.
func TestPickerHistoryTellsHostsApart(t *testing.T) {
	useHistoryDir(t)
	devbox := state.Visit{Host: "devbox", SessionID: "session:a"}
	recordSwitches(t, "label:mac",
		[2]state.Visit{localVisit("session:b"), devbox},
		[2]state.Visit{devbox, localVisit("session:a")})
	got := shownHistory("label:mac", abc("A"))
	assert.Equal(t, model.Session{Machine: "devbox", SessionID: "session:a"}.Ref(), got.LastSessionID)
	assert.Equal(t, []string{"session:a", "devbox\x00session:a", "session:b"}, got.RecentSessionIDs)
}

// fakeRexTwins lists a session api on this machine, holding the picker, and
// one on devbox with the same label and session ID.
const fakeRexTwins = `case "$*" in
  *"hosts --json"*) printf '%s' '{"hosts":[{"host":{"label":"devbox"}}]}' ;;
  *"-S devbox do -"*) printf '%s' '{"sessions":[{"session_id":"session:1","label":"api"}]}' ;;
  *) printf '%s' '{"sessions":[{"session_id":"session:1","label":"api","current":true}]}' ;;
esac`

// The picker's own listing keeps same-named sessions on two hosts apart, so
// history can name either.
func TestPickerHistoryNamesASameNamedSessionOnAnotherHost(t *testing.T) {
	useHistoryDir(t)
	useFakeRex(t, fakeRexTwins)
	devbox := state.Visit{Host: "devbox", SessionID: "session:1"}
	recordSwitches(t, "label:mac", [2]state.Visit{localVisit("session:1"), devbox}, [2]state.Visit{devbox, localVisit("session:1")})
	col, err := (&App{}).collectPicker(t.Context(), config.Default())
	require.NoError(t, err)
	require.Equal(t, []string{"api", "[devbox] api"}, sessionNames(col.Sessions))
	got := shownHistory("label:mac", col)
	assert.Equal(t, visitRef(devbox), got.LastSessionID)
	assert.Equal(t, []string{"session:1", visitRef(devbox)}, got.RecentSessionIDs)
}

// Same-ID sessions on two hosts keep their own statuses.
func TestRefreshKeepsTwinSessionStatusesApart(t *testing.T) {
	useFakeRex(t, strings.NewReplacer(`"label":"api"}`, `"label":"api","agent_status":"idle"}`, `"current":true}`, `"current":true,"agent_status":"blocked"}`).Replace(fakeRexTwins))
	backend := &pickerBackend{app: &App{}, ctx: t.Context(), cfg: config.Default()}
	statuses, err := backend.RefreshAgentStatuses()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"session:1": "blocked", "devbox\x00session:1": "idle"}, statuses)
}

func TestPickerHistorySkipsClosedSessions(t *testing.T) {
	useHistoryDir(t)
	recordSwitches(t, "label:mac",
		[2]state.Visit{localVisit("session:b"), localVisit("session:gone")},
		[2]state.Visit{{Host: "devbox", SessionID: "session:gone"}, localVisit("session:c")})
	// devbox answered without its session, so that session is closed too.
	got := shownHistory("label:mac", abc("C"))
	assert.Equal(t, "session:b", got.LastSessionID)
	assert.Equal(t, []string{"session:c", "session:b"}, got.RecentSessionIDs)
	assert.Len(t, historyOf(t, "label:mac"), 4, "reading history prunes nothing")
}

// A host that does not answer may still hold the previous session, so the
// picker reports it unknown instead of skipping to an older one, and keeps it.
func TestPickerHistoryDoesNotTakeAnUnreachableHostAsClosed(t *testing.T) {
	useHistoryDir(t)
	away := state.Visit{Host: "away", SessionID: "session:x"}
	recordSwitches(t, "label:mac",
		[2]state.Visit{localVisit("session:a"), away},
		[2]state.Visit{away, localVisit("session:c")})
	col := abc("C")
	got := shownHistory("label:mac", col)
	assert.Empty(t, got.LastSessionID)
	assert.True(t, got.LastSessionUnknown)
	col.ListedHosts["away"] = true
	assert.Equal(t, "session:a", shownHistory("label:mac", col).LastSessionID, "away answered")
	assert.Equal(t, []state.Visit{localVisit("session:c"), away, localVisit("session:a")}, historyOf(t, "label:mac"))
}

func TestCollectPickerTracksSuccessfulEmptyHostListing(t *testing.T) {
	t.Setenv("FAKE_REX_LOG", filepath.Join(t.TempDir(), "rex.log"))
	useFakeRex(t, strings.ReplaceAll(fakeRexHosts, `[{"session_id":"session:9","label":"train","agent_status":"working"}]`, `[]`))
	col, err := (&App{}).collectPicker(t.Context(), config.Default())
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"devbox": true}, col.ListedHosts)
}

func TestPickerHistoryWithoutAnAppIsUnknown(t *testing.T) {
	useHistoryDir(t)
	recordSwitches(t, "label:mac", [2]state.Visit{localVisit("session:a"), localVisit("session:c")})
	got := shownHistory("", abc("C"))
	assert.True(t, got.LastSessionUnknown)
	assert.Equal(t, []string{"session:c"}, got.RecentSessionIDs)
}

func TestPickerHistoryThatDoesNotDecodeIsUnknown(t *testing.T) {
	dir := useHistoryDir(t)
	require.NoError(t, os.WriteFile(state.HistoryPath(dir), []byte("{"), 0o600))
	assert.True(t, shownHistory("label:mac", abc("C")).LastSessionUnknown)
}

func TestSwitchRecordsOnlyWhatTheAppAccepted(t *testing.T) {
	useHistoryDir(t)
	t.Setenv("FAKE_REX_LOG", filepath.Join(t.TempDir(), "rex.log"))
	useFakeRex(t, fakeRexWithApp)
	a := &App{Err: &bytes.Buffer{}}
	target := switchTarget{opener: "client:app", app: rex.AppClient{ID: "client:app"}, currentSessionID: "session:1"}
	web := model.Session{Source: "rex", Name: "web", SessionID: "session:2"}

	t.Setenv("FAKE_REX_REFUSE", "Remote Control is off")
	require.Error(t, a.switchTo(t.Context(), target, web))
	assert.NoFileExists(t, state.HistoryPath(stateDir()), "a refused switch keeps the history")

	t.Setenv("FAKE_REX_REFUSE", "")
	require.NoError(t, a.switchTo(t.Context(), target, web))
	assert.Equal(t, []state.Visit{localVisit("session:2"), localVisit("session:1")}, historyOf(t, "id:client:app"))

	// Choosing the session the picker is in changes nothing.
	require.NoError(t, a.switchTo(t.Context(), switchTarget{opener: "client:app", app: rex.AppClient{ID: "client:app"}, currentSessionID: "session:2"}, web))
	assert.Equal(t, []state.Visit{localVisit("session:2"), localVisit("session:1")}, historyOf(t, "id:client:app"))

	// The picker opened without knowing its app, but the switch found the
	// app by listing this machine's apps. Its ID names it here, so the switch
	// is recorded, without an origin.
	ops := model.Session{Source: "rex", Name: "ops", SessionID: "session:3"}
	require.NoError(t, a.switchTo(t.Context(), switchTarget{currentSessionID: "session:1"}, ops))
	assert.Equal(t, []state.Visit{localVisit("session:3"), localVisit("session:2"), localVisit("session:1")}, historyOf(t, "id:client:app"))
}

// The picker opened in A, and another picker switched the app to B before
// Enter here. The app showed A again for this picker to take Enter, so A is
// still the origin.
func TestSwitchTakesItsOriginWhenEnterIsPressed(t *testing.T) {
	useHistoryDir(t)
	t.Setenv("FAKE_REX_LOG", filepath.Join(t.TempDir(), "rex.log"))
	useFakeRex(t, fakeRexWithApp)
	target := switchTarget{opener: "client:app", app: rex.AppClient{ID: "client:app"}, currentSessionID: "session:a"}
	recordSwitches(t, "id:client:app", [2]state.Visit{localVisit("session:a"), localVisit("session:b")})
	require.NoError(t, (&App{}).switchTo(t.Context(), target, model.Session{Source: "rex", Name: "C", SessionID: "session:c"}))
	assert.Equal(t, []state.Visit{localVisit("session:c"), localVisit("session:a"), localVisit("session:b")}, historyOf(t, "id:client:app"))
}

// Only the app that opened the picker is known to show the picker's session.
// An app named by --client alone may show another, so its switch records no
// origin, and a switch to the picker's own session is a switch like any other.
func TestSwitchWithoutAnOpenerRecordsOnlyTheDestination(t *testing.T) {
	useHistoryDir(t)
	t.Setenv("FAKE_REX_LOG", filepath.Join(t.TempDir(), "rex.log"))
	useFakeRex(t, fakeRexWithApp)
	ipad := switchTarget{client: "client:app", app: rex.AppClient{ID: "client:app"}, currentSessionID: "session:a"}
	require.NoError(t, (&App{}).switchTo(t.Context(), ipad, model.Session{Source: "rex", Name: "C", SessionID: "session:c"}))
	assert.Equal(t, []state.Visit{localVisit("session:c")}, historyOf(t, "id:client:app"))
	require.NoError(t, (&App{}).switchTo(t.Context(), ipad, model.Session{Source: "rex", Name: "A", SessionID: "session:a"}))
	assert.Equal(t, []state.Visit{localVisit("session:a"), localVisit("session:c")}, historyOf(t, "id:client:app"))

	// --client naming the opener changes nothing, even by its label.
	ipad.opener = "client:app"
	assert.Equal(t, "session:a", ipad.shownSessionID())
	mac := switchTarget{client: "Studio Mac", opener: "client:mac", currentSessionID: "session:a"}
	var err error
	mac.app, err = historyApp(t.Context(), &fakeApps{all: []string{"client:mac"}, labels: map[string]string{"client:mac": "Studio Mac"}}, mac)
	require.NoError(t, err)
	assert.Equal(t, "session:a", mac.shownSessionID())
	ipad.client, ipad.app = "client:other", rex.AppClient{ID: "client:other"}
	assert.Empty(t, ipad.shownSessionID())
}

// The picker shows an app it did not open from as showing no known session:
// the newest visit stands in for its current one.
func TestPickerHistoryForAnAppThatMayShowAnotherSession(t *testing.T) {
	useHistoryDir(t)
	recordSwitches(t, "label:mac",
		[2]state.Visit{localVisit("session:a"), localVisit("session:b")},
		[2]state.Visit{localVisit("session:b"), localVisit("session:c")})
	got := pickerHistory("label:mac", "", abc("A"))
	assert.Equal(t, "session:b", got.LastSessionID)
	assert.Equal(t, []string{"session:c", "session:b", "session:a"}, got.RecentSessionIDs)
}

// fakeRexHeldSwitch answers client ls with one app and holds its switch to
// session:b until $FAKE_RELEASE exists.
const fakeRexHeldSwitch = `printf '%s\n' "$*" >> "$FAKE_REX_LOG"
case "$*" in
  *"client ls"*) printf '%s' '{"clients":[{"client_id":"client:app","info":{"kind":"app","label":"Mac"}}]}' ;;
  *"session_id=session:b"*) while [ ! -e "$FAKE_RELEASE" ]; do sleep 0.01; done ;;
  *) ;;
esac`

// Two pickers switch one app, to B and then C, while the first is still
// waiting for the app. The history keeps C as the newest visit.
func TestSwitchesReachTheHistoryInTheOrderTheAppTookThem(t *testing.T) {
	useHistoryDir(t)
	dir := t.TempDir()
	log, release := filepath.Join(dir, "rex.log"), filepath.Join(dir, "release")
	t.Setenv("FAKE_REX_LOG", log)
	t.Setenv("FAKE_RELEASE", release)
	useFakeRex(t, fakeRexHeldSwitch)
	target := switchTarget{opener: "client:app", app: rex.AppClient{ID: "client:app", Label: "Mac"}, currentSessionID: "session:a"}
	toB := make(chan error, 1)
	go func() {
		toB <- (&App{}).switchTo(context.Background(), target, model.Session{Source: "rex", Name: "B", SessionID: "session:b"})
	}()
	require.Eventually(t, func() bool {
		//nolint:gosec // log is in this test's temporary directory.
		data, _ := os.ReadFile(log)
		return strings.Contains(string(data), "session_id=session:b")
	}, 10*time.Second, 10*time.Millisecond, "the switch to B never reached the app")

	toC := make(chan error, 1)
	go func() {
		toC <- (&App{}).switchTo(context.Background(), target, model.Session{Source: "rex", Name: "C", SessionID: "session:c"})
	}()
	// Without the lock the switch to C finishes first; give it the chance.
	select {
	case err := <-toC:
		toC <- err
	case <-time.After(300 * time.Millisecond):
	}
	require.NoError(t, os.WriteFile(release, nil, 0o600))
	require.NoError(t, <-toB)
	require.NoError(t, <-toC)
	assert.Equal(t, []state.Visit{localVisit("session:c"), localVisit("session:b"), localVisit("session:a")}, historyOf(t, "id:client:app"))
}

// A directory row records the session the switch created for it.
func TestSwitchRecordsTheSessionOpenedForADirectory(t *testing.T) {
	useHistoryDir(t)
	t.Setenv("FAKE_REX_LOG", filepath.Join(t.TempDir(), "rex.log"))
	useFakeRex(t, fakeRexNew)
	useFakeZoxide(t)
	row := model.Session{Source: "zoxide", Name: "api", Path: t.TempDir()}
	require.NoError(t, (&App{}).switchTo(t.Context(), switchTarget{opener: "client:app", app: rex.AppClient{ID: "client:app"}, currentSessionID: "session:1"}, row))
	assert.Equal(t, []state.Visit{localVisit("session:new"), localVisit("session:1")}, historyOf(t, "id:client:app"))
}

// fakeRexHostApp answers this machine's client ls with the app Mac and
// devbox's with one app labelled $FAKE_DEVBOX_APP, and accepts every switch.
const fakeRexHostApp = `case "$*" in
  *"-S devbox client ls"*) printf '{"clients":[{"client_id":"client:on-devbox","info":{"kind":"app","label":"%s"}}]}' "$FAKE_DEVBOX_APP" ;;
  *"client ls"*) printf '%s' '{"clients":[{"client_id":"client:mac","info":{"kind":"app","label":"Mac"}}]}' ;;
  *) ;;
esac`

// The app keeps its label on devbox, so a switch to a devbox session is the
// opener's, with the picker's session as its origin.
func TestSwitchRecordsAHostSessionByItsHost(t *testing.T) {
	useHistoryDir(t)
	useFakeRex(t, fakeRexHostApp)
	t.Setenv("FAKE_DEVBOX_APP", "Mac")
	mac := rex.AppClient{ID: "client:mac", Label: "Mac"}
	train := model.Session{Source: "rex", Machine: "devbox", Name: "train", SessionID: "session:9"}
	require.NoError(t, (&App{}).switchTo(t.Context(), switchTarget{opener: "client:mac", app: mac, currentSessionID: "session:1"}, train))
	assert.Equal(t, []state.Visit{{Host: "devbox", SessionID: "session:9"}, localVisit("session:1")}, historyOf(t, "id:client:mac"))
}

// When devbox's only app is another app, that app took the switch. It goes in
// that app's history, without the picker's session, which it never showed.
func TestSwitchRecordsTheAppThatTookTheSwitch(t *testing.T) {
	useHistoryDir(t)
	useFakeRex(t, strings.Replace(fakeRexHostApp, `"label":"Mac"}}]`, `"label":"Mac"}},{"client_id":"client:ipad","info":{"kind":"app","label":"iPad"}}]`, 1))
	t.Setenv("FAKE_DEVBOX_APP", "iPad")
	mac := rex.AppClient{ID: "client:mac", Label: "Mac"}
	train := model.Session{Source: "rex", Machine: "devbox", Name: "train", SessionID: "session:9"}
	require.NoError(t, (&App{}).switchTo(t.Context(), switchTarget{opener: "client:mac", app: mac, currentSessionID: "session:1"}, train))
	assert.Empty(t, historyOf(t, "id:client:mac"))
	assert.Equal(t, []state.Visit{{Host: "devbox", SessionID: "session:9"}}, historyOf(t, "id:client:ipad"))

	// An app without a label has no name here, so its switch is not recorded.
	t.Setenv("FAKE_DEVBOX_APP", "")
	require.NoError(t, (&App{}).switchTo(t.Context(), switchTarget{opener: "client:mac", app: mac, currentSessionID: "session:1"}, model.Session{Source: "rex", Machine: "devbox", Name: "lab", SessionID: "session:10"}))
	h, err := state.LoadHistory(stateDir())
	require.NoError(t, err)
	assert.Len(t, h.Apps, 1)
}

// A blacklisted session is hidden from the picker, but it still holds the
// picker, so it is still the switch's origin.
func TestSwitchFromABlacklistedSessionRecordsItsOrigin(t *testing.T) {
	useHistoryDir(t)
	t.Setenv("FAKE_REX_LOG", filepath.Join(t.TempDir(), "rex.log"))
	useFakeRex(t, fakeRexLabelled)
	configureFakeSources(t, "")
	cfgPath := writeConfig(t, "version = 1\n[list]\nblacklist = [\"^api$\"]\n")
	t.Setenv("FAKE_REX_SESSIONS", `{"sessions":[{"session_id":"session:1","label":"api","current":true},{"session_id":"session:2","label":"web"}]}`)
	useFakeFZF(t, "\tweb\t")
	var out bytes.Buffer
	require.NoError(t, (&App{Out: &out, Err: &bytes.Buffer{}}).Run(t.Context(), []string{"picker", "--fzf", "--opener", "client:mac", "--config", cfgPath}))
	assert.Equal(t, "web\n", out.String())
	assert.Equal(t, []state.Visit{localVisit("session:2"), localVisit("session:1")}, historyOf(t, "id:client:mac"))
}

// fakeRexLabelled answers client ls with one labelled app, lists
// $FAKE_REX_SESSIONS, and accepts every switch.
const fakeRexLabelled = `printf '%s\n' "$*" >> "$FAKE_REX_LOG"
case "$*" in
  *"client ls"*) printf '%s' '{"clients":[{"client_id":"client:mac","info":{"kind":"app","label":"Studio Mac"}}]}' ;;
  *"hosts --json"*) printf '%s' '{"hosts":[]}' ;;
  *"session.select"*) ;;
  *) printf '%s' "$FAKE_REX_SESSIONS" ;;
esac`

// useFakeFZF puts an fzf on PATH that picks the first row containing pick,
// or cancels, as Esc does, when pick is "".
func useFakeFZF(t *testing.T, pick string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		`[ -n "$FAKE_FZF_PICK" ] || exit 130` + "\n" +
		`while IFS= read -r line; do case "$line" in *"$FAKE_FZF_PICK"*) printf '%s\n' "$line"; exit 0 ;; esac; done` + "\n" +
		"exit 1\n"
	//nolint:gosec // the fake fzf must be executable.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fzf"), []byte(script), 0o700))
	t.Setenv("FAKE_FZF_PICK", pick)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// The fzf picker records the switch it makes, a cancelled picker records
// nothing, and the next picker shows the history.
func TestFZFPickerRecordsTheSwitchForTheNextPicker(t *testing.T) {
	useHistoryDir(t)
	t.Setenv("FAKE_REX_LOG", filepath.Join(t.TempDir(), "rex.log"))
	useFakeRex(t, fakeRexLabelled)
	configureFakeSources(t, "")
	cfgPath := writeConfig(t, "version = 1\n")
	t.Setenv("FAKE_REX_SESSIONS", `{"sessions":[{"session_id":"session:1","label":"api","current":true},{"session_id":"session:2","label":"web"}]}`)

	useFakeFZF(t, "")
	var out bytes.Buffer
	require.NoError(t, (&App{Out: &out, Err: &bytes.Buffer{}}).Run(t.Context(), []string{"picker", "--fzf", "--opener", "client:mac", "--config", cfgPath}))
	assert.Empty(t, out.String())
	assert.NoFileExists(t, state.HistoryPath(stateDir()), "a cancelled picker keeps the history")

	useFakeFZF(t, "\tweb\t")
	require.NoError(t, (&App{Out: &out, Err: &bytes.Buffer{}}).Run(t.Context(), []string{"picker", "--fzf", "--opener", "client:mac", "--config", cfgPath}))
	assert.Equal(t, "web\n", out.String())
	assert.Equal(t, []state.Visit{localVisit("session:2"), localVisit("session:1")}, historyOf(t, "id:client:mac"))

	// The app now shows web, where the next picker opens.
	t.Setenv("FAKE_REX_SESSIONS", `{"sessions":[{"session_id":"session:1","label":"api"},{"session_id":"session:2","label":"web","current":true}]}`)
	backend := &pickerBackend{app: &App{}, ctx: t.Context(), cfg: config.Default(), activeConfigPath: cfgPath, target: switchTarget{opener: "client:mac", currentSessionID: "session:2", app: rex.AppClient{ID: "client:mac", Label: "Studio Mac"}}}
	reloaded, err := backend.ReloadPicker(t.Context())
	require.NoError(t, err)
	assertHistoryResult(t, reloaded)
	_, reloaded, err = backend.ReloadSettings(t.Context(), settings.Result{Path: cfgPath})
	require.NoError(t, err)
	assertHistoryResult(t, reloaded)
}

// The second picker opens after the first recorded its switch. It still
// records the session it opened in, because no switch came between.
func TestFZFPickerKeepsItsOriginAfterAnEarlierSwitch(t *testing.T) {
	useHistoryDir(t)
	t.Setenv("FAKE_REX_LOG", filepath.Join(t.TempDir(), "rex.log"))
	useFakeRex(t, fakeRexLabelled)
	configureFakeSources(t, "")
	cfgPath := writeConfig(t, "version = 1\n")
	pick := func(current, row string) {
		t.Helper()
		sessions := `{"sessions":[{"session_id":"session:1","label":"api"},{"session_id":"session:2","label":"web"},{"session_id":"session:3","label":"ops"}]}`
		t.Setenv("FAKE_REX_SESSIONS", strings.Replace(sessions, `"label":"`+current+`"`, `"label":"`+current+`","current":true`, 1))
		useFakeFZF(t, "\t"+row+"\t")
		require.NoError(t, (&App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}).Run(t.Context(), []string{"picker", "--fzf", "--opener", "client:mac", "--config", cfgPath}))
	}
	pick("api", "web")
	pick("ops", "api")
	assert.Equal(t, []state.Visit{localVisit("session:1"), localVisit("session:3"), localVisit("session:2")}, historyOf(t, "id:client:mac"))
}

func assertHistoryResult(t *testing.T, got pickerpkg.ReloadResult) {
	t.Helper()
	assert.Equal(t, "session:1", got.LastSessionID)
	assert.Equal(t, []string{"session:2", "session:1"}, got.RecentSessionIDs)
	assert.Equal(t, []string{"api", "web"}, sessionNames(got.RexSessions))
	assert.Equal(t, []string{"api", "web"}, sessionNames(got.Sessions))
}

func TestSameLabelClientsKeepIndependentVisits(t *testing.T) {
	useHistoryDir(t)
	dir := t.TempDir()
	t.Setenv("FAKE_REX_LOG", filepath.Join(dir, "rex.log"))
	useFakeRex(t, strings.Replace(fakeRexHostApp, `"label":"Mac"}}]`, `"label":"Mac"}},{"client_id":"client:other","info":{"kind":"app","label":"Mac"}}]`, 1))
	t.Setenv("FAKE_DEVBOX_APP", "Mac")
	a := &App{}
	for _, pair := range [][2]string{{"client:mac", "session:b"}, {"client:other", "session:c"}} {
		target := switchTarget{opener: pair[0], currentSessionID: "session:a", app: rex.AppClient{ID: pair[0], Label: "Mac"}}
		require.NoError(t, a.switchTo(t.Context(), target, model.Session{Source: "rex", SessionID: pair[1]}))
		require.NoError(t, a.switchTo(t.Context(), target, model.Session{Source: "rex", Machine: "devbox", SessionID: "session:remote"}))
	}
	assert.Equal(t, []state.Visit{localVisit("session:b"), localVisit("session:a")}, historyOf(t, "id:client:mac"))
	assert.Equal(t, []state.Visit{localVisit("session:c"), localVisit("session:a")}, historyOf(t, "id:client:other"))
}

func TestSwitchPreservesAmbiguousSelectorAndVerifiedOpener(t *testing.T) {
	useHistoryDir(t)
	log := filepath.Join(t.TempDir(), "rex.log")
	t.Setenv("FAKE_REX_LOG", log)
	useFakeRex(t, `printf '%s\n' "$*" >> "$FAKE_REX_LOG"
case "$*" in
  *"client ls"*) [ -z "$FAIL_LIST" ] || exit 1
    printf '%s' '{"clients":[{"client_id":"client:a","info":{"kind":"app","label":"Mac"}},{"client_id":"client:b","info":{"kind":"app","label":"mac"}}]}' ;;
  *"-C Mac "*) echo "ambiguous" >&2; exit 3 ;;
esac`)
	a := &App{}
	row := model.Session{Source: "rex", SessionID: "session:b"}
	target := switchTarget{client: "Mac"}
	_, err := historyApp(t.Context(), rex.NewClient(), target)
	require.Error(t, err)
	require.ErrorContains(t, a.switchTo(t.Context(), target, row), "ambiguous")

	target = switchTarget{opener: "client:a", currentSessionID: "session:a"}
	target.app, err = historyApp(t.Context(), rex.NewClient(), target)
	require.NoError(t, err)
	t.Setenv("FAIL_LIST", "1")
	require.NoError(t, a.switchTo(t.Context(), target, row))
	assert.Equal(t, []state.Visit{localVisit("session:b"), localVisit("session:a")}, historyOf(t, "id:client:a"))
	assert.Empty(t, historyOf(t, "id:client:b"))
}

func TestSwitchByLabelPinsTheInspectedConnection(t *testing.T) {
	useHistoryDir(t)
	log := filepath.Join(t.TempDir(), "rex.log")
	t.Setenv("FAKE_REX_LOG", log)
	useFakeRex(t, `printf '%s\n' "$*" >> "$FAKE_REX_LOG"
case "$*" in
  *"client inspect --json -- Mac"*) printf '%s' '{"client":{"client_id":"client:old","info":{"kind":"app","label":"Mac"}}}' ;;
  *"-C client:old "*) echo "client disconnected" >&2; exit 3 ;;
  *"-C Mac "*) exit 0 ;; # the label now belongs to client:new
esac`)
	err := (&App{}).switchTo(t.Context(), switchTarget{client: "Mac"}, model.Session{Source: "rex", SessionID: "session:b"})
	require.ErrorContains(t, err, "client disconnected")
	assert.Equal(t, []string{"--autostart=false -C client:old do session.select session_id=session:b"}, selectCalls(t, log))
	assert.NoFileExists(t, state.HistoryPath(stateDir()))
}
