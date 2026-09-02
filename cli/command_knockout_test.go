package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kopia/kopia/knockout"
	"github.com/kopia/kopia/tests/testenv"
)

func TestKnockoutSetupAndStatus(t *testing.T) {
	env := testenv.NewCLITest(t, testenv.RepoFormatNotImportant, testenv.NewInProcRunner(t))

	storage := t.TempDir()
	docs := filepath.Join(t.TempDir(), "Documents")
	require.NoError(t, os.MkdirAll(docs, 0o700))

	destFile := filepath.Join(t.TempDir(), "knockout-destination.json")
	writeKnockoutDestination(t, destFile, storage)

	stdout := env.RunAndExpectSuccess(t, "knockout", "setup",
		"--site-id", "Acme-HQ",
		"--passcode", testenv.TestRepoPassword,
		"--profile", knockout.ProfileWorkstation,
		"--destination", destFile,
		"--path", docs,
		"--json",
	)

	var res knockout.Result
	require.NoError(t, json.Unmarshal([]byte(strings.Join(stdout, "\n")), &res))
	require.Equal(t, "acme-hq", res.SiteID)
	require.True(t, res.Created)
	require.Contains(t, res.Sources, docs)

	statusOut := env.RunAndExpectSuccess(t, "knockout", "status", "--json")
	var st knockout.SiteState
	require.NoError(t, json.Unmarshal([]byte(strings.Join(statusOut, "\n")), &st))
	require.Equal(t, "acme-hq", st.SiteID)
	require.Equal(t, knockout.ProfileWorkstation, st.Profile)

	env.RunAndExpectSuccess(t, "policy", "list")
	env.RunAndExpectSuccess(t, "repo", "status")
}

func TestKnockoutProfiles(t *testing.T) {
	env := testenv.NewCLITest(t, testenv.RepoFormatNotImportant, testenv.NewInProcRunner(t))

	stdout := env.RunAndExpectSuccess(t, "knockout", "profiles", "--json")
	var rows []map[string]string
	require.NoError(t, json.Unmarshal([]byte(strings.Join(stdout, "\n")), &rows))
	require.GreaterOrEqual(t, len(rows), 4)
}

func TestKnockoutSetupRejectsUnknownProfile(t *testing.T) {
	env := testenv.NewCLITest(t, testenv.RepoFormatNotImportant, testenv.NewInProcRunner(t))

	destFile := filepath.Join(t.TempDir(), "knockout-destination.json")
	writeKnockoutDestination(t, destFile, t.TempDir())

	_, stderr := env.RunAndExpectFailure(t, "knockout", "setup",
		"--site-id", "x1",
		"--passcode", "passcode1",
		"--profile", "not-a-profile",
		"--destination", destFile,
		"--path", t.TempDir(),
	)
	require.Contains(t, stderr[len(stderr)-1], "unknown backup profile")
}

func writeKnockoutDestination(t *testing.T, destFile, storage string) {
	t.Helper()

	b, err := json.Marshal(knockout.Destination{
		Type: knockout.StorageFilesystem,
		Path: storage,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(destFile, b, 0o600))
}
