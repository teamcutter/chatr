package registry

import "syscall"

func macOSProductVersion() string {
	v, err := syscall.Sysctl("kern.osproductversion")
	if err != nil {
		return ""
	}
	return v
}
