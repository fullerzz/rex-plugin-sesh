package sources

import (
	"context"
	"testing"

	"github.com/fullerzz/rex-plugin-sesh/internal/config"
	"github.com/fullerzz/rex-plugin-sesh/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoteConfigSessionKeepsPathsForItsMachine(t *testing.T) {
	cfg := config.Config{
		SessionConfigs: []config.SessionConfig{
			{Name: "api", Machine: "buntu26", Path: "~/src/api", Windows: []string{"dev"}},
			{Name: "local", Path: "~/src/local", Windows: []string{"dev"}},
		},
		WindowConfigs: []model.WindowConfig{{Name: "dev", Path: "web", Panes: []model.PaneConfig{
			{Name: "shell"}, {Name: "logs", Path: "~/logs"},
		}}},
		WildcardConfigs: []config.WildcardConfig{{Pattern: "~/src/**", StartupCommand: "local-rule"}},
	}
	got, err := ConfigSessions{Config: cfg, Home: "/Users/me"}.List(context.Background())
	require.NoError(t, err)
	ApplyConfig(&got, cfg, "/Users/me")
	items := got.Ordered()
	require.Len(t, items, 2)

	remote := items[0]
	assert.Equal(t, "buntu26", remote.Machine)
	assert.Equal(t, "~/src/api", remote.Path, "no local home expansion")
	assert.Empty(t, remote.StartupCommand, "no local rule")
	require.Len(t, remote.WindowConfigs, 1)
	assert.Equal(t, "~/src/api/web", remote.WindowConfigs[0].Path)
	assert.Equal(t, "~/src/api/web", remote.WindowConfigs[0].Panes[0].Path)
	assert.Equal(t, "~/logs", remote.WindowConfigs[0].Panes[1].Path, "a ~ path is not joined to its parent")

	local := items[1]
	assert.Equal(t, "/Users/me/src/local", local.Path)
	assert.Equal(t, "local-rule", local.StartupCommand)
	assert.Equal(t, "/Users/me/logs", local.WindowConfigs[0].Panes[1].Path)
}

func TestLocalOnlyConfigSessionsOmitRemoteWorkspaces(t *testing.T) {
	cfg := config.Config{SessionConfigs: []config.SessionConfig{
		{Name: "api", Machine: "buntu26", Path: "~/src/api"},
		{Name: "web", Path: "/src/web"},
	}}
	got, err := ConfigSessions{Config: cfg, LocalOnly: true}.List(context.Background())
	require.NoError(t, err)
	items := got.Ordered()
	require.Len(t, items, 1)
	assert.Equal(t, "web", items[0].Name)

	got, err = ConfigSessions{Config: cfg}.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, got.Ordered(), 2, "the zero value keeps remote entries")
}

func TestRemoteConfigRelativePathsKeepHomeAnchor(t *testing.T) {
	cfg := config.Config{
		SessionConfigs: []config.SessionConfig{
			{Name: "home", Machine: "buntu26", Path: "~", Windows: []string{"up", "down"}},
			{Name: "src", Machine: "buntu26", Path: "~/src", Windows: []string{"up2"}},
		},
		WindowConfigs: []model.WindowConfig{
			{Name: "up", Path: ".."},
			{Name: "down", Path: "web", Panes: []model.PaneConfig{{Name: "back", Path: ".."}}},
			{Name: "up2", Path: "../.."},
		},
	}
	got, err := ConfigSessions{Config: cfg, Home: "/Users/me"}.List(context.Background())
	require.NoError(t, err)
	items := got.Ordered()
	require.Len(t, items, 2)
	require.Len(t, items[0].WindowConfigs, 2)
	assert.Equal(t, "~/..", items[0].WindowConfigs[0].Path)
	assert.Equal(t, "~/web", items[0].WindowConfigs[1].Path)
	assert.Equal(t, "~", items[0].WindowConfigs[1].Panes[0].Path)
	require.Len(t, items[1].WindowConfigs, 1)
	assert.Equal(t, "~/..", items[1].WindowConfigs[0].Path)
}
