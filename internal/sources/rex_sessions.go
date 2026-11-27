package sources

import (
	"context"

	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/fullerzz/rex-plugin-sesh/internal/rex"
)

// SessionLister lists the sessions on a Rex server. selfBlockID, when set,
// marks the session that holds that block as current.
type SessionLister interface {
	Sessions(ctx context.Context, selfBlockID string) ([]rex.Session, error)
}

// RexSessions lists open Rex sessions. Each one carries its session ID in
// SessionID, which marks it as open. With BlockID set, the session holding
// that block is Attached.
type RexSessions struct {
	Client  SessionLister
	BlockID string
}

func (RexSessions) Name() string { return "rex" }

func (s RexSessions) List(ctx context.Context) (model.Sessions, error) {
	out := model.NewSessions()
	sessions, err := s.Client.Sessions(ctx, s.BlockID)
	if err != nil {
		return out, err
	}
	for _, rs := range sessions {
		out.Add(model.Session{
			Source: "rex", Name: rs.Label, Path: rs.Path, SessionID: rs.ID, BlockID: rs.BlockID,
			AgentStatus: rs.AgentStatus, Attached: rs.Current,
		})
	}
	return out, nil
}
