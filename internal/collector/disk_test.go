package collector

import "testing"

func TestIsUserFacingMount(t *testing.T) {
	cases := []struct {
		mp   string
		want bool
	}{
		{"/", true},
		{"/Volumes/ExternalDrive", true},
		{"/Users/someone/data", true},
		{"/System/Volumes/Data", false},
		{"/System/Volumes/VM", false},
		{"/System/Volumes/Preboot", false},
		{"/private/var/folders/abc", false},
		{"/private/var/vm", false},
	}
	for _, c := range cases {
		got := isUserFacingMount(c.mp)
		if got != c.want {
			t.Errorf("isUserFacingMount(%q) = %v, want %v", c.mp, got, c.want)
		}
	}
}
