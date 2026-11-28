package picker

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisableCloseRefusesCtrlXAndHidesIt(t *testing.T) {
	closed := ""
	m := newTeaModel([]model.Session{{Source: "rex", Name: "api", SessionID: "session:1"}}, Options{
		DisplayOptions: DisplayOptions{DisableClose: true},
		Backend: &fakeBackend{closeWorkspace: func(_ context.Context, id string) error {
			closed = id
			return nil
		}},
	})
	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	m = updated.(teaModel)
	if cmd != nil {
		updated, _ = m.Update(cmd())
		m = updated.(teaModel)
	}
	assert.Empty(t, closed)
	assert.Empty(t, m.closingSessionID)
	assert.Equal(t, []string{"api"}, sessionNames(m.list.All))
	assert.True(t, m.shake.Animating(), "the refused key shakes the picker")
	assert.NotContains(t, ansi.Strip(m.footerText()), "ctrl+x")
}

func TestCurrentSessionRowShowsMarker(t *testing.T) {
	current := ansi.Strip(row(model.Session{Source: "rex", Name: "api", SessionID: "session:1", Attached: true}, false, 80, false, ""))
	other := ansi.Strip(row(model.Session{Source: "rex", Name: "web", SessionID: "session:2"}, false, 80, false, ""))
	assert.True(t, strings.HasPrefix(current, "◆   [rex]"), current)
	assert.NotContains(t, other, "◆")
}

// The marker sits in the rail, so an agent's status glyph still shows.
func TestCurrentMarkerKeepsTheAgentStatus(t *testing.T) {
	got := ansi.Strip(row(model.Session{Source: "rex", Name: "api", SessionID: "session:1", Attached: true, AgentStatus: "done"}, false, 80, false, ""))
	assert.True(t, strings.HasPrefix(got, "◆ ● [rex]"), got)
	selected := ansi.Strip(row(model.Session{Source: "rex", Name: "api", SessionID: "session:1", Attached: true}, true, 80, false, ""))
	assert.True(t, strings.HasPrefix(selected, "┃"), "the selection rail wins on the selected row")
}

// Without history or agent statuses every sort mode keeps rex ls order.
func TestSortModesKeepRexOrderWithoutHistory(t *testing.T) {
	items := []model.Session{
		{Source: "rex", Name: "c", SessionID: "session:3"},
		{Source: "rex", Name: "a", SessionID: "session:1"},
		{Source: "rex", Name: "b", SessionID: "session:2"},
	}
	for _, mode := range []string{workspaceSortWorkspace, workspaceSortRecent, workspaceSortAgent} {
		m := newTeaModel(items, Options{DisplayOptions: DisplayOptions{WorkspaceSort: mode}})
		require.Equal(t, []string{"c", "a", "b"}, sessionNames(m.list.All), mode)
		next, _ := m.toggleWorkspaceSort()
		assert.Equal(t, []string{"c", "a", "b"}, sessionNames(next.list.All), "after toggling from "+mode)
	}
}

func TestRemoteDiscoveryPreservesFilteredSelection(t *testing.T) {
	local := model.Session{Source: "rex", SessionID: "local", Name: "api-local"}
	remote := model.Session{Source: "rex", Machine: "build", SessionID: "remote", Name: "api-remote"}
	load := func() (ReloadResult, error) {
		return ReloadResult{Sessions: []model.Session{remote, local}}, nil
	}
	m := newTeaModel([]model.Session{local}, Options{LoadSessions: load})
	assert.Regexp(t, "╰─+ ∙∙∙ Resolving hosts ─*╯", ansi.Strip(m.View().Content))
	m.list.Filter("api")
	result, err := load()
	require.NoError(t, err)
	updated, _ := m.Update(sessionsLoadedMsg{result: result})
	assert.NotContains(t, ansi.Strip(updated.View().Content), "Resolving hosts")
	list := updated.(teaModel).list
	require.Len(t, list.Filtered, 2)
	selected, ok := list.Current()
	require.True(t, ok)
	assert.Equal(t, local, selected)
	assert.Equal(t, "api", list.Query)
}

func TestDiscoveryStatusInShortPane(t *testing.T) {
	m := newTeaModel(nil, Options{LoadSessions: func() (ReloadResult, error) { return ReloadResult{}, nil }})
	m.width, m.height = 80, 8
	require.Less(t, m.bodyRows(), panelFrameRows+1)
	assert.Contains(t, ansi.Strip(m.View().Content), "∙∙∙ Resolving hosts")
}

func TestDiscoveryFailureKeepsLocalRows(t *testing.T) {
	local := []model.Session{{Source: "rex", Name: "api", SessionID: "local"}}
	load := func() (ReloadResult, error) { return ReloadResult{}, errors.New("discovery failed") }
	m := newTeaModel(local, Options{LoadSessions: load})
	result, err := load()
	next, _ := m.Update(sessionsLoadedMsg{result: result, err: err})
	m = next.(teaModel)
	assert.Equal(t, local, m.list.All)
	assert.NotContains(t, ansi.Strip(m.View().Content), "Resolving hosts")
	assert.Equal(t, "discovery failed", m.toastText)
}

func TestDiscoverySpinnerTicks(t *testing.T) {
	t.Setenv("REX_SESH_REDUCE_MOTION", "0")
	m := newTeaModel(nil, Options{LoadSessions: func() (ReloadResult, error) { return ReloadResult{}, nil }})
	frame, agent := m.discoverySpinner.View(), m.agentSpinner.View()
	tick := m.discoverySpinner.Tick()
	m.reduceMotion = true
	still, cmd := m.update(tick)
	assert.Nil(t, cmd)
	assert.Equal(t, frame, still.(teaModel).discoverySpinner.View())
	m.reduceMotion = false
	next, cmd := m.update(tick)
	require.NotNil(t, cmd)
	m = next.(teaModel)
	assert.NotEqual(t, frame, m.discoverySpinner.View())
	assert.Equal(t, agent, m.agentSpinner.View())
	done, _ := m.update(sessionsLoadedMsg{})
	m = done.(teaModel)
	_, cmd = m.update(m.discoverySpinner.Tick())
	assert.Nil(t, cmd)
}
