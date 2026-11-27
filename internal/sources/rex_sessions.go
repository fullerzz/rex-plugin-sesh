package sources

import (
	"context"
	"sync"
	"time"

	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/fullerzz/rex-plugin-sesh/internal/rex"
)

// hostTimeout bounds listing the hosts and each host's sessions, so a host
// that is away delays the listing by at most this long.
const hostTimeout = 3 * time.Second

// SessionLister lists the sessions on a Rex server. selfBlockID, when set,
// marks the session that holds that block as current.
type SessionLister interface {
	Sessions(ctx context.Context, selfBlockID string) ([]rex.Session, error)
}

// RexHosts finds the other Rex servers this machine connects to.
type RexHosts interface {
	// Hosts lists the host labels in "rex hosts" order.
	Hosts(ctx context.Context) ([]string, error)
	// Host returns the lister for one host's server.
	Host(label string) SessionLister
}

// RexSessions lists open Rex sessions: this machine's first, then each
// host's, with Machine set to the host's label. Each one carries its session
// ID in SessionID, which marks it as open. With BlockID set, the session on
// this machine holding that block is Attached.
type RexSessions struct {
	Client  SessionLister
	BlockID string
	// Hosts, when set, adds the sessions of every host.
	Hosts RexHosts
	// HostErr, when set, receives each host that could not be listed, with
	// host "" when the host list itself failed. Those sessions are left out;
	// only a failure on this machine fails List.
	HostErr func(host string, err error)
	// HostListed receives each host whose session query succeeded, even if empty.
	HostListed func(host string)
}

func (RexSessions) Name() string { return "rex" }

func (s RexSessions) List(ctx context.Context) (model.Sessions, error) {
	out := model.NewSessions()
	var hosts []hostSessions
	done := make(chan struct{})
	go func() {
		defer close(done)
		hosts = s.listHosts(ctx)
	}()
	local, err := s.Client.Sessions(ctx, s.BlockID)
	<-done
	if err != nil {
		return out, err
	}
	addRexSessions(&out, "", local)
	for _, h := range hosts {
		if h.err != nil {
			s.hostErr(h.label, h.err)
			continue
		}
		if s.HostListed != nil {
			s.HostListed(h.label)
		}
		addRexSessions(&out, h.label, h.sessions)
	}
	return out, nil
}

type hostSessions struct {
	label    string
	sessions []rex.Session
	err      error
}

// listHosts lists every host's sessions at once, in host order.
func (s RexSessions) listHosts(ctx context.Context) []hostSessions {
	if s.Hosts == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, hostTimeout)
	defer cancel()
	labels, err := s.Hosts.Hosts(ctx)
	if err != nil {
		return []hostSessions{{err: err}}
	}
	out := make([]hostSessions, len(labels))
	var wg sync.WaitGroup
	for i, label := range labels {
		out[i].label = label
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i].sessions, out[i].err = s.Hosts.Host(label).Sessions(ctx, "")
		}()
	}
	wg.Wait()
	return out
}

func (s RexSessions) hostErr(host string, err error) {
	if s.HostErr != nil {
		s.HostErr(host, err)
	}
}

func addRexSessions(out *model.Sessions, machine string, sessions []rex.Session) {
	for _, rs := range sessions {
		out.Add(model.Session{
			Machine: machine, Source: "rex", Name: rs.Label, Path: rs.Path, SessionID: rs.ID, BlockID: rs.BlockID,
			AgentStatus: rs.AgentStatus, Attached: rs.Current,
		})
	}
}
