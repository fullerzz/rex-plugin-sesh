package picker

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisableCloseRefusesCtrlXAndHidesIt(t *testing.T) {
	closed := ""
	m := newTeaModel([]model.Session{{Source: "rex", Name: "api", WorkspaceID: "session:1"}}, Options{
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
	assert.Empty(t, m.closingWorkspaceID)
	assert.Equal(t, []string{"api"}, sessionNames(m.list.All))
	assert.True(t, m.shake.Animating(), "the refused key shakes the picker")
	assert.NotContains(t, ansi.Strip(m.footerText()), "ctrl+x")
}

func TestCurrentSessionRowShowsMarker(t *testing.T) {
	current := ansi.Strip(row(model.Session{Source: "rex", Name: "api", WorkspaceID: "session:1", Attached: true}, false, 80, false, ""))
	other := ansi.Strip(row(model.Session{Source: "rex", Name: "web", WorkspaceID: "session:2"}, false, 80, false, ""))
	assert.Contains(t, current, "● [rex]")
	assert.NotContains(t, other, "●")
}

// Without history or agent statuses every sort mode keeps rex ls order.
func TestSortModesKeepRexOrderWithoutHistory(t *testing.T) {
	items := []model.Session{
		{Source: "rex", Name: "c", WorkspaceID: "session:3"},
		{Source: "rex", Name: "a", WorkspaceID: "session:1"},
		{Source: "rex", Name: "b", WorkspaceID: "session:2"},
	}
	for _, mode := range []string{workspaceSortWorkspace, workspaceSortRecent, workspaceSortAgent} {
		m := newTeaModel(items, Options{DisplayOptions: DisplayOptions{WorkspaceSort: mode}})
		require.Equal(t, []string{"c", "a", "b"}, sessionNames(m.list.All), mode)
		next, _ := m.toggleWorkspaceSort()
		assert.Equal(t, []string{"c", "a", "b"}, sessionNames(next.list.All), "after toggling from "+mode)
	}
}
