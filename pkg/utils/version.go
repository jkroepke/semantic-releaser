package utils

import (
	"github.com/Masterminds/semver/v3"
	cc "github.com/leodido/go-conventionalcommits"
)

func IncrementSemVerVersion(version *semver.Version, bump cc.VersionBump) semver.Version {
	switch bump {
	case cc.MajorVersion:
		return version.IncMajor()
	case cc.MinorVersion:
		return version.IncMinor()
	case cc.PatchVersion:
		return version.IncPatch()
	case cc.UnknownVersion:
		fallthrough
	default:
		return *version
	}
}
