package server

import (
	"os"
	"runtime"
	"strings"
)

// HostArch returns the architecture of the supervisor host.
// It defaults to runtime.GOARCH, but can be overridden via SUPERVISOR_HOST_ARCH.
func HostArch() string {
	if arch := strings.TrimSpace(os.Getenv("SUPERVISOR_HOST_ARCH")); arch != "" {
		return arch
	}
	return runtime.GOARCH
}

// HostOS returns the operating system of the supervisor host.
// It defaults to runtime.GOOS, but can be overridden via SUPERVISOR_HOST_OS.
func HostOS() string {
	if osName := strings.TrimSpace(os.Getenv("SUPERVISOR_HOST_OS")); osName != "" {
		return osName
	}
	return runtime.GOOS
}
