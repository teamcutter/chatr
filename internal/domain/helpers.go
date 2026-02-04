package domain

func formatVersion(version, revision string) string {
	if revision != "0" && revision != "" {
		return version + "_" + revision
	}
	return version
}
