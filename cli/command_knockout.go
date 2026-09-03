package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/kopia/kopia/knockout"
)

type commandKnockout struct {
	enroll   commandKnockoutSetup
	status   commandKnockoutStatus
	profiles commandKnockoutProfiles
}

func (c *commandKnockout) setup(svc advancedAppServices, parent commandParent) {
	cmd := parent.Command("knockout", "Knockout Networks managed backup enrollment.")

	c.enroll.setup(svc, cmd)
	c.status.setup(svc, cmd)
	c.profiles.setup(svc, cmd)
}

type commandKnockoutSetup struct {
	siteID         string
	passcode       string
	profile        string
	destination    string
	extraPaths     []string
	includeMissing bool
	jsonOutput     bool

	svc advancedAppServices
	out textOutput
}

func (c *commandKnockoutSetup) setup(svc advancedAppServices, parent commandParent) {
	cmd := parent.Command("setup", "Enroll this computer with a site ID and start nightly backups.")

	cmd.Flag("site-id", "Client site ID (becomes the storage subdirectory)").Required().StringVar(&c.siteID)
	cmd.Flag("passcode", "Site passcode (repository password; also checked against optional enrollment PIN)").StringVar(&c.passcode)
	cmd.Flag("profile", "Backup profile: workstation, workstation-full, server, server-system").Default(knockout.ProfileWorkstation).StringVar(&c.profile)
	cmd.Flag("destination", "Path to knockout-destination.json").Envar(svc.EnvName("KNOCKOUT_DESTINATION_FILE")).StringVar(&c.destination)
	cmd.Flag("path", "Additional directory to include (repeatable)").StringsVar(&c.extraPaths)
	cmd.Flag("include-missing", "Register profile paths even if they do not exist yet").BoolVar(&c.includeMissing)
	cmd.Flag("json", "Print enrollment result as JSON").BoolVar(&c.jsonOutput)

	c.svc = svc
	c.out.setup(svc)

	cmd.Action(svc.noRepositoryAction(c.run))
}

func (c *commandKnockoutSetup) run(ctx context.Context) error {
	passcode := c.passcode
	if passcode == "" {
		p, err := c.svc.getPasswordFromFlags(ctx, true, false)
		if err != nil {
			return errors.Wrap(err, "passcode")
		}

		passcode = p
	}

	dest, err := loadKnockoutDestination(c.svc, c.destination)
	if err != nil {
		return err
	}

	res, err := knockout.Setup(ctx, knockout.Options{
		SiteID:         c.siteID,
		Passcode:       passcode,
		Profile:        c.profile,
		Destination:    dest,
		ConfigFile:     c.svc.repositoryConfigFileName(),
		ExtraPaths:     c.extraPaths,
		IncludeMissing: c.includeMissing,
		Persist:        c.svc.passwordPersistenceStrategy(),
	})
	if err != nil {
		return err
	}

	if c.jsonOutput {
		enc := json.NewEncoder(c.out.stdout())
		enc.SetIndent("", "  ")

		return errors.Wrap(enc.Encode(res), "json")
	}

	created := "connected to existing repository"
	if res.Created {
		created = "created new repository"
	}

	c.out.printStdout("%s enrolled site %s (%s)\n", knockout.ProductFullName, res.SiteID, created)
	c.out.printStdout("  profile:  %s\n", res.Profile)
	c.out.printStdout("  prefix:   %s\n", res.Prefix)
	c.out.printStdout("  storage:  %s\n", res.StorageType)
	c.out.printStdout("  config:   %s\n", res.ConfigFile)
	c.out.printStdout("  sources:\n")

	for _, p := range res.Sources {
		c.out.printStdout("    - %s\n", p)
	}

	c.out.printStdout("\nNightly backups start automatically while %s or `kopia server start --ui` is running.\n", knockout.ProductName)

	return nil
}

type commandKnockoutStatus struct {
	jsonOutput bool

	svc advancedAppServices
	out textOutput
}

func (c *commandKnockoutStatus) setup(svc advancedAppServices, parent commandParent) {
	cmd := parent.Command("status", "Show Knockout site enrollment for this computer.")
	cmd.Flag("json", "Print site state as JSON").BoolVar(&c.jsonOutput)

	c.svc = svc
	c.out.setup(svc)
	cmd.Action(svc.noRepositoryAction(c.run))
}

func (c *commandKnockoutStatus) run(_ context.Context) error {
	st, err := knockout.ReadSiteState(c.svc.repositoryConfigFileName())
	if err != nil {
		return errors.Wrap(err, "this computer is not enrolled; run `kopia knockout setup`")
	}

	if c.jsonOutput {
		enc := json.NewEncoder(c.out.stdout())
		enc.SetIndent("", "  ")

		return errors.Wrap(enc.Encode(st), "json")
	}

	c.out.printStdout("Site ID:  %s\n", st.SiteID)
	c.out.printStdout("Profile:  %s\n", st.Profile)
	c.out.printStdout("Prefix:   %s\n", st.Prefix)
	c.out.printStdout("Storage:  %s\n", st.StorageType)
	c.out.printStdout("Created:  %v\n", st.Created)
	c.out.printStdout("Sources:\n")

	for _, p := range st.Sources {
		c.out.printStdout("  - %s\n", p)
	}

	return nil
}

type commandKnockoutProfiles struct {
	jsonOutput bool
	out        textOutput
}

func (c *commandKnockoutProfiles) setup(svc appServices, parent commandParent) {
	cmd := parent.Command("profiles", "List built-in Knockout backup profiles.")
	cmd.Flag("json", "Print profiles as JSON").BoolVar(&c.jsonOutput)
	c.out.setup(svc)
	cmd.Action(svc.noRepositoryAction(c.run))
}

func (c *commandKnockoutProfiles) run(_ context.Context) error {
	type row struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Summary  string `json:"summary"`
		Schedule string `json:"schedule"`
	}

	var rows []row

	for _, p := range knockout.Profiles() {
		rows = append(rows, row{
			ID:       p.ID,
			Title:    p.Title,
			Summary:  p.Summary,
			Schedule: p.Schedule.String(),
		})
	}

	if c.jsonOutput {
		enc := json.NewEncoder(c.out.stdout())
		enc.SetIndent("", "  ")

		return errors.Wrap(enc.Encode(rows), "json")
	}

	for _, r := range rows {
		c.out.printStdout("%s  (%s)\n  %s\n\n", r.ID, r.Schedule, r.Summary)
	}

	return nil
}

func loadKnockoutDestination(svc appServices, explicit string) (*knockout.Destination, error) {
	path := explicit
	if path == "" {
		exeDir := ""
		if exe, err := os.Executable(); err == nil {
			exeDir = filepath.Dir(exe)
		}

		found, err := knockout.FindDestinationFile(exeDir, filepath.Dir(svc.repositoryConfigFileName()))
		if err != nil {
			return nil, err
		}

		path = found
	}

	dest, err := knockout.LoadDestinationFile(path)
	if err != nil {
		return nil, err
	}

	dest.ApplyEnv(os.Getenv)

	if err := dest.Validate(); err != nil {
		return nil, errors.Wrapf(err, "destination file %s", path)
	}

	return dest, nil
}
