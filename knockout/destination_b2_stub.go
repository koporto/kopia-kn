//go:build no_extra_providers

package knockout

import (
	"github.com/pkg/errors"

	"github.com/kopia/kopia/repo/blob"
)

func b2ConnectionInfo(_ *Destination, _ string) (blob.ConnectionInfo, error) {
	return blob.ConnectionInfo{}, errors.New("B2 storage was not included in this build; use type s3 with a Backblaze S3-compatible endpoint")
}
