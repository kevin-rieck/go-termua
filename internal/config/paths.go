package config

import (
	"os"
	"path/filepath"
)

// PathOptions controls where local, non-secret configuration is stored.
type PathOptions struct {
	Portable bool
}

// Paths are local storage locations used by the app.
type Paths struct {
	ConfigDir string
	CacheDir  string
}

// ResolvePaths returns OS-standard paths by default, or paths next to the
// executable when portable mode is requested.
func ResolvePaths(opts PathOptions) (Paths, error) {
	if opts.Portable {
		exe, err := os.Executable()
		if err != nil {
			return Paths{}, err
		}
		base := filepath.Dir(exe)
		return Paths{
			ConfigDir: filepath.Join(base, "termua-config"),
			CacheDir:  filepath.Join(base, "termua-cache"),
		}, nil
	}

	configRoot, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		cacheRoot = configRoot
	}

	return Paths{
		ConfigDir: filepath.Join(configRoot, "termua"),
		CacheDir:  filepath.Join(cacheRoot, "termua"),
	}, nil
}
