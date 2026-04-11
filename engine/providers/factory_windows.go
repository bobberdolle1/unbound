//go:build windows

package providers

func NewAutoTuneProvider(binDir, luaDir, listDir string) BypassProvider {
	return NewZapret2WindowsProvider(binDir, luaDir, listDir, true, false)
}
