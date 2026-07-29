//go:build windows

package engine

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	programData, err := os.MkdirTemp("", "unbound-test-programdata-")
	if err != nil {
		panic(err)
	}
	systemProgramDataPath = func() (string, error) { return programData, nil }
	code := m.Run()
	_ = os.RemoveAll(programData)
	os.Exit(code)
}
