//go:build !darwin

package registry

func macOSProductVersion() string { return "" }
