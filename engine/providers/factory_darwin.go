//go:build darwin

package providers

// NewAutoTuneProvider builds the provider the auto-tuner drives while it
// benchmarks profiles.
//
// binDir is a directory; the provider now takes the executable's full path, so
// resolve it here rather than constructing a provider that can never start.
func NewAutoTuneProvider(binDir, luaDir, listDir, _ string) BypassProvider {
	binPath, err := ResolveEngineBinary(MacOSEngineBinary, binDir)
	if err != nil {
		return nil
	}
	return NewZapretMacOSProvider(binPath)
}
