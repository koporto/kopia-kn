//go:build !no_extra_providers

package knockout

import (
	"github.com/kopia/kopia/repo/blob"
	"github.com/kopia/kopia/repo/blob/b2"
)

func b2ConnectionInfo(d *Destination, prefix string) (blob.ConnectionInfo, error) {
	return blob.ConnectionInfo{
		Type: StorageB2,
		Config: &b2.Options{
			BucketName: d.Bucket,
			Prefix:     prefix,
			KeyID:      d.KeyID,
			Key:        d.Key,
		},
	}, nil
}
