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

type switchingBackend struct {
	fakeBackend

	switched []string
	err      error
}

func (b *switchingBackend) Switch(_ context.Context, s model.Session) error {
	b.switched = append(b.switched, s.WorkspaceID)
	return b.err
}

func newSwitchingModel(backend *switchingBackend) teaModel {
	return newTeaModel([]model.Session{
		{Source: "rex", Name: "api", WorkspaceID: "session:1", Attached: true},
		{Source: "rex", Name: "web", WorkspaceID: "session:2"},
	}, Options{Backend: backend})
}

func press(m teaModel, key tea.KeyPressMsg) (teaModel, tea.Cmd) {
	updated, cmd := m.Update(key)
	return updated.(teaModel), cmd
}

func TestEnterQuitsOnlyAfterTheSwitch(t *testing.T) {
	backend := &switchingBackend{}
	m := newSwitchingModel(backend)
	m.list.Move(1)
	m, cmd := press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.False(t, m.chosen)
	assert.Contains(t, ansi.Strip(m.footerText()), "Switching to web…")

	updated, quit := m.Update(cmd())
	m = updated.(teaModel)
	assert.True(t, quits(quit))
	assert.True(t, m.chosen)
	assert.Equal(t, "web", m.choice.Name)
	assert.Equal(t, []string{"session:2"}, backend.switched)
}

// The ● row holds the picker's block, which need not be the session the app
// shows, so it switches like any other.
func TestEnterSwitchesToTheMarkedSessionToo(t *testing.T) {
	backend := &switchingBackend{}
	m, cmd := press(newSwitchingModel(backend), tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	m.Update(cmd())
	assert.Equal(t, []string{"session:1"}, backend.switched)
}

func TestFailedSwitchKeepsThePickerOpen(t *testing.T) {
	backend := &switchingBackend{err: errors.New("rex session.select: Remote Control is off")}
	m, cmd := press(newSwitchingModel(backend), tea.KeyPressMsg{Code: tea.KeyEnter})
	updated, next := m.Update(cmd())
	m = updated.(teaModel)
	assert.False(t, quits(next))
	assert.False(t, m.chosen)
	assert.Empty(t, m.switchingKey)
	assert.Equal(t, "Could not switch: rex session.select: Remote Control is off", m.closeError)

	// The picker can try again.
	_, retry := press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.NotNil(t, retry)
}

func TestFailedSwitchToastFitsAnEightyColumnPane(t *testing.T) {
	backend := &switchingBackend{err: errors.New("rex session.select: the app refused the action because Remote Control is turned off in its settings")}
	m := newSwitchingModel(backend)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(teaModel)
	m, cmd := press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	updated, _ = m.Update(cmd())
	// The toast wraps inside its border; join its words to read the message.
	view := strings.Join(strings.Fields(strings.ReplaceAll(ansi.Strip(settledView(updated.(teaModel))), "│", " ")), " ")
	assert.Contains(t, view, "Remote Control is turned off in its settings")
}

func TestEnterAndSettingsWaitForAPendingSwitch(t *testing.T) {
	backend := &switchingBackend{}
	m, first := press(newSwitchingModel(backend), tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, first)
	m, again := press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, quits(again))
	assert.True(t, m.shake.Animating(), "a second Enter is refused")
	m, _ = press(m, tea.KeyPressMsg{Code: tea.KeyF2})
	assert.Nil(t, m.settings, "settings stay closed while switching")
	assert.Empty(t, backend.switched, "only the first command switches, once it runs")
}

func TestStaleSwitchResultIsIgnored(t *testing.T) {
	m, _ := press(newSwitchingModel(&switchingBackend{}), tea.KeyPressMsg{Code: tea.KeyEnter})
	updated, cmd := m.Update(sessionSwitchedMsg{key: "rex:other"})
	m = updated.(teaModel)
	assert.False(t, quits(cmd))
	assert.False(t, m.chosen)
	assert.NotEmpty(t, m.switchingKey)
}
