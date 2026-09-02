package knockout_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kopia/kopia/knockout"
)

func TestNormalizeSiteID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in      string
		want    string
		wantErr string
	}{
		{in: "Acme-HQ", want: "acme-hq"},
		{in: " site_01 ", want: "site_01"},
		{in: "a", wantErr: "between"},
		{in: "", wantErr: "required"},
		{in: "bad id", wantErr: "may contain"},
		{in: strings.Repeat("a", 65), wantErr: "between"},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()

			got, err := knockout.NormalizeSiteID(tc.in)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestValidatePasscode(t *testing.T) {
	t.Parallel()

	require.Error(t, knockout.ValidatePasscode(""))
	require.Error(t, knockout.ValidatePasscode("short"))
	require.Error(t, knockout.ValidatePasscode(" leading8"))
	require.NoError(t, knockout.ValidatePasscode("s3cret!!"))
	require.Len(t, knockout.PasscodeSHA256("s3cret!!"), 64)
}

func TestSitePrefix(t *testing.T) {
	t.Parallel()

	require.Equal(t, "sites/acme/", knockout.SitePrefix("", "acme"))
	require.Equal(t, "clients/acme/", knockout.SitePrefix("clients", "acme"))
	require.Equal(t, "kn/acme/", knockout.SitePrefix("kn/", "acme"))
}
