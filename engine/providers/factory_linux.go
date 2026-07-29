//go:build linux

package providers

// NewAutoTuneProvider builds the provider the auto-tuner drives while it
// benchmarks profiles.
//
// This used to `return nil`, so App.AutoTune() on Linux handed a nil
// BypassProvider to the tuner and the auto-tune button panicked with a nil
// pointer dereference.
func NewAutoTuneProvider(binDir, luaDir, listDir, engineSHA256 string) BypassProvider {
	binPath, err := ResolveEngineBinary(LinuxEngineBinary, binDir)
	if err != nil {
		return nil
	}
	return NewZapretLinuxProvider(binPath, luaDir, engineSHA256)
}
