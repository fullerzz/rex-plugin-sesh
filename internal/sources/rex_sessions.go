package sources

import (
	"context"

	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/fullerzz/rex-plugin-sesh/internal/rex"
)

// SessionLister lists the sessions on a Rex server.
type SessionLister interface {
	Sessions(context.Context) ([]rex.Session, error)
}

// RexSessions lists open Rex sessions. Each one carries its session ID in
// WorkspaceID, which marks it as open.
type RexSessions struct{ Client SessionLister }

func (RexSessions) Name() string { return "rex" }

func (s RexSessions) List(ctx context.Context) (model.Sessions, error) {
	out := model.NewSessions()
	sessions, err := s.Client.Sessions(ctx)
	if err != nil {
		return out, err
	}
	for _, rs := range sessions {
		out.Add(model.Session{Source: "rex", Name: rs.Label, Path: rs.Path, WorkspaceID: rs.ID})
	}
	return out, nil
}
