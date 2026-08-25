package cli

import (
	"runtime/debug"
	"testing"
)

func TestBuildVersion(t *testing.T) {
	tests := []struct {
		name     string
		fallback string
		info     *debug.BuildInfo
		want     string
	}{
		{
			name:     "release stamp overrides module metadata",
			fallback: "v0.1.0",
			info:     &debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-pseudo"}},
			want:     "v0.1.0",
		},
		{
			name:     "development build uses module metadata",
			fallback: devVersion,
			info:     &debug.BuildInfo{Main: debug.Module{Version: "v0.2.0"}},
			want:     "v0.2.0",
		},
		{
			name:     "empty fallback remains development version",
			fallback: "",
			info:     &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			want:     devVersion,
		},
		{
			name:     "dirty release stamp is visible",
			fallback: "v0.1.0",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "v0.0.0-pseudo"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.modified", Value: "true"},
				},
			},
			want: "v0.1.0+dirty",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := buildVersion(func() (*debug.BuildInfo, bool) {
				return testCase.info, testCase.info != nil
			}, testCase.fallback)
			if got.Version != testCase.want {
				t.Fatalf("Version = %q, want %q", got.Version, testCase.want)
			}
		})
	}
}
