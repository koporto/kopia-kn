package knockout

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"

	"github.com/pkg/errors"
)

const (
	minSiteIDLength   = 2
	maxSiteIDLength   = 64
	minPasscodeLength = 8
	maxPasscodeLength = 256
)

var siteIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// NormalizeSiteID trims and lowercases a site ID used as a storage prefix.
func NormalizeSiteID(siteID string) (string, error) {
	id := strings.TrimSpace(siteID)
	if id == "" {
		return "", errors.New("site ID is required")
	}

	if len(id) < minSiteIDLength || len(id) > maxSiteIDLength {
		return "", errors.Errorf("site ID must be between %d and %d characters", minSiteIDLength, maxSiteIDLength)
	}

	if !siteIDPattern.MatchString(id) {
		return "", errors.New("site ID may contain only letters, numbers, dots, hyphens, and underscores")
	}

	return strings.ToLower(id), nil
}

// ValidatePasscode checks the first-pass enrollment secret.
func ValidatePasscode(passcode string) error {
	if strings.TrimSpace(passcode) == "" {
		return errors.New("passcode is required")
	}

	if strings.TrimSpace(passcode) != passcode {
		return errors.New("passcode must not start or end with whitespace")
	}

	if len(passcode) < minPasscodeLength {
		return errors.Errorf("passcode must be at least %d characters", minPasscodeLength)
	}

	if len(passcode) > maxPasscodeLength {
		return errors.Errorf("passcode must be at most %d characters", maxPasscodeLength)
	}

	for _, r := range passcode {
		if unicode.IsControl(r) {
			return errors.New("passcode must not contain control characters")
		}
	}

	return nil
}

// PasscodeSHA256 returns the lowercase hex SHA-256 of passcode.
func PasscodeSHA256(passcode string) string {
	sum := sha256.Sum256([]byte(passcode))
	return hex.EncodeToString(sum[:])
}

// SitePrefix returns the object-store prefix for a site, always slash-terminated.
func SitePrefix(prefixBase, siteID string) string {
	base := strings.TrimSpace(prefixBase)
	if base == "" {
		base = "sites/"
	}

	if !strings.HasSuffix(base, "/") {
		base += "/"
	}

	return base + siteID + "/"
}
