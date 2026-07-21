package httpapi

import "strings"

var (
	buildVersion = "0.1.0"
	buildCommit  = "dev"
	buildDate    = "unknown"
)

type BuildInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

func currentBuildInfo() BuildInfo {
	return BuildInfo{
		Version: firstNonBlank(strings.TrimSpace(buildVersion), "0.1.0"),
		Commit:  firstNonBlank(strings.TrimSpace(buildCommit), "dev"),
		Date:    firstNonBlank(strings.TrimSpace(buildDate), "unknown"),
	}
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
