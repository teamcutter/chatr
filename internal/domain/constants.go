package domain

var extensions = []string{".tar.zst", ".tar.gz", ".tar.xz", ".tar.bz2", ".tgz", ".txz", ".tzst"}

func Extensions() []string {
	return append([]string{}, extensions...)
}
