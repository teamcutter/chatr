package registry

import (
	"runtime"
	"strconv"
	"strings"
	"sync"
)

var macOSReleases = []struct {
	major int
	tag   string
}{
	{27, "golden_gate"},
	{26, "tahoe"},
	{15, "sequoia"},
	{14, "sonoma"},
	{13, "ventura"},
	{12, "monterey"},
	{11, "big_sur"},
}

var hostMacOSMajor = sync.OnceValue(func() int {
	return parseMacOSMajor(macOSProductVersion())
})

func parseMacOSMajor(version string) int {
	major, rest, _ := strings.Cut(strings.TrimSpace(version), ".")
	n, err := strconv.Atoi(major)
	if err != nil {
		return 0
	}
	switch {
	case n == 10 && strings.HasPrefix(rest, "16"):
		return 11
	case n >= 16 && n <= 25:
		return n + 10
	}
	return n
}

func bottleTags(goos, goarch string, macMajor int) []string {
	var tags []string
	switch goos {
	case "darwin":
		prefix := ""
		if goarch == "arm64" {
			prefix = "arm64_"
		}
		for _, r := range macOSReleases {
			if macMajor == 0 || r.major <= macMajor {
				tags = append(tags, prefix+r.tag)
			}
		}
	case "linux":
		switch goarch {
		case "amd64":
			tags = []string{"x86_64_linux"}
		case "arm64":
			tags = []string{"arm64_linux"}
		}
	}
	return append(tags, "all")
}

func getPlatformCandidates() []string {
	return bottleTags(runtime.GOOS, runtime.GOARCH, hostMacOSMajor())
}

func platformName() string {
	tags := getPlatformCandidates()
	if len(tags) > 1 {
		return tags[0]
	}
	return runtime.GOOS + "/" + runtime.GOARCH
}
