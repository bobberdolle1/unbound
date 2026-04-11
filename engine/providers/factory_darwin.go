//go:build darwin

package providers

func NewAutoTuneProvider(binDir, luaDir, listDir string) BypassProvider {
	return NewZapretMacOSProvider(binDir)
}
