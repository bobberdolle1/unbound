//go:build linux

package providers

func NewAutoTuneProvider(binDir, luaDir, listDir string) BypassProvider {
	// Linux typically uses nfqws or tpws from Zapret. 
	// For now, return a dummy or a generic provider if available.
	return nil 
}
