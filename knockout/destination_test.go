package knockout_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kopia/kopia/knockout"
	"github.com/kopia/kopia/repo/blob/filesystem"
	"github.com/kopia/kopia/repo/blob/s3"
)

func TestDestinationValidateAndPrefix(t *testing.T) {
	t.Parallel()

	d := &knockout.Destination{Type: "s3", Bucket: "kn-backups"}
	require.Error(t, d.Validate())

	d.AccessKeyID = "AKIATEST"
	d.SecretAccessKey = "secret"
	require.NoError(t, d.Validate())
	require.Equal(t, "sites/acme-hq/", d.SitePrefix("acme-hq"))

	ci, err := d.ConnectionInfo("acme-hq")
	require.NoError(t, err)
	require.Equal(t, "s3", ci.Type)

	s3opt, ok := ci.Config.(*s3.Options)
	require.True(t, ok)
	require.Equal(t, "kn-backups", s3opt.BucketName)
	require.Equal(t, "sites/acme-hq/", s3opt.Prefix)
}

func TestDestinationFilesystemConnectionInfo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	d := &knockout.Destination{Type: "filesystem", Path: root, PrefixBase: "sites/"}
	require.NoError(t, d.Validate())

	ci, err := d.ConnectionInfo("lab1")
	require.NoError(t, err)

	fsopt, ok := ci.Config.(*filesystem.Options)
	require.True(t, ok)
	require.Equal(t, filepath.Join(root, "sites", "lab1"), fsopt.Path)
}

func TestDestinationEnrollment(t *testing.T) {
	t.Parallel()

	d := &knockout.Destination{
		Type:                     "filesystem",
		Path:                     t.TempDir(),
		EnrollmentPasscodeSHA256: knockout.PasscodeSHA256("enroll-me"),
	}
	require.Error(t, d.CheckEnrollment("wrong-pass"))
	require.NoError(t, d.CheckEnrollment("enroll-me"))
}

func TestDestinationApplyEnvAndFindFile(t *testing.T) {
	t.Parallel()

	d := &knockout.Destination{Type: "s3"}
	d.ApplyEnv(func(key string) string {
		switch key {
		case "KNOCKOUT_BUCKET":
			return "from-env"
		case "KNOCKOUT_ACCESS_KEY_ID":
			return "id"
		case "KNOCKOUT_SECRET_ACCESS_KEY":
			return "secret"
		default:
			return ""
		}
	})
	require.Equal(t, "from-env", d.Bucket)
	require.NoError(t, d.Validate())

	dir := t.TempDir()
	destFile := filepath.Join(dir, knockout.DestinationName)
	require.NoError(t, os.WriteFile(destFile, []byte(`{"type":"filesystem","path":"/tmp/kn"}`), 0o600))

	found, err := knockout.FindDestinationFile("", dir)
	require.NoError(t, err)
	require.Equal(t, destFile, found)

	loaded, err := knockout.LoadDestinationFile(found)
	require.NoError(t, err)
	require.Equal(t, "filesystem", loaded.Type)
}
