package app

import (
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
	"time"

	"github.com/fullerzz/rex-plugin-sesh/internal/config"
	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/fullerzz/rex-plugin-sesh/internal/namer"
	pickerpkg "github.com/fullerzz/rex-plugin-sesh/internal/picker"
	"github.com/fullerzz/rex-plugin-sesh/internal/preview"
	"github.com/fullerzz/rex-plugin-sesh/internal/sources"
	"github.com/fullerzz/rex-plugin-sesh/internal/state"
)

var Version = "dev"

type App struct {
	Out io.Writer
	Err io.Writer
}

func New() *App { return &App{Out: os.Stdout, Err: os.Stderr} }
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
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
func (a *App) usage() error {
	_, err := fmt.Fprintln(a.Out, "rex-sesh list|preview|root|picker|config|--version")
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

// collect merges config and zoxide sessions. localOnly drops [[workspace]]
// entries on another machine: list and the picker cannot open them yet.
func (a *App) collect(ctx context.Context, cfg config.Config, target string, localOnly bool) ([]model.Session, error) {
	srcs := []sources.Source{sources.ConfigSessions{Config: cfg, LocalOnly: localOnly}, sources.Zoxide{}}
	if target != "" {
		direct := sources.DirectPath{Path: target}
		// Naming runs git in the target, so only a local directory is named:
		// the target may be the name of a workspace on another machine.
		if st, err := os.Stat(target); err == nil && st.IsDir() {
			direct.Label = namer.Namer{}.Name(ctx, target, cfg.DirLength)
		}
		srcs = append(srcs, direct)
	}
	merged, err := sources.Merge(ctx, srcs, cfg.SortOrder, cfg.Blacklist, false, true)
	if err != nil {
		return nil, err
	}
	sources.ApplyConfig(&merged, cfg, "")
	return merged.Ordered(), nil
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
	ss, err := sources.Merge(ctx, []sources.Source{sources.ConfigSessions{Config: cfg, LocalOnly: true}, sources.Zoxide{}}, cfg.SortOrder, cfg.Blacklist, *blacklisted, *hideDup)
	if err != nil {
		return err
	}
	sources.ApplyConfig(&ss, cfg, "")
	sessions := ss.Ordered()
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
		HideLastWorkspace:              !cfg.TUI.ShowLastWorkspace,
		HideLastWorkspacePath:          !cfg.TUI.ShowLastWorkspacePath,
		SeparatorAware:                 cfg.SeparatorAware,
		DisableHomePrioritization:      !cfg.TUI.PrioritizeHome,
		HidePath:                       !cfg.TUI.ShowPath,
		HidePreview:                    !cfg.TUI.ShowPreview,
		PreviewMode:                    cfg.TUI.PreviewMode,
		CyclePreviewModeKey:            &cfg.Keys.CyclePreviewMode,
		DefaultPreviewCommand:          cfg.DefaultSessionConfig.PreviewCommand,
		WorkspaceSort:                  cfg.TUI.DefaultSort,
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

// picker prints the chosen session's name. Switching to it comes with the Rex
// client.
func (a *App) picker(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("picker", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	jsonOut := fs.Bool("json", false, "")
	cfgPath := fs.String("config", "", "")
	fzfPicker := fs.Bool("fzf", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, activeConfigPath, err := config.Load(config.LoadOptions{Path: *cfgPath, Warn: a.Err})
	if err != nil {
		return err
	}
	sessions, err := a.collect(ctx, cfg, "", true)
	if err != nil {
		return err
	}
	if *jsonOut {
		return a.printSessions(sessions, true)
	}
	opts := pickerOptionsFromConfig(ctx, a.Out, cfg)
	var selected model.Session
	var ok bool
	if *fzfPicker || strings.EqualFold(os.Getenv("REX_SESH_PICKER"), "fzf") {
		selected, ok, err = pickerpkg.RunFZF(ctx, sessions, pickerpkg.Options{DisplayOptions: opts})
	} else {
		backend := &pickerBackend{app: a, ctx: ctx, cfg: cfg, activeConfigPath: activeConfigPath}
		selected, ok, err = pickerpkg.Run(sessions, pickerpkg.Options{DisplayOptions: opts, Backend: backend})
	}
	if err != nil || !ok {
		return err
	}
	_, err = fmt.Fprintln(a.Out, selected.Name)
	return err
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
	sessions, err := a.collect(ctx, cfg, target, false)
	if err != nil {
		return err
	}
	s, dir, ok := resolveTarget(sessions, target)
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

// stateDir holds rex-sesh's session cache: REX_SESH_STATE_DIR, or rex-sesh
// under the XDG state home. It is "" when there is nowhere to cache, which
// disables caching.
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
		if c.WorkspaceID != "" && c.WorkspaceID == target {
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
