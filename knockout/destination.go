package knockout

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/kopia/kopia/repo/blob"
	"github.com/kopia/kopia/repo/blob/filesystem"
	"github.com/kopia/kopia/repo/blob/s3"
)

// Supported destination types.
const (
	StorageS3         = "s3"
	StorageB2         = "b2"
	StorageFilesystem = "filesystem"
)

// Destination is the MSP-wide storage target. Per-client isolation is the
// site ID prefix, not a separate bucket.
type Destination struct {
	Type string `json:"type"`

	// Shared object-store settings (S3 and B2).
	Bucket          string `json:"bucket,omitempty"`
	Endpoint        string `json:"endpoint,omitempty"`
	Region          string `json:"region,omitempty"`
	AccessKeyID     string `json:"accessKeyID,omitempty"`
	SecretAccessKey string `json:"secretAccessKey,omitempty"`
	SessionToken    string `json:"sessionToken,omitempty"`
	KeyID           string `json:"keyID,omitempty"`
	Key             string `json:"key,omitempty"`
	DoNotUseTLS     bool   `json:"doNotUseTLS,omitempty"`
	DoNotVerifyTLS  bool   `json:"doNotVerifyTLS,omitempty"`

	// Filesystem path used by tests and lab installs.
	Path string `json:"path,omitempty"`

	// PrefixBase is prepended to the site ID (default "sites/").
	PrefixBase string `json:"prefixBase,omitempty"`

	// EnrollmentPasscodeSHA256, when set, must match SHA-256(passcode).
	EnrollmentPasscodeSHA256 string `json:"enrollmentPasscodeSHA256,omitempty"`

	// EnrollmentPasscode is a plaintext shared PIN for lab use only.
	EnrollmentPasscode string `json:"enrollmentPasscode,omitempty"`
}

// LoadDestinationFile reads a destination JSON file.
func LoadDestinationFile(path string) (*Destination, error) {
	b, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, errors.Wrap(err, "unable to read destination file")
	}

	d := &Destination{}
	if err := json.Unmarshal(b, d); err != nil {
		return nil, errors.Wrap(err, "invalid destination file")
	}

	return d, nil
}

// ApplyEnv overlays KNOCKOUT_* environment variables onto d.
func (d *Destination) ApplyEnv(getenv func(string) string) {
	if getenv == nil {
		getenv = os.Getenv
	}

	set := func(dst *string, key string) {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			*dst = v
		}
	}

	set(&d.Type, "KNOCKOUT_STORAGE_TYPE")
	set(&d.Bucket, "KNOCKOUT_BUCKET")
	set(&d.Endpoint, "KNOCKOUT_ENDPOINT")
	set(&d.Region, "KNOCKOUT_REGION")
	set(&d.AccessKeyID, "KNOCKOUT_ACCESS_KEY_ID")
	set(&d.SecretAccessKey, "KNOCKOUT_SECRET_ACCESS_KEY")
	set(&d.SessionToken, "KNOCKOUT_SESSION_TOKEN")
	set(&d.KeyID, "KNOCKOUT_B2_KEY_ID")
	set(&d.Key, "KNOCKOUT_B2_KEY")
	set(&d.Path, "KNOCKOUT_FILESYSTEM_PATH")
	set(&d.PrefixBase, "KNOCKOUT_PREFIX_BASE")
	set(&d.EnrollmentPasscode, "KNOCKOUT_ENROLLMENT_PASSCODE")
	set(&d.EnrollmentPasscodeSHA256, "KNOCKOUT_ENROLLMENT_PASSCODE_SHA256")
}

// Validate checks that the destination has the fields required for its type.
func (d *Destination) Validate() error {
	switch strings.ToLower(strings.TrimSpace(d.Type)) {
	case StorageS3:
		if d.Bucket == "" {
			return errors.New("S3 destination requires bucket")
		}

		if d.AccessKeyID == "" || d.SecretAccessKey == "" {
			return errors.New("S3 destination requires accessKeyID and secretAccessKey")
		}
	case StorageB2:
		if d.Bucket == "" {
			return errors.New("B2 destination requires bucket")
		}

		if d.KeyID == "" || d.Key == "" {
			return errors.New("B2 destination requires keyID and key")
		}
	case StorageFilesystem:
		if d.Path == "" {
			return errors.New("filesystem destination requires path")
		}
	case "":
		return errors.New("destination type is required (s3, b2, or filesystem)")
	default:
		return errors.Errorf("unsupported destination type %q", d.Type)
	}

	return nil
}

// CheckEnrollment verifies the technician passcode against optional destination PIN.
func (d *Destination) CheckEnrollment(passcode string) error {
	if d.EnrollmentPasscode != "" && passcode != d.EnrollmentPasscode {
		return errors.New("passcode does not match enrollment PIN")
	}

	if d.EnrollmentPasscodeSHA256 != "" && !strings.EqualFold(PasscodeSHA256(passcode), d.EnrollmentPasscodeSHA256) {
		return errors.New("passcode does not match enrollment PIN")
	}

	return nil
}

// SitePrefix returns the storage prefix for siteID.
func (d *Destination) SitePrefix(siteID string) string {
	return SitePrefix(d.PrefixBase, siteID)
}

// ConnectionInfo builds a Kopia blob connection for the given site.
func (d *Destination) ConnectionInfo(siteID string) (blob.ConnectionInfo, error) {
	if err := d.Validate(); err != nil {
		return blob.ConnectionInfo{}, err
	}

	prefix := d.SitePrefix(siteID)

	switch strings.ToLower(d.Type) {
	case StorageS3:
		return blob.ConnectionInfo{
			Type: StorageS3,
			Config: &s3.Options{
				BucketName:      d.Bucket,
				Prefix:          prefix,
				Endpoint:        d.Endpoint,
				Region:          d.Region,
				AccessKeyID:     d.AccessKeyID,
				SecretAccessKey: d.SecretAccessKey,
				SessionToken:    d.SessionToken,
				DoNotUseTLS:     d.DoNotUseTLS,
				DoNotVerifyTLS:  d.DoNotVerifyTLS,
			},
		}, nil
	case StorageB2:
		return b2ConnectionInfo(d, prefix)
	case StorageFilesystem:
		return blob.ConnectionInfo{
			Type: StorageFilesystem,
			Config: &filesystem.Options{
				Path: filepath.Join(d.Path, filepath.FromSlash(strings.TrimSuffix(prefix, "/"))),
			},
		}, nil
	default:
		return blob.ConnectionInfo{}, errors.Errorf("unsupported destination type %q", d.Type)
	}
}

// CandidateDestinationFiles returns likely locations for the baked destination file.
func CandidateDestinationFiles(exeDir, configDir string) []string {
	var out []string

	if v := strings.TrimSpace(os.Getenv("KNOCKOUT_DESTINATION_FILE")); v != "" {
		out = append(out, v)
	}

	if configDir != "" {
		out = append(out, filepath.Join(configDir, DestinationName))
	}

	if exeDir != "" {
		out = append(out, filepath.Join(exeDir, DestinationName))
		out = append(out, filepath.Join(exeDir, "resources", DestinationName))
	}

	if home, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(home, DestinationName))
	}

	return out
}

// FindDestinationFile returns the first existing candidate destination file.
func FindDestinationFile(exeDir, configDir string) (string, error) {
	for _, p := range CandidateDestinationFiles(exeDir, configDir) {
		st, err := os.Stat(p)
		if err == nil && !st.IsDir() {
			return p, nil
		}
	}

	return "", errors.New("no knockout destination file found; set KNOCKOUT_DESTINATION_FILE or place knockout-destination.json next to the installer")
}
