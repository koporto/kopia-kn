package knockout

import (
	"context"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/kopia/kopia/internal/passwordpersist"
	"github.com/kopia/kopia/repo"
	"github.com/kopia/kopia/repo/blob"
	"github.com/kopia/kopia/repo/format"
	"github.com/kopia/kopia/repo/logging"
	"github.com/kopia/kopia/repo/maintenance"
	"github.com/kopia/kopia/snapshot"
	"github.com/kopia/kopia/snapshot/policy"
)

var log = logging.Module("kopia/knockout")

// Options controls enrollment.
type Options struct {
	SiteID         string
	Passcode       string
	Profile        string
	Destination    *Destination
	ConfigFile     string
	ExtraPaths     []string
	IncludeMissing bool
	Persist        passwordpersist.Strategy
}

// Result is returned after a successful create or connect.
type Result struct {
	SiteID      string   `json:"siteID"`
	Profile     string   `json:"profile"`
	Prefix      string   `json:"prefix"`
	StorageType string   `json:"storageType"`
	Created     bool     `json:"created"`
	Sources     []string `json:"sources"`
	ConfigFile  string   `json:"configFile"`
}

// Setup creates or connects the site repository and applies the selected profile.
func Setup(ctx context.Context, opt Options) (*Result, error) {
	siteID, dest, prof, err := validateSetup(opt)
	if err != nil {
		return nil, err
	}

	paths := prof.ResolvePaths(opt.ExtraPaths, opt.IncludeMissing)
	if len(paths) == 0 {
		return nil, errors.New("no backup paths found for this profile; pass --path or use --include-missing")
	}

	ci, err := dest.ConnectionInfo(siteID)
	if err != nil {
		return nil, err
	}

	created, err := connectOrCreate(ctx, opt, ci)
	if err != nil {
		return nil, err
	}

	if err := applyProfile(ctx, opt.ConfigFile, opt.Passcode, prof, paths); err != nil {
		return nil, err
	}

	res := &Result{
		SiteID:      siteID,
		Profile:     prof.ID,
		Prefix:      dest.SitePrefix(siteID),
		StorageType: dest.Type,
		Created:     created,
		Sources:     paths,
		ConfigFile:  opt.ConfigFile,
	}

	if err := WriteSiteState(opt.ConfigFile, SiteState{
		SiteID:      res.SiteID,
		Profile:     res.Profile,
		Prefix:      res.Prefix,
		StorageType: res.StorageType,
		Created:     res.Created,
		Sources:     res.Sources,
	}); err != nil {
		return nil, err
	}

	log(ctx).Infof("Knockout Backup enrolled site %s using profile %s (%d sources)", siteID, prof.ID, len(paths))

	return res, nil
}

func validateSetup(opt Options) (string, *Destination, Profile, error) {
	siteID, err := NormalizeSiteID(opt.SiteID)
	if err != nil {
		return "", nil, Profile{}, err
	}

	if err := ValidatePasscode(opt.Passcode); err != nil {
		return "", nil, Profile{}, err
	}

	if opt.Destination == nil {
		return "", nil, Profile{}, errors.New("destination is required")
	}

	if err := opt.Destination.CheckEnrollment(opt.Passcode); err != nil {
		return "", nil, Profile{}, err
	}

	if opt.ConfigFile == "" {
		return "", nil, Profile{}, errors.New("config file is required")
	}

	prof, err := LookupProfile(opt.Profile)
	if err != nil {
		return "", nil, Profile{}, err
	}

	return siteID, opt.Destination, prof, nil
}

func connectOrCreate(ctx context.Context, opt Options, ci blob.ConnectionInfo) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(opt.ConfigFile), 0o700); err != nil {
		return false, errors.Wrap(err, "unable to create config directory")
	}

	st, err := blob.NewStorage(ctx, ci, true)
	if err != nil {
		return false, errors.Wrap(err, "unable to open backup storage")
	}
	defer st.Close(ctx) //nolint:errcheck

	created := false

	exists, err := repositoryExists(ctx, st)
	if err != nil {
		return false, err
	}

	if !exists {
		if err := repo.Initialize(ctx, st, &repo.NewRepositoryOptions{}, opt.Passcode); err != nil {
			return false, errors.Wrap(err, "unable to create repository")
		}

		created = true
		log(ctx).Info("Created new Knockout Backup repository")
	}

	co := &repo.ConnectOptions{}
	co.ClientOptions.Description = ProductFullName + " - " + opt.SiteID

	persist := opt.Persist
	if persist == nil {
		persist = passwordpersist.File()
	}

	if err := passwordpersist.OnSuccess(ctx, repo.Connect(ctx, opt.ConfigFile, st, opt.Passcode, co), persist, opt.ConfigFile, opt.Passcode); err != nil {
		return false, errors.Wrap(err, "unable to connect to repository")
	}

	return created, nil
}

func repositoryExists(ctx context.Context, st blob.Storage) (bool, error) {
	_, err := st.GetMetadata(ctx, format.KopiaRepositoryBlobID)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, blob.ErrBlobNotFound) {
		return false, nil
	}

	return false, errors.Wrap(err, "unable to check for existing repository")
}

func applyProfile(ctx context.Context, configFile, password string, prof Profile, paths []string) error {
	rep, err := repo.Open(ctx, configFile, password, nil)
	if err != nil {
		return errors.Wrap(err, "unable to open repository")
	}
	defer rep.Close(ctx) //nolint:errcheck

	return errors.Wrap(repo.WriteSession(ctx, rep, repo.WriteSessionOptions{
		Purpose: "knockout setup",
	}, func(ctx context.Context, w repo.RepositoryWriter) error {
		return writePolicies(ctx, w, prof, paths)
	}), "unable to apply backup policy")
}

func writePolicies(ctx context.Context, w repo.RepositoryWriter, prof Profile, paths []string) error {
	global := prof.PolicyForPath()
	if err := policy.SetPolicy(ctx, w, policy.GlobalPolicySourceInfo, global); err != nil {
		return errors.Wrap(err, "unable to set global policy")
	}

	host := w.ClientOptions().Hostname
	userName := w.ClientOptions().Username
	pathPolicy := prof.PolicyForPath()

	for _, path := range paths {
		si := snapshot.SourceInfo{
			Host:     host,
			UserName: userName,
			Path:     path,
		}
		if err := policy.SetPolicy(ctx, w, si, pathPolicy); err != nil {
			return errors.Wrapf(err, "unable to set policy for %s", path)
		}
	}

	params := maintenance.DefaultParams()
	params.Owner = w.ClientOptions().UsernameAtHost()

	if err := maintenance.SetParams(ctx, w, &params); err != nil {
		return errors.Wrap(err, "unable to set maintenance parameters")
	}

	return nil
}
