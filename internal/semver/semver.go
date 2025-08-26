package semver

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse 解析版本字符串，支持 "v1.2.3" 或 "1.2.3" 格式
func Parse(version string) Version {
	version = strings.TrimPrefix(version, "v")
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return Version{}
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	patch, _ := strconv.Atoi(parts[2])
	return Version{
		Major: major,
		Minor: minor,
		Patch: patch,
	}
}

// Version 表示一个语义版本
type Version struct {
	Major int
	Minor int
	Patch int
}

func (v Version) GreaterEqual(other Version) bool {
	if v.Major != other.Major {
		return v.Major > other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor > other.Minor
	}
	return v.Patch >= other.Patch
}

func (v Version) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}
