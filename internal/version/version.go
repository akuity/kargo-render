package version

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"
)

var (
	version   = ""                     // Injected with a linker flag
	buildDate = "1970-01-01T00:00:00Z" // Injected with a linker flag
)

// Version encapsulates all available information about the source code and the
// build.
type Version struct {
	// Version is a human-friendly version string.
	Version string
	// BuildDate is the date/time on which the application was built.
	BuildDate time.Time
	// GitCommitDate is the date of the last commit to the application's source
	// code that is included in this build.
	GitCommitDate time.Time
	// GitCommit is the ID (sha) of the last commit to the application's source
	// code that is included in this build.
	GitCommit string
	// GitTreeDirty is true if the application's source code contained
	// uncommitted changes at the time it was built; otherwise it is false.
	GitTreeDirty bool
	// GoVersion is the version of Go that was used to build the application.
	GoVersion string
	// Compiler indicates what Go compiler was used for the build.
	Compiler string
	// Platform indicates the OS and CPU architecture for which the application
	// was built.
	Platform string
}

var ver Version

func init() {
	ver = Version{
		GoVersion: runtime.Version(),
		Compiler:  runtime.Compiler,
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	var err error
	if ver.BuildDate, err =
		time.Parse(time.RFC3339, buildDate); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("Build info not found")
		os.Exit(1)
	}
	for _, setting := range buildInfo.Settings {
		switch setting.Key {
		case "vcs.modified":
			if ver.GitTreeDirty, err = strconv.ParseBool(setting.Value); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		case "vcs.revision":
			ver.GitCommit = setting.Value
		case "vcs.time":
			if ver.GitCommitDate, err =
				time.Parse(time.RFC3339, setting.Value); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	}

	// If we're missing the version string or commit info, or if the tree is
	// dirty, dynamically formulate a version string from available info...
	if version == "" || ver.GitCommit == "" || ver.GitTreeDirty {
		// Override whatever version string we started with
		version = "devel"
		// Tack on commit info
		if len(ver.GitCommit) >= 7 {
			version = fmt.Sprintf("%s+%s", version, ver.GitCommit[0:7])
		} else {
			version = fmt.Sprintf("%s+unknown", version)
		}
		// Indicate if the tree was dirty
		if ver.GitTreeDirty {
			version = fmt.Sprintf("%s.dirty", version)
		}
	}

	ver.Version = version
}

func GetVersion() Version {
	return ver
}
