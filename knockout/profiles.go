package knockout

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"

	"github.com/kopia/kopia/snapshot/policy"
)

// Backup profile names offered at enrollment.
const (
	ProfileWorkstation     = "workstation"
	ProfileWorkstationFull = "workstation-full"
	ProfileServer          = "server"
	ProfileServerSystem    = "server-system"
)

// Profile describes a preconfigured set of sources and snapshot policy.
type Profile struct {
	ID        string
	Title     string
	Summary   string
	Schedule  policy.TimeOfDay
	VSS       policy.OSSnapshotMode
	Retention policy.RetentionPolicy
	Ignores   []string
	OneFS     bool
	pathFn    func() []string
}

// Profiles returns the built-in MSP backup choices.
func Profiles() []Profile {
	return []Profile{
		{
			ID:       ProfileWorkstation,
			Title:    "Windows workstation",
			Summary:  "Nightly copy of Documents, Desktop, Pictures, and other user folders. Best for typical office PCs.",
			Schedule: policy.TimeOfDay{Hour: 2, Minute: 0},
			VSS:      policy.OSSnapshotWhenAvailable,
			Retention: policy.RetentionPolicy{
				KeepLatest:  intPtr(10),
				KeepHourly:  intPtr(0),
				KeepDaily:   intPtr(14),
				KeepWeekly:  intPtr(8),
				KeepMonthly: intPtr(12),
				KeepAnnual:  intPtr(3),
			},
			Ignores: workstationIgnores(),
			pathFn:  workstationEssentialPaths,
		},
		{
			ID:       ProfileWorkstationFull,
			Title:    "Windows workstation (full profile)",
			Summary:  "Nightly copy of the entire user profile, minus caches and temp files.",
			Schedule: policy.TimeOfDay{Hour: 2, Minute: 0},
			VSS:      policy.OSSnapshotWhenAvailable,
			Retention: policy.RetentionPolicy{
				KeepLatest:  intPtr(10),
				KeepHourly:  intPtr(0),
				KeepDaily:   intPtr(14),
				KeepWeekly:  intPtr(8),
				KeepMonthly: intPtr(12),
				KeepAnnual:  intPtr(3),
			},
			Ignores: workstationIgnores(),
			pathFn:  workstationFullPaths,
		},
		{
			ID:       ProfileServer,
			Title:    "Windows server (data volumes)",
			Summary:  "Nightly copy of data drives and common share/IIS roots. Best for file and application servers.",
			Schedule: policy.TimeOfDay{Hour: 1, Minute: 30},
			VSS:      policy.OSSnapshotAlways,
			Retention: policy.RetentionPolicy{
				KeepLatest:  intPtr(14),
				KeepHourly:  intPtr(0),
				KeepDaily:   intPtr(30),
				KeepWeekly:  intPtr(12),
				KeepMonthly: intPtr(24),
				KeepAnnual:  intPtr(7),
			},
			Ignores: serverIgnores(),
			OneFS:   true,
			pathFn:  serverDataPaths,
		},
		{
			ID:       ProfileServerSystem,
			Title:    "Windows server (system + data)",
			Summary:  "Nightly copy of C: plus data volumes, excluding pagefile, hibernation, and Windows update cache.",
			Schedule: policy.TimeOfDay{Hour: 1, Minute: 30},
			VSS:      policy.OSSnapshotAlways,
			Retention: policy.RetentionPolicy{
				KeepLatest:  intPtr(14),
				KeepHourly:  intPtr(0),
				KeepDaily:   intPtr(30),
				KeepWeekly:  intPtr(12),
				KeepMonthly: intPtr(24),
				KeepAnnual:  intPtr(7),
			},
			Ignores: serverSystemIgnores(),
			OneFS:   true,
			pathFn:  serverSystemPaths,
		},
	}
}

// LookupProfile returns a built-in profile by ID.
func LookupProfile(id string) (Profile, error) {
	want := strings.ToLower(strings.TrimSpace(id))
	if want == "" {
		want = ProfileWorkstation
	}

	for _, p := range Profiles() {
		if p.ID == want {
			return p, nil
		}
	}

	return Profile{}, errors.Errorf("unknown backup profile %q (use workstation, workstation-full, server, or server-system)", id)
}

