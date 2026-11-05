//go:build !windows

package bootstrap

import "os"

func secureDirectory(path string) error { return os.Chmod(path, 0700) }
