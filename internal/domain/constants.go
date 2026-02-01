package domain

var extensions = []string{".tar.gz", ".tar.zst", ".tar.xz", ".tar.bz2", ".tgz", ".txz", ".tzst"}

func Extensions() []string {
	return append([]string{}, extensions...)
}