// ResolvePaths returns profile sources that exist, plus extraPaths.
func (p Profile) ResolvePaths(extraPaths []string, includeMissing bool) []string {
	seen := map[string]bool{}
	var out []string

	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}

		if !includeMissing {
			st, err := os.Stat(path)
			if err != nil || !st.IsDir() {
				return
			}
		}

		if seen[path] {
			return
		}

		seen[path] = true
		out = append(out, path)
	}

	if p.pathFn != nil {
		for _, path := range p.pathFn() {
			add(path)
		}
	}

	for _, path := range extraPaths {
		add(path)
	}

	return out
}

// PolicyForPath builds the snapshot policy applied to each source.
func (p Profile) PolicyForPath() *policy.Policy {
	vss := p.VSS
	one := policy.OptionalBool(p.OneFS)
	runMissed := policy.OptionalBool(true)
	ignoreCaches := policy.OptionalBool(true)

	return &policy.Policy{
		RetentionPolicy: p.Retention,
		SchedulingPolicy: policy.SchedulingPolicy{
			TimesOfDay: []policy.TimeOfDay{p.Schedule},
			RunMissed:  policy.NewOptionalBool(runMissed),
		},
		FilesPolicy: policy.FilesPolicy{
			IgnoreRules:            p.Ignores,
			IgnoreCacheDirectories: policy.NewOptionalBool(ignoreCaches),
			OneFileSystem:          policy.NewOptionalBool(one),
		},
		CompressionPolicy: policy.CompressionPolicy{
			CompressorName: "zstd",
		},
		OSSnapshotPolicy: policy.OSSnapshotPolicy{
			VolumeShadowCopy: policy.VolumeShadowCopyPolicy{
				Enable: policy.NewOSSnapshotMode(vss),
			},
		},
	}
}

func intPtr(v int) *policy.OptionalInt {
	n := policy.OptionalInt(v)
	return &n
}

func userHome() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return h
}

func joinHome(parts ...string) string {
	home := userHome()
	if home == "" {
		return ""
	}

	return filepath.Join(append([]string{home}, parts...)...)
}

func workstationEssentialPaths() []string {
	return []string{
		joinHome("Documents"),
		joinHome("Desktop"),
		joinHome("Pictures"),
		joinHome("Videos"),
		joinHome("Music"),
		joinHome("Downloads"),
		joinHome("Favorites"),
		joinHome("OneDrive"),
	}
}

func workstationFullPaths() []string {
	if runtime.GOOS == "windows" {
		return []string{userHome()}
	}

	return []string{userHome()}
}

func serverDataPaths() []string {
	if runtime.GOOS != "windows" {
		return []string{"/srv", "/data", "/shares"}
	}

	return []string{
		`C:\Shares`,
		`D:\`,
		`E:\`,
		`F:\`,
		`C:\inetpub`,
		`C:\FileShares`,
	}
}

func serverSystemPaths() []string {
	if runtime.GOOS != "windows" {
		return append(serverDataPaths(), "/")
	}

	return append([]string{`C:\`}, serverDataPaths()...)
}

func workstationIgnores() []string {
	return []string{
		"**/AppData/Local/Temp/**",
		"**/AppData/Local/Microsoft/Windows/INetCache/**",
		"**/AppData/Local/Microsoft/Windows/Explorer/**",
		"**/AppData/Local/Google/Chrome/User Data/*/Cache/**",
		"**/AppData/Local/Microsoft/Edge/User Data/*/Cache/**",
		"**/AppData/Local/Mozilla/Firefox/Profiles/*/cache2/**",
		"**/AppData/Roaming/Microsoft/Windows/Recent/**",
		"**/$Recycle.Bin/**",
		"**/Temp/**",
		"**/tmp/**",
		"**/node_modules/**",
		"**/.git/**",
		"*.tmp",
		"Thumbs.db",
		"desktop.ini",
		"NTUSER.DAT*",
		"*.ost",
	}
}

func serverIgnores() []string {
	return append(workstationIgnores(),
		"pagefile.sys",
		"hiberfil.sys",
		"swapfile.sys",
		"**/Windows/Temp/**",
		"**/Windows/SoftwareDistribution/**",
		"**/System Volume Information/**",
	)
}

func serverSystemIgnores() []string {
	return append(serverIgnores(),
		"**/Windows/WinSxS/**",
		"**/Windows/Installer/**",
		"**/Windows/Logs/**",
		"**/Windows/Prefetch/**",
		"**/Windows/ServiceProfiles/**",
		"**/Recovery/**",
	)
}
