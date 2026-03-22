package engine

import (
	"testing"
)

func TestGetBlob(t *testing.T) {
	testCases := []BlobType{
		BlobTypeTLSDefault,
		BlobTypeQUICDefault,
		BlobTypeHTTPRequest,
	}
	
	for _, blobType := range testCases {
		data, err := GetBlob(blobType)
		
		if err != nil {
			t.Errorf("Failed to get blob %s: %v", blobType, err)
		}
		
		if len(data) == 0 {
			t.Errorf("Blob %s has no data", blobType)
		}
		
		t.Logf("Blob %s: %d bytes", blobType, len(data))
	}
}

func TestGenerateRandomTLSClientHello(t *testing.T) {
	testCases := []struct {
		name string
		sni  string
	}{
		{"No SNI", ""},
		{"With SNI", "example.com"},
		{"Long SNI", "very.long.subdomain.example.com"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := GenerateRandomTLSClientHello(tc.sni)
			
			if len(data) == 0 {
				t.Error("Generated TLS ClientHello is empty")
			}
			
			if data[0] != 0x16 {
				t.Error("Invalid TLS record type")
			}
			
			if data[1] != 0x03 {
				t.Error("Invalid TLS version major")
			}
			
			t.Logf("Generated TLS ClientHello: %d bytes", len(data))
		})
	}
}

func TestGenerateRandomQUICInitial(t *testing.T) {
	data := GenerateRandomQUICInitial()
	
	if len(data) == 0 {
		t.Error("Generated QUIC Initial is empty")
	}
	
	if data[0]&0xc0 != 0xc0 {
		t.Error("Invalid QUIC long header")
	}
	
	t.Logf("Generated QUIC Initial: %d bytes", len(data))
}

func TestRegisterCustomBlob(t *testing.T) {
	customType := BlobType("custom_test")
	customData := []byte("test data")
	
	RegisterCustomBlob(customType, customData, "Test blob")
	
	data, err := GetBlob(customType)
	if err != nil {
		t.Errorf("Failed to get custom blob: %v", err)
	}
	
	if string(data) != string(customData) {
		t.Errorf("Custom blob data mismatch: expected %s, got %s", customData, data)
	}
}

func TestListBlobs(t *testing.T) {
	blobs := ListBlobs()
	
	if len(blobs) == 0 {
		t.Error("No blobs registered")
	}
	
	t.Logf("Total blobs: %d", len(blobs))
	
	for _, blob := range blobs {
		if blob.Type == "" {
			t.Error("Blob has empty type")
		}
		if len(blob.Data) == 0 {
			t.Errorf("Blob %s has no data", blob.Type)
		}
		t.Logf("Blob: %s - %s (%d bytes)", blob.Type, blob.Description, len(blob.Data))
	}
}

func TestGenerateBlobHex(t *testing.T) {
	hex, err := GenerateBlobHex(BlobTypeTLSDefault)
	
	if err != nil {
		t.Errorf("Failed to generate hex: %v", err)
	}
	
	if len(hex) == 0 {
		t.Error("Generated hex is empty")
	}
	
	if len(hex)%2 != 0 {
		t.Error("Hex string has odd length")
	}
	
	t.Logf("Hex length: %d characters", len(hex))
}

func TestModifyTLSSessionID(t *testing.T) {
	original := GenerateRandomTLSClientHello("")
	newSessionID := make([]byte, 32)
	for i := range newSessionID {
		newSessionID[i] = byte(i)
	}
	
	modified := ModifyTLSSessionID(original, newSessionID)
	
	if len(modified) != len(original) {
		t.Error("Modified ClientHello has different length")
	}
	
	for i := 0; i < 32; i++ {
		if modified[44+i] != byte(i) {
			t.Errorf("Session ID not modified correctly at position %d", i)
		}
	}
}

func TestDuplicateTLSSessionID(t *testing.T) {
	original := GenerateRandomTLSClientHello("")
	
	duplicated := DuplicateTLSSessionID(original)
	
	if len(duplicated) != len(original) {
		t.Error("Duplicated ClientHello has different length")
	}
	
	for i := 0; i < 32; i++ {
		if duplicated[44+i] != original[44+i] {
			t.Error("Session ID not duplicated correctly")
			break
		}
	}
}
