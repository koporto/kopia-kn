package knockout_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kopia/kopia/knockout"
	"github.com/kopia/kopia/snapshot/policy"
)

func TestLookupProfile(t *testing.T) {
	t.Parallel()

	ids := map[string]bool{}
	for _, p := range knockout.Profiles() {
		ids[p.ID] = true
		require.NotEmpty(t, p.Title)
		require.NotEmpty(t, p.Summary)
	}

	require.True(t, ids[knockout.ProfileWorkstation])
	require.True(t, ids[knockout.ProfileServer])

	p, err := knockout.LookupProfile("")
	require.NoError(t, err)
	require.Equal(t, knockout.ProfileWorkstation, p.ID)

	_, err = knockout.LookupProfile("nope")
	require.Error(t, err)
}

func TestProfileResolvePathsAndPolicy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	docs := filepath.Join(root, "Documents")
	require.NoError(t, os.Mkdir(docs, 0o700))

	p, err := knockout.LookupProfile(knockout.ProfileWorkstation)
	require.NoError(t, err)

	got := p.ResolvePaths([]string{docs, filepath.Join(root, "missing")}, false)
	require.Equal(t, []string{docs}, got)

	got = p.ResolvePaths([]string{docs}, true)
	require.Contains(t, got, docs)

	pol := p.PolicyForPath()
	require.Equal(t, 2, pol.SchedulingPolicy.TimesOfDay[0].Hour)
	require.Equal(t, policy.OSSnapshotWhenAvailable, pol.OSSnapshotPolicy.VolumeShadowCopy.Enable.OrDefault(policy.OSSnapshotNever))
	require.EqualValues(t, 14, pol.RetentionPolicy.KeepDaily.OrDefault(0))
	require.Contains(t, pol.FilesPolicy.IgnoreRules, "*.tmp")
}

func TestServerProfileDefaults(t *testing.T) {
	t.Parallel()

	p, err := knockout.LookupProfile(knockout.ProfileServer)
	require.NoError(t, err)
	require.Equal(t, 1, p.Schedule.Hour)
	require.Equal(t, 30, p.Schedule.Minute)

	pol := p.PolicyForPath()
	require.Equal(t, policy.OSSnapshotAlways, pol.OSSnapshotPolicy.VolumeShadowCopy.Enable.OrDefault(policy.OSSnapshotNever))
	require.EqualValues(t, 30, pol.RetentionPolicy.KeepDaily.OrDefault(0))
	require.True(t, pol.FilesPolicy.OneFileSystem.OrDefault(false))
}
