package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fullerzz/rex-plugin-sesh/internal/config"
	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/fullerzz/rex-plugin-sesh/internal/namer"
	pickerpkg "github.com/fullerzz/rex-plugin-sesh/internal/picker"
	"github.com/fullerzz/rex-plugin-sesh/internal/preview"
	"github.com/fullerzz/rex-plugin-sesh/internal/rex"
	"github.com/fullerzz/rex-plugin-sesh/internal/sources"
	"github.com/fullerzz/rex-plugin-sesh/internal/state"
)

var Version = "dev"

type App struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
	// Rex lists Rex sessions; nil uses the rex CLI.
	Rex sources.SessionLister
	// RexHosts finds other Rex hosts. When both it and Rex are nil, it uses
	// the rex CLI; a test that sets only Rex lists no hosts.
	RexHosts sources.RexHosts

	// opened maps a directory to the Rex session a switch created for it, so
	// a retry after a refused switch selects that session instead of creating
	// another.
	openedMu sync.Mutex
	opened   map[string]string
}

func New() *App { return &App{In: os.Stdin, Out: os.Stdout, Err: os.Stderr} }
func (a *App) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return a.usage()
	}
	switch args[0] {
	case "--version", "version":
		_, err := fmt.Fprintf(a.Out, "rex-sesh %s\n", Version)
		return err
	case "list":
		return a.list(ctx, args[1:])
	case "preview":
		return a.preview(ctx, args[1:])
	case "root":
		return a.root(ctx, args[1:])
	case "config":
		return a.config(ctx, args[1:])
	case "picker":
		return a.picker(ctx, args[1:])
	case "last":
		return a.last(ctx, args[1:])
	case "lua":
		return a.lua(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
func (a *App) usage() error {
	_, err := fmt.Fprintln(a.Out, "rex-sesh list|preview|root|picker|last|lua|config|--version")
	return err
}

func (a *App) warnf(format string, args ...any) {
	if a.Err == nil {
		return
	}
	_, _ = fmt.Fprintf(a.Err, "warning: "+format+"\n", args...)
}

func sameFile(first, second string) bool {
	//nolint:gosec // config paths are user-selected by design.
	firstInfo, firstErr := os.Stat(first)
	if firstErr != nil {
		return false
	}
	//nolint:gosec // config paths are user-selected by design.
	secondInfo, secondErr := os.Stat(second)
	return secondErr == nil && os.SameFile(firstInfo, secondInfo)
}

func (a *App) loadConfig(path string) (config.Config, error) {
	cfg, _, err := config.Load(config.LoadOptions{Path: path, Warn: a.Err})
	return cfg, err
}

// capturingSource records a source's error and lists nothing in its place,
// so one unavailable source does not fail the merge.
type capturingSource struct {
	sources.Source

	err error
	// hostWarnings words each host the Rex source could not list.
	hostWarnings []string
	listedHosts  map[string]bool
	// current is the session on this machine that holds the picker, found
	// before the merge's blacklist can hide it.
	current string
	rows    []model.Session
}

func (s *capturingSource) hostFailed(host string, err error) {
	if host == "" {
		s.hostWarnings = append(s.hostWarnings, fmt.Sprintf("rex hosts unavailable: %v", err))
		return
	}
	s.hostWarnings = append(s.hostWarnings, fmt.Sprintf("rex host %s unavailable: %v", host, err))
}

func (s *capturingSource) List(ctx context.Context) (model.Sessions, error) {
	ss, err := s.Source.List(ctx)
	if err != nil {
		s.err = err
		return model.NewSessions(), nil
	}
	s.rows = ss.Ordered()
	for _, sess := range s.rows {
		if sess.Attached && sess.Machine == "" {
			s.current = sess.SessionID
			break
		}
	}
	return ss, nil
}

// rexSource lists Rex sessions on this machine, then on each host. With
// blockID set, the session holding that block is marked current; only the
// picker asks, because list caches its output for every session to read.
func (a *App) rexSource(blockID string) *capturingSource {
	client, hosts := a.Rex, a.RexHosts
	if client == nil {
		client = rex.NewClient()
		if hosts == nil {
			hosts = cliHosts{}
		}
	}
	src := &capturingSource{listedHosts: make(map[string]bool)}
	src.Source = sources.RexSessions{Client: client, BlockID: blockID, Hosts: hosts, HostErr: src.hostFailed, HostListed: func(host string) { src.listedHosts[host] = true }}
	return src
}

// cliHosts finds Rex hosts with the rex CLI.
type cliHosts struct{}

func (cliHosts) Hosts(ctx context.Context) ([]string, error) { return rex.NewClient().Hosts(ctx) }
func (cliHosts) Host(label string) sources.SessionLister { //nolint:ireturn // RexHosts contract.
	return rex.NewHostClient(label)
}

// collection is a merged session list. RexErr says why Rex sessions are
// missing from it, and HostWarnings which hosts' sessions are; callers report
// them. ListedHosts includes hosts that successfully returned zero sessions.
// CurrentSessionID is the session that holds the picker,
// even when the blacklist leaves it out of Sessions.
type collection struct {
	// Open Rex sessions before the picker applies its row filters.
	RexSessions      []model.Session
	Sessions         []model.Session
	RexErr           error
	HostWarnings     []string
	ListedHosts      map[string]bool
	CurrentSessionID string
}

// collect merges Rex, config, and zoxide sessions. localOnly drops
// [[workspace]] entries on another machine: list and the picker cannot open
// them yet.
func (a *App) collect(ctx context.Context, cfg config.Config, target string, localOnly bool) (collection, error) {
	rexSrc := a.rexSource("")
	srcs := []sources.Source{rexSrc, sources.ConfigSessions{Config: cfg, LocalOnly: localOnly}, sources.Zoxide{}}
	if target != "" {
		direct := sources.DirectPath{Path: target}
		// Naming runs git in the target, so only a local directory is named:
		// the target may be the name of a workspace on another machine.
		if st, err := os.Stat(target); err == nil && st.IsDir() {
			direct.Label = namer.Namer{}.Name(ctx, target, cfg.DirLength)
		}
		srcs = append(srcs, direct)
	}
	merged, err := sources.Merge(ctx, srcs, config.MergeOrder(cfg.SortOrder), cfg.Blacklist, false, true)
	if err != nil {
		return collection{}, err
	}
	sources.ApplyConfig(&merged, cfg, "")
	return collection{RexSessions: rexSrc.rows, Sessions: merged.Ordered(), RexErr: rexSrc.err, HostWarnings: rexSrc.hostWarnings, ListedHosts: rexSrc.listedHosts, CurrentSessionID: rexSrc.current}, nil
}

// collectPicker lists the picker's rows: the open Rex sessions, with the one
// that holds this terminal's block marked current, and zoxide directories, in
// source_order. A directory named like an open session is left out.
func (a *App) collectPicker(ctx context.Context, cfg config.Config) (collection, error) {
	rexSrc := a.rexSource(os.Getenv("REX_BLOCK"))
	merged, err := sources.Merge(ctx, []sources.Source{rexSrc, sources.Zoxide{}}, config.MergeOrder(cfg.SortOrder), cfg.Blacklist, false, true)
	if err != nil {
		return collection{}, err
	}
	sources.ApplyConfig(&merged, cfg, "")
	return collection{RexSessions: rexSrc.rows, Sessions: merged.Ordered(), RexErr: rexSrc.err, HostWarnings: rexSrc.hostWarnings, ListedHosts: rexSrc.listedHosts, CurrentSessionID: rexSrc.current}, nil
}

// rexWarning words an unavailable Rex server for the user, or "" when Rex
// answered.
func rexWarning(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("rex sessions unavailable: %v", err)
}

func (a *App) list(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	jsonOut := fs.Bool("json", false, "")
	cfgPath := fs.String("config", "", "")
	blacklisted := fs.Bool("blacklisted", false, "")
	hideDup := fs.Bool("hide-duplicates", true, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, resolvedConfigPath, err := config.Load(config.LoadOptions{Path: *cfgPath, Warn: a.Err})
	if err != nil {
		return err
	}
	cacheable := cfg.Cache && !*blacklisted && *hideDup
	if cacheable {
		if cached, ok, err := state.LoadSessionCache(stateDir(), resolvedConfigPath, 5*time.Second, time.Now()); err != nil {
			a.warnf("ignoring session cache: %v", err)
		} else if ok {
			return a.printSessions(cached, *jsonOut)
		}
	}
	rexSrc := a.rexSource("")
	ss, err := sources.Merge(ctx, []sources.Source{rexSrc, sources.ConfigSessions{Config: cfg, LocalOnly: true}, sources.Zoxide{}}, config.MergeOrder(cfg.SortOrder), cfg.Blacklist, *blacklisted, *hideDup)
	if err != nil {
		return err
	}
	sources.ApplyConfig(&ss, cfg, "")
	sessions := ss.Ordered()
	if warning := rexWarning(rexSrc.err); warning != "" {
		a.warnf("%s", warning)
		// A cache without Rex sessions would hide them after the server returns.
		cacheable = false
	}
	for _, warning := range rexSrc.hostWarnings {
		a.warnf("%s", warning)
		cacheable = false
	}
	if cacheable {
		if err := state.SaveSessionCache(stateDir(), resolvedConfigPath, sessions, time.Now()); err != nil {
			a.warnf("could not save session cache: %v", err)
		}
	}
	return a.printSessions(sessions, *jsonOut)
}

func pickerOptionsFromConfig(ctx context.Context, output io.Writer, cfg config.Config) pickerpkg.DisplayOptions {
	return pickerpkg.DisplayOptions{
		Context:                        ctx,
		Output:                         output,
		Prompt:                         cfg.TUI.Prompt,
		Placeholder:                    cfg.TUI.Placeholder,
		ShowIcons:                      cfg.TUI.ShowIcons,
		HerdrThemeInherit:              cfg.TUI.HerdrThemeInherit,
		DisableWorktreeIconReplacement: !cfg.TUI.ReplaceWorktreeIcon,
		HideLastWorkspacePath:          !cfg.TUI.ShowLastWorkspacePath,
		SeparatorAware:                 cfg.SeparatorAware,
		DisableHomePrioritization:      !cfg.TUI.PrioritizeHome,
		HidePath:                       !cfg.TUI.ShowPath,
		HidePreview:                    !cfg.TUI.ShowPreview,
		PreviewMode:                    cfg.TUI.PreviewMode,
		CyclePreviewModeKey:            &cfg.Keys.CyclePreviewMode,
		DefaultPreviewCommand:          cfg.DefaultSessionConfig.PreviewCommand,
		WorkspaceSort:                  cfg.TUI.DefaultSort,
		HideLastWorkspace:              !cfg.TUI.ShowLastWorkspace,
		LastWorkspaceLabel:             &cfg.History.LastWorkspaceLabel,
		DisableClose:                   true,
	}
}

func (a *App) printSessions(sessions []model.Session, jsonOut bool) error {
	if jsonOut {
		enc := json.NewEncoder(a.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(sessions)
	}
	for _, s := range sessions {
		var err error
		name := s.Name
		if s.Machine != "" {
			name = s.DisplayName()
		}
		if s.Path != "" {
			_, err = fmt.Fprintf(a.Out, "%s	%s	%s\n", s.Source, name, s.Path)
		} else {
			_, err = fmt.Fprintf(a.Out, "%s	%s\n", s.Source, name)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// picker lists the open Rex sessions, switches the Rex app to the chosen one,
// and prints its name. --client names the app; see resolveClient.
func (a *App) picker(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("picker", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	jsonOut := fs.Bool("json", false, "")
	cfgPath := fs.String("config", "", "")
	fzfPicker := fs.Bool("fzf", false, "")
	clientID := fs.String("client", "", "the Rex app to switch, as a client ID or label")
	openerID := fs.String("opener", "", "the Rex app that opened the picker, as a client ID")
	waitOnError := fs.Bool("wait-on-error", false, "after an error, wait for Enter before exiting")
	if err := fs.Parse(args); err != nil {
		return err
	}
	err := a.runPicker(ctx, *cfgPath, *jsonOut, *fzfPicker, switchTarget{client: *clientID, opener: *openerID})
	if err != nil && *waitOnError {
		a.waitForEnter(err)
	}
	return err
}

// waitForEnter shows err until Enter. A picker in a Rex floating layer would
// otherwise exit, and the layer would close before the error could be read.
func (a *App) waitForEnter(err error) {
	if a.In == nil {
		return
	}
	_, _ = fmt.Fprintf(a.Err, "rex-sesh: %v\n\nPress Enter to close.\n", err)
	_, _ = bufio.NewReader(a.In).ReadString('\n')
}

func (a *App) runPicker(ctx context.Context, cfgPath string, jsonOut, fzfPicker bool, target switchTarget) error {
	cfg, activeConfigPath, err := config.Load(config.LoadOptions{Path: cfgPath, Warn: a.Err})
	if err != nil {
		return err
	}
	col, err := a.collectPicker(ctx, cfg)
	if err != nil {
		return err
	}
	// Without Rex the picker would open empty, so say why instead.
	if col.RexErr != nil {
		return fmt.Errorf("list Rex sessions: %w", col.RexErr)
	}
	if jsonOut {
		for _, warning := range col.HostWarnings {
			a.warnf("%s", warning)
		}
		return a.printSessions(col.Sessions, true)
	}
	target.currentSessionID = col.CurrentSessionID
	// Without a known app the picker still opens, with no history.
	target.app, _ = historyApp(ctx, rex.NewClient(), target)
	backend := &pickerBackend{app: a, ctx: ctx, cfg: cfg, activeConfigPath: activeConfigPath, target: target}
	opts := backend.withHistory(pickerOptionsFromConfig(ctx, a.Out, cfg), col)
	var selected model.Session
	var ok bool
	if fzfPicker || strings.EqualFold(os.Getenv("REX_SESH_PICKER"), "fzf") {
		selected, ok, err = pickerpkg.RunFZF(ctx, col.Sessions, pickerpkg.Options{DisplayOptions: opts})
		if err == nil && ok {
			err = a.switchTo(ctx, target, selected)
		}
	} else {
		// The native picker switches through the backend before it exits.
		selected, ok, err = pickerpkg.Run(col.Sessions, pickerpkg.Options{DisplayOptions: opts, Backend: backend})
	}
	if err != nil || !ok {
		return err
	}
	_, err = fmt.Fprintln(a.Out, selected.Name)
	return err
}

// switchTarget says which Rex app a picker switches.
type switchTarget struct {
	// client is the --client flag, or "".
	client string
	// opener is the --opener flag, or "": the app that opened the picker,
	// which the Lua module passes for a key or the command palette. That app
	// shows the session that holds the picker.
	opener string
	// currentSessionID is the session that holds the picker, or "".
	currentSessionID string
	// app is the app to switch, as historyApp finds it, or the zero
	// AppClient when it is unknown.
	app rex.AppClient
}

// shownSessionID is the session that the app to switch shows, or "" when
// that is unknown. Only the app that opened the picker is known to show the
// picker's session; any other app may show another. --client may name the
// opener by its label, so the resolved app's ID is compared.
func (t switchTarget) shownSessionID() string {
	if t.opener == "" || t.app.ID != t.opener {
		return ""
	}
	return t.currentSessionID
}

// switchTo asks the Rex app to show s. A directory row, such as a zoxide
// entry, first gets a new Rex session that starts there, and the directory is
// added to zoxide once the app shows it. Only a switch the app accepts is
// recorded in its history.
func (a *App) switchTo(ctx context.Context, target switchTarget, s model.Session) error {
	sessionID := s.SessionID
	if s.Source == "rex" {
		if sessionID == "" {
			return fmt.Errorf("%q is not an open Rex session", s.DisplayName())
		}
	} else if s.Path == "" || s.Machine != "" {
		return fmt.Errorf("%q is not a Rex session or a directory on this machine", s.DisplayName())
	}
	// The app showed the picker's session when Enter was pressed, so the
	// history then is the one an intervening switch would change.
	before, _ := state.LoadHistory(stateDir())
	client := rex.NewClient()
	var app rex.AppClient
	var localApp rex.AppClient
	var err error
	if s.Machine != "" {
		// A host's session is found on the host's own server.
		host := rex.NewHostClient(s.Machine)
		app, err = resolveHostClient(ctx, client, host, s.Machine, target)
		if err == nil && app.Label != "" {
			if all, listErr := client.AppClients(ctx, ""); listErr == nil {
				localApp, _ = uniqueLabel(all, app.Label)
			}
		}
		client = host
	} else {
		// Find the app before creating anything, so a failure leaves no session.
		var listed bool
		app, listed, err = resolveApp(ctx, client, target)
		switch {
		case listed:
			localApp = app
		case app.ID != "" && app.ID == target.app.ID:
			// The picker already verified this exact ID when it opened.
			localApp = target.app
		case err == nil:
			// Rex resolves labels, positions and ID suffixes. Pin the
			// accepted target and its history lock to the same connection.
			localApp, _ = client.InspectApp(ctx, app.ID)
			if localApp.ID != "" {
				app = localApp
			}
		}
	}
	if err != nil {
		return err
	}
	if s.Source != "rex" {
		if sessionID, err = a.openDirectory(ctx, client, s); err != nil {
			return err
		}
	}
	key := historyKey(localApp)
	var from state.Visit
	if localApp.ID != "" && localApp.ID == target.app.ID {
		from.SessionID = target.shownSessionID()
	}
	// Holding the lock from the switch until it is recorded keeps two
	// pickers that switch one app from recording in the wrong order.
	unlock := a.lockSwitches(key)
	err = client.SelectSession(ctx, app.ID, sessionID)
	if err == nil {
		a.recordSwitch(key, from, state.Visit{Host: s.Machine, SessionID: sessionID}, before.Revision(key))
	}
	unlock()
	if err != nil {
		return err
	}
	if s.Source != "rex" {
		sources.AddPath(ctx, s.Path)
	}
	return nil
}

// openDirectory returns the Rex session for the directory row s, creating it
// on first use. The session takes the row's name, so the next listing shows
// the session in place of the row.
func (a *App) openDirectory(ctx context.Context, client *rex.Client, s model.Session) (string, error) {
	a.openedMu.Lock()
	defer a.openedMu.Unlock()
	if id, ok := a.opened[s.Path]; ok {
		return id, nil
	}
	//nolint:gosec // the path comes from the user's own zoxide database.
	if st, err := os.Stat(s.Path); err != nil || !st.IsDir() {
		return "", fmt.Errorf("%s is not a directory on this machine", s.Path)
	}
	id, err := client.NewSession(ctx, s.Name, s.Path)
	if err != nil {
		return "", err
	}
	if a.opened == nil {
		a.opened = map[string]string{}
	}
	a.opened[s.Path] = id
	return id, nil
}

// appLister lists connected Rex apps; *rex.Client implements it.
type appLister interface {
	InspectApp(ctx context.Context, selector string) (rex.AppClient, error)
	AppClients(ctx context.Context, sessionID string) ([]rex.AppClient, error)
}

// resolveClient picks the Rex app to switch: --client, then --opener, then
// REX_CLIENT, then the only connected app. With several apps, the only one attached to the
// picker's session wins. Rex apps attach to many sessions, so that rarely
// narrows the choice; otherwise the user names one with --client.
func resolveClient(ctx context.Context, apps appLister, target switchTarget) (string, error) {
	app, _, err := resolveApp(ctx, apps, target)
	return app.ID, err
}

// resolveApp is resolveClient returning the whole app. An app named by a
// flag or REX_CLIENT has only its ID set, which may be a label. listed says
// that a listing found the app instead, so it has its client ID and label.
func resolveApp(ctx context.Context, apps appLister, target switchTarget) (app rex.AppClient, listed bool, err error) {
	if target.client != "" {
		return rex.AppClient{ID: target.client}, false, nil
	}
	if target.opener != "" {
		return rex.AppClient{ID: target.opener}, false, nil
	}
	if id := os.Getenv("REX_CLIENT"); id != "" {
		return rex.AppClient{ID: id}, false, nil
	}
	all, err := apps.AppClients(ctx, "")
	if err != nil {
		return rex.AppClient{}, false, err
	}
	if len(all) == 1 {
		return all[0], true, nil
	}
	if len(all) == 0 {
		return rex.AppClient{}, false, errors.New("no Rex app is connected to switch")
	}
	if target.currentSessionID != "" {
		attached, err := apps.AppClients(ctx, target.currentSessionID)
		if err != nil {
			return rex.AppClient{}, false, err
		}
		if len(attached) == 1 {
			return attached[0], true, nil
		}
	}
	return rex.AppClient{}, false, fmt.Errorf("%d Rex apps are connected; choose one with --client", len(all))
}

// resolveHostClient picks the Rex app to switch to a session on host. The app
// has another client ID on each server, so --client and REX_CLIENT name it
// only here. The only app connected to the host wins. With several, the one
// whose device label matches the app resolveClient picks here wins.
func resolveHostClient(ctx context.Context, local, remote appLister, host string, target switchTarget) (rex.AppClient, error) {
	apps, err := remote.AppClients(ctx, "")
	if err != nil {
		return rex.AppClient{}, err
	}
	switch len(apps) {
	case 0:
		return rex.AppClient{}, fmt.Errorf("no Rex app is connected to %s", host)
	case 1:
		return apps[0], nil
	}
	localID, err := resolveClient(ctx, local, target)
	if err != nil {
		return rex.AppClient{}, err
	}
	localApps, err := local.AppClients(ctx, "")
	if err != nil {
		return rex.AppClient{}, err
	}
	// --client may already be a label.
	label := localID
	for _, app := range localApps {
		if app.ID == localID {
			label = app.Label
			break
		}
	}
	if _, ok := uniqueLabel(localApps, label); ok {
		if match, ok := uniqueLabel(apps, label); ok {
			return match, nil
		}
	}
	return rex.AppClient{}, fmt.Errorf("%d Rex apps are connected to %s, and none is this machine's app", len(apps), host)
}

func (a *App) preview(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	cfgPath := fs.String("config", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errors.New("preview requires target")
	}
	target := fs.Arg(0)
	cfg, err := a.loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	// A preview without Rex sessions still previews config and directories.
	col, err := a.collect(ctx, cfg, target, false)
	if err != nil {
		return err
	}
	s, dir, ok := resolveTarget(col.Sessions, target)
	if dir || !ok {
		s = model.Session{Name: filepath.Base(target), Path: target}
	}
	out, err := preview.Render(ctx, s, cfg.DefaultSessionConfig.PreviewCommand)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(a.Out, out)
	return err
}
func (a *App) root(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("root", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, err := gitRoot(ctx, ".")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(a.Out, root)
	return err
}
func gitRoot(ctx context.Context, dir string) (string, error) {
	//nolint:gosec // dir is passed as an argv value to a fixed git command.
	b, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// stateDir holds rex-sesh's session cache and history: REX_SESH_STATE_DIR,
// or rex-sesh under the XDG state home. It is "" when there is nowhere to
// keep them, which disables both.
func stateDir() string {
	if dir := os.Getenv("REX_SESH_STATE_DIR"); dir != "" {
		return dir
	}
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "rex-sesh")
}

// resolveTarget finds the session a preview target names, in this order: an
// exact workspace ID on any machine; a local session by configured root; any
// local session by name or path; a local directory at the target; the
// [[workspace]] entry on another machine with that name. For a bare local
// directory dir is true and only Path is set: the caller names it.
func resolveTarget(candidates []model.Session, target string) (s model.Session, dir, ok bool) {
	for _, c := range candidates {
		if c.SessionID != "" && c.SessionID == target {
			return c, false, true
		}
	}
	for _, c := range candidates {
		if c.Machine == "" && c.ConfiguredPath != "" && c.ConfiguredPath == target {
			return c, false, true
		}
	}
	// Remote sessions resolve only by qualified ID here, so a local name or
	// path never selects a session on another machine.
	for _, c := range candidates {
		if c.Machine == "" && (c.Name == target || c.Path == target) {
			return c, false, true
		}
	}
	if st, err := os.Stat(target); err == nil && st.IsDir() {
		return model.Session{Path: target}, true, true
	}
	// A configured remote workspace's path is a path on that machine, so a
	// target never matches it: a local directory with the same path would
	// otherwise resolve to the remote entry.
	for _, c := range candidates {
		if c.Machine != "" && (c.Source == "config" || c.Configured) && c.Name == target {
			return c, false, true
		}
	}
	return model.Session{}, false, false
}

func (a *App) config(ctx context.Context, args []string) error { //nolint:gocyclo // Routes config subcommands; implementations are kept separate.
	if len(args) == 0 {
		return errors.New("config requires path, init, validate, migrate, or edit")
	}
	dir := config.PluginConfigDir(config.LoadOptions{})
	switch args[0] {
	case "edit":
		return a.editSettings(ctx, args[1:])
	case "path":
		p, err := config.ResolvePath(config.LoadOptions{})
		if err != nil {
			return err
		}
		if p == "" {
			p = filepath.Join(dir, config.NativeFileName)
		}
		_, err = fmt.Fprintln(a.Out, p)
		return err
	case "init":
		// Never shadow a config that already loads: an existing candidate
		// anywhere in the lookup order is returned instead of creating a
		// higher-precedence native file next to it.
		p, err := config.ResolvePath(config.LoadOptions{})
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil && p != "" {
			_, err = fmt.Fprintln(a.Out, p)
			return err
		}
		if target := os.Getenv("REX_SESH_CONFIG"); target != "" {
			p, err := config.InitConfigAt(config.ExpandHome(target, ""))
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(a.Out, p)
			return err
		}
		p, err = config.InitConfig(dir)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(a.Out, p)
		return err
	case "validate":
		return a.validateConfig(args[1:])
	case "migrate":
		fs := flag.NewFlagSet("config migrate", flag.ContinueOnError)
		fs.SetOutput(a.Err)
		cfgPath := fs.String("config", "", "")
		force := fs.Bool("force", false, "overwrite an existing native config")
		flagArgs := args[1:]
		positionalPath := ""
		if len(flagArgs) > 0 && !strings.HasPrefix(flagArgs[0], "-") {
			positionalPath = flagArgs[0]
			flagArgs = flagArgs[1:]
		}
		if err := fs.Parse(flagArgs); err != nil {
			return err
		}
		if positionalPath == "" && fs.NArg() == 1 {
			positionalPath = fs.Arg(0)
		} else if fs.NArg() != 0 {
			return errors.New("config migrate accepts at most one path")
		}
		if positionalPath != "" {
			if *cfgPath != "" {
				return errors.New("config migrate path provided twice")
			}
			*cfgPath = positionalPath
		}
		legacy, native, err := config.Migrate(config.LoadOptions{Path: *cfgPath}, dir, *force)
		if err != nil {
			return err
		}
		if envPath := config.ExpandHome(os.Getenv("REX_SESH_CONFIG"), ""); envPath != "" {
			if sameFile(envPath, legacy) {
				a.warnf("migrated %s; set REX_SESH_CONFIG=%s before deleting %s", legacy, native, legacy)
			} else {
				a.warnf("migrated %s; REX_SESH_CONFIG continues to select %s", legacy, envPath)
			}
		} else {
			resolved, resolveErr := config.ResolvePath(config.LoadOptions{})
			if resolveErr == nil && filepath.Base(resolved) == config.NativeFileName && sameFile(resolved, native) {
				a.warnf("migrated %s; the native file now takes precedence — delete the legacy file once satisfied: %s", legacy, legacy)
			} else {
				a.warnf("migrated %s; set REX_SESH_CONFIG=%s before deleting %s (or keep using --config %s)", legacy, native, legacy, native)
			}
		}
		_, err = fmt.Fprintln(a.Out, native)
		return err
	default:
		return errors.New("unknown config command")
	}
}

func (a *App) validateConfig(args []string) error {
	if len(args) > 1 {
		return errors.New("config validate accepts at most one path")
	}
	var path string
	if len(args) == 1 {
		path = args[0] //nolint:gosec // len(args) is exactly one.
		if path == "" {
			return errors.New("config validate path must not be empty")
		}
	}
	_, resolved, err := config.Load(config.LoadOptions{Path: path, Warn: a.Err, StrictLegacy: true})
	if err != nil {
		return err
	}
	if resolved == "" {
		return errors.New("no config file found")
	}
	_, err = fmt.Fprintln(a.Out, resolved)
	return err
}
