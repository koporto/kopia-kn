package knockout_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kopia/kopia/internal/testlogging"
	"github.com/kopia/kopia/knockout"
	"github.com/kopia/kopia/repo"
	"github.com/kopia/kopia/snapshot"
	"github.com/kopia/kopia/snapshot/policy"
)

func TestSetupCreatesRepositoryAndPolicies(t *testing.T) {
	ctx := testlogging.Context(t)
	root := t.TempDir()
	storage := filepath.Join(root, "storage")
	config := filepath.Join(root, "repository.config")
	docs := filepath.Join(root, "Documents")
	require.NoError(t, os.MkdirAll(docs, 0o700))

	res, err := knockout.Setup(ctx, knockout.Options{
		SiteID:   "Acme-HQ",
		Passcode: "client-pin",
		Profile:  knockout.ProfileWorkstation,
		Destination: &knockout.Destination{
			Type: knockout.StorageFilesystem,
			Path: storage,
		},
		ConfigFile: config,
		ExtraPaths: []string{docs},
	})
	require.NoError(t, err)
	require.True(t, res.Created)
	require.Equal(t, "acme-hq", res.SiteID)
	require.Equal(t, "sites/acme-hq/", res.Prefix)
	require.Contains(t, res.Sources, docs)

	st, err := knockout.ReadSiteState(config)
	require.NoError(t, err)
	require.Equal(t, "acme-hq", st.SiteID)

	rep, err := repo.Open(ctx, config, "client-pin", nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rep.Close(ctx)) })

	si := snapshot.SourceInfo{
		Host:     rep.ClientOptions().Hostname,
		UserName: rep.ClientOptions().Username,
		Path:     docs,
	}
	pol, err := policy.GetDefinedPolicy(ctx, rep, si)
	require.NoError(t, err)
	require.Equal(t, 2, pol.SchedulingPolicy.TimesOfDay[0].Hour)
	require.EqualValues(t, 14, pol.RetentionPolicy.KeepDaily.OrDefault(0))
}

func TestSetupConnectsExistingRepository(t *testing.T) {
	ctx := testlogging.Context(t)
	root := t.TempDir()
	storage := filepath.Join(root, "storage")
	docs := filepath.Join(root, "data")
	require.NoError(t, os.MkdirAll(docs, 0o700))

	opt := knockout.Options{
		SiteID:   "site-a",
		Passcode: "same-pin!",
		Profile:  knockout.ProfileWorkstation,
		Destination: &knockout.Destination{
			Type: knockout.StorageFilesystem,
			Path: storage,
		},
		ExtraPaths: []string{docs},
	}

	opt.ConfigFile = filepath.Join(root, "one", "repository.config")
	first, err := knockout.Setup(ctx, opt)
	require.NoError(t, err)
	require.True(t, first.Created)

	opt.ConfigFile = filepath.Join(root, "two", "repository.config")
	second, err := knockout.Setup(ctx, opt)
	require.NoError(t, err)
	require.False(t, second.Created)
	require.Equal(t, first.Prefix, second.Prefix)
}

func TestSetupRejectsWrongEnrollmentPin(t *testing.T) {
	ctx := testlogging.Context(t)
	_, err := knockout.Setup(ctx, knockout.Options{
		SiteID:   "site-b",
		Passcode: "wrong-pin",
		Destination: &knockout.Destination{
			Type:                     knockout.StorageFilesystem,
			Path:                     t.TempDir(),
			EnrollmentPasscodeSHA256: knockout.PasscodeSHA256("right-pin"),
		},
		ConfigFile: filepath.Join(t.TempDir(), "repository.config"),
		ExtraPaths: []string{t.TempDir()},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "enrollment PIN")
}

func TestSetupRequiresExistingPaths(t *testing.T) {
	ctx := testlogging.Context(t)
	_, err := knockout.Setup(ctx, knockout.Options{
		SiteID:   "empty1",
		Passcode: "passcode1",
		Destination: &knockout.Destination{
			Type: knockout.StorageFilesystem,
			Path: t.TempDir(),
		},
		ConfigFile: filepath.Join(t.TempDir(), "repository.config"),
		Profile:    knockout.ProfileWorkstation,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no backup paths")
}

func TestSiteStateRoundTripJSON(t *testing.T) {
	t.Parallel()

	config := filepath.Join(t.TempDir(), "repository.config")
	require.NoError(t, knockout.WriteSiteState(config, knockout.SiteState{
		SiteID:  "acme",
		Profile: knockout.ProfileServer,
		Sources: []string{`D:\`},
	}))

	st, err := knockout.ReadSiteState(config)
	require.NoError(t, err)
	require.Equal(t, "acme", st.SiteID)

	raw, err := os.ReadFile(knockout.StatePath(config))
	require.NoError(t, err)
	require.True(t, json.Valid(raw))
}

func TestSetupUnknownProfile(t *testing.T) {
	ctx := testlogging.Context(t)
	_, err := knockout.Setup(ctx, knockout.Options{
		SiteID:     "site-c",
		Passcode:   "passcode1",
		Profile:    "laptop-custom",
		ConfigFile: filepath.Join(t.TempDir(), "repository.config"),
		Destination: &knockout.Destination{
			Type: knockout.StorageFilesystem,
			Path: t.TempDir(),
		},
		ExtraPaths: []string{t.TempDir()},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown backup profile")
}
