package domain

var extensions = []string{".tar.gz", ".tar.zst", ".tar.xz", ".tar.bz2", ".tgz", ".txz", ".tzst", ".zip", ".dmg", ".pkg"}

func Extensions() []string {
	return append([]string{}, extensions...)
}
