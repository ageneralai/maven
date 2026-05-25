package version

import (
	"fmt"
	"runtime/debug"
	"strings"
)

const SDKModule = "github.com/ageneralai/ageneral-agents-go"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

type Info struct {
	Version string
	Commit  string
	Date    string
	SDK     string
	Go      string
}

func Load() Info {
	out := Info{Version: Version, Commit: Commit, Date: Date, SDK: "unknown", Go: "unknown"}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return out
	}
	out.Go = bi.GoVersion
	for _, dep := range bi.Deps {
		if dep.Path == SDKModule {
			out.SDK = moduleVersion(dep)
			break
		}
	}
	return out
}

func moduleVersion(dep *debug.Module) string {
	if dep == nil {
		return "unknown"
	}
	if dep.Replace != nil {
		rev := strings.TrimSpace(dep.Replace.Version)
		if rev != "" {
			return rev
		}
		return "(local)"
	}
	rev := strings.TrimSpace(dep.Version)
	if rev != "" {
		return rev
	}
	return "unknown"
}

func (i Info) Lines() []string {
	return []string{
		"Build:",
		fmt.Sprintf("  maven  %s (%s, %s)", i.Version, i.Commit, i.Date),
		fmt.Sprintf("  sdk    %s %s", SDKModule, i.SDK),
		fmt.Sprintf("  go     %s", i.Go),
	}
}
