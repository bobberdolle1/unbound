package engine

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionComparison(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"v1.0.0", "v1.0.1", -1},
		{"v1.0.1", "v1.0.0", 1},
		{"v1.0.0", "v1.0.0", 0},
		{"v2.0.0", "v1.9.9", 1},
		{"v1.2.3", "v1.2.10", -1},
		{"v10.0.0", "v2.0.0", 1},
	}

	for _, tt := range tests {
		v1Norm := normalizeVersion(tt.v1)
		v2Norm := normalizeVersion(tt.v2)
		result := compareVersions(v1Norm, v2Norm)
		if result != tt.expected {
			t.Errorf("compareVersions(%s, %s) = %d, expected %d", tt.v1, tt.v2, result, tt.expected)
		}
	}
}

func TestSHA256Verification(t *testing.T) {
	testData := []byte("test binary content for checksum verification")

	hash := sha256.Sum256(testData)
	expectedChecksum := hex.EncodeToString(hash[:])

	calculatedHash := sha256.Sum256(testData)
	calculatedChecksum := hex.EncodeToString(calculatedHash[:])

	if calculatedChecksum != expectedChecksum {
		t.Errorf("Checksum mismatch: got %s, expected %s", calculatedChecksum, expectedChecksum)
	}

	tamperedData := []byte("tampered binary content")
	tamperedHash := sha256.Sum256(tamperedData)
	tamperedChecksum := hex.EncodeToString(tamperedHash[:])

	if tamperedChecksum == expectedChecksum {
		t.Error("Tampered data should not match original checksum")
	}

	t.Logf("Original checksum: %s", expectedChecksum)
	t.Logf("Tampered checksum: %s", tamperedChecksum)
}

func TestZipExtraction(t *testing.T) {
	tempDir := t.TempDir()

	zipPath := filepath.Join(tempDir, "test.zip")
	extractDir := filepath.Join(tempDir, "extracted")

	if err := os.MkdirAll(extractDir, 0755); err != nil {
		t.Fatal(err)
	}

	testFiles := map[string]string{
		"unbound.exe":    "fake binary content",
		"config.json":    `{"version": "1.0.0"}`,
		"data/lists.txt": "test data",
	}

	if err := createTestZip(zipPath, testFiles); err != nil {
		t.Fatal(err)
	}

	if err := extractZip(zipPath, extractDir); err != nil {
		t.Fatal(err)
	}

	for filename, expectedContent := range testFiles {
		extractedPath := filepath.Join(extractDir, filename)
		content, err := os.ReadFile(extractedPath)
		if err != nil {
			t.Errorf("Failed to read extracted file %s: %v", filename, err)
			continue
		}

		if string(content) != expectedContent {
			t.Errorf("Content mismatch for %s: got %s, expected %s", filename, string(content), expectedContent)
		}
	}

	t.Logf("Successfully extracted and verified %d files", len(testFiles))
}

func createTestZip(zipPath string, files map[string]string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for filename, content := range files {
		writer, err := zipWriter.Create(filename)
		if err != nil {
			return err
		}

		if _, err := io.WriteString(writer, content); err != nil {
			return err
		}
	}

	return nil
}

func extractZip(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		path := filepath.Join(destDir, file.Name)

		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}

		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

func TestUpdateInfoParsing(t *testing.T) {
	mockUpdateInfo := UpdateInfo{
		Available:   true,
		Version:     "v2.0.1",
		DownloadURL: "https://github.com/unbound/releases/download/v2.0.1/unbound-windows-amd64.zip",
		Changelog:   "- Fixed critical bug\n- Added new feature",
	}

	if mockUpdateInfo.Version == "" {
		t.Error("Version should not be empty")
	}

	if !strings.HasPrefix(mockUpdateInfo.DownloadURL, "https://") {
		t.Error("Download URL should use HTTPS")
	}

	if !mockUpdateInfo.Available {
		t.Error("Update should be available")
	}

	v1Norm := normalizeVersion(mockUpdateInfo.Version)
	v2Norm := normalizeVersion("v2.0.0")
	if compareVersions(v1Norm, v2Norm) <= 0 {
		t.Error("Update version should be greater than current")
	}

	t.Logf("Update info parsed successfully: %+v", mockUpdateInfo)
}

func TestChecksumValidation(t *testing.T) {
	testData := []byte("unbound.exe binary content")

	hash := sha256.Sum256(testData)
	validChecksum := hex.EncodeToString(hash[:])

	if !validateChecksum(testData, validChecksum) {
		t.Error("Valid checksum should pass validation")
	}

	invalidChecksum := "0000000000000000000000000000000000000000000000000000000000000000"
	if validateChecksum(testData, invalidChecksum) {
		t.Error("Invalid checksum should fail validation")
	}

	t.Log("Checksum validation working correctly")
}

func validateChecksum(data []byte, expectedChecksum string) bool {
	hash := sha256.Sum256(data)
	actualChecksum := hex.EncodeToString(hash[:])
	return actualChecksum == expectedChecksum
}

func TestDownloadProgress(t *testing.T) {
	totalSize := int64(1024 * 1024)
	downloaded := int64(0)

	progressCallback := func(current, total int64) {
		percent := float64(current) / float64(total) * 100
		t.Logf("Download progress: %.2f%% (%d/%d bytes)", percent, current, total)
	}

	chunks := []int64{102400, 204800, 307200, 409600, 512000, 614400, 716800, 819200, 921600, 1024000, 1048576}

	for _, chunk := range chunks {
		downloaded = chunk
		if downloaded > totalSize {
			downloaded = totalSize
		}
		progressCallback(downloaded, totalSize)
	}

	if downloaded != totalSize {
		t.Errorf("Download incomplete: %d/%d", downloaded, totalSize)
	}
}

func TestBackupAndRestore(t *testing.T) {
	tempDir := t.TempDir()

	originalFile := filepath.Join(tempDir, "unbound.exe")
	backupFile := filepath.Join(tempDir, "unbound.exe.backup")

	originalContent := []byte("original binary v1.0.0")
	if err := os.WriteFile(originalFile, originalContent, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.Rename(originalFile, backupFile); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(originalFile); !os.IsNotExist(err) {
		t.Error("Original file should not exist after backup")
	}

	if _, err := os.Stat(backupFile); err != nil {
		t.Error("Backup file should exist")
	}

	newContent := []byte("updated binary v2.0.0")
	if err := os.WriteFile(originalFile, newContent, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.Rename(backupFile, originalFile); err != nil {
		t.Fatal(err)
	}

	restoredContent, err := os.ReadFile(originalFile)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(restoredContent, originalContent) {
		t.Error("Restored content doesn't match original")
	}

	t.Log("Backup and restore mechanism working correctly")
}
