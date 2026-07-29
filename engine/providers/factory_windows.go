//go:build windows

package providers

func NewAutoTuneProvider(binDir, luaDir, listDir, engineSHA256 string) BypassProvider {
	return NewZapret2WindowsProvider(binDir, luaDir, listDir, engineSHA256, true, false)
}
