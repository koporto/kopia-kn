package knockout

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// SiteState is written next to the repository config after enrollment.
type SiteState struct {
	SiteID      string   `json:"siteID"`
	Profile     string   `json:"profile"`
	Prefix      string   `json:"prefix"`
	StorageType string   `json:"storageType"`
	Created     bool     `json:"created"`
	Sources     []string `json:"sources"`
	ConfigFile  string   `json:"configFile"`
}

// StatePath returns the knockout-site.json path beside configFile.
func StatePath(configFile string) string {
	return filepath.Join(filepath.Dir(configFile), StateFileName)
}

// WriteSiteState persists enrollment details for the UI and later reconnects.
func WriteSiteState(configFile string, st SiteState) error {
	st.ConfigFile = configFile

	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return errors.Wrap(err, "unable to encode site state")
	}

	if err := os.MkdirAll(filepath.Dir(configFile), 0o700); err != nil {
		return errors.Wrap(err, "unable to create config directory")
	}

	return errors.Wrap(os.WriteFile(StatePath(configFile), b, 0o600), "unable to write site state")
}

// ReadSiteState loads enrollment details if present.
func ReadSiteState(configFile string) (*SiteState, error) {
	b, err := os.ReadFile(StatePath(configFile)) //nolint:gosec
	if err != nil {
		return nil, errors.Wrap(err, "unable to read site state")
	}

	st := &SiteState{}
	if err := json.Unmarshal(b, st); err != nil {
		return nil, errors.Wrap(err, "invalid site state")
	}

	return st, nil
}
