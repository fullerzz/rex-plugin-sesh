package sources

import (
	"context"
	"errors"
	"testing"

	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/fullerzz/rex-plugin-sesh/internal/rex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRex struct {
	sessions []rex.Session
	err      error
	block    *string
}

func (f fakeRex) Sessions(_ context.Context, selfBlockID string) ([]rex.Session, error) {
	if f.block != nil {
		*f.block = selfBlockID
	}
	return f.sessions, f.err
}

func TestRexSessionsListsOpenSessions(t *testing.T) {
	var block string
	got, err := RexSessions{BlockID: "block:7", Client: fakeRex{block: &block, sessions: []rex.Session{
		{ID: "session:1", Label: "api", Path: "/src/api", Windows: 2, BlockID: "block:9", AgentStatus: "working"},
		{ID: "session:2", Label: "scratch", Current: true},
	}}}.List(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "block:7", block)
	assert.Equal(t, []model.Session{
		{Source: "rex", Name: "api", Path: "/src/api", SessionID: "session:1", BlockID: "block:9", AgentStatus: "working"},
		{Source: "rex", Name: "scratch", SessionID: "session:2", Attached: true},
	}, got.Ordered())
}

func TestRexSessionsReportsListError(t *testing.T) {
	_, err := RexSessions{Client: fakeRex{err: errors.New("no server")}}.List(context.Background())
	require.EqualError(t, err, "no server")
}

// Rex allows two sessions with one label; both must stay pickable, and a
// [[session]] entry with that label still merges into the first.
func TestMergeKeepsOpenSessionsThatShareALabel(t *testing.T) {
	got, err := Merge(context.Background(), []Source{
		RexSessions{Client: fakeRex{sessions: []rex.Session{
			{ID: "session:1", Label: "api", Path: "/src/api"},
			{ID: "session:2", Label: "api", Path: "/src/api-v2"},
		}}},
		staticSource{name: "config", sessions: []model.Session{{Source: "config", Name: "api", Path: "/src/api", PreviewCommand: "ls"}}},
	}, []string{"rex", "config"}, nil, false, true)
	require.NoError(t, err)
	items := got.Ordered()
	require.Len(t, items, 2)
	assert.Equal(t, "session:1", items[0].SessionID)
	assert.True(t, items[0].Configured)
	assert.Equal(t, "ls", items[0].PreviewCommand)
	assert.Equal(t, "session:2", items[1].SessionID)
	assert.False(t, items[1].Configured)
}
