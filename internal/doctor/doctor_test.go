package doctor

import "testing"

func TestPkgManagerFor(t *testing.T) {
	cases := map[string]string{
		// debian family
		`ID=debian
NAME="Debian GNU/Linux"`: "apt",
		`ID=ubuntu
ID_LIKE=debian`: "apt",
		`ID=raspbian`: "apt",
		// suse family (must beat the fedora checks)
		`ID=opensuse-tumbleweed
ID_LIKE="opensuse suse"`: "zypper",
		`ID=SLES`: "zypper",
		// the fedora/red hat family, by ID and not just ID_LIKE
		`ID=fedora`: "dnf",
		`ID=rhel
ID_LIKE="fedora"`: "dnf",
		`ID=centos`: "dnf",
		`ID=rocky
ID_LIKE="rhel centos fedora"`: "dnf",
		`ID=almalinux`: "dnf",
		`ID=oracle`:    "dnf",
		// arch family
		`ID=arch`: "pacman",
		`ID=manjaro
ID_LIKE=arch`: "pacman",
		// alpine
		`ID=alpine`: "apk",
		// unknown
		`ID=voidlinux`: "",
	}
	for body, want := range cases {
		if got := pkgManagerFor(body); got != want {
			t.Errorf("pkgManagerFor(%q) = %q, want %q", body, got, want)
		}
	}
}
