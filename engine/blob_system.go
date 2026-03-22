package engine

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

type BlobType string

const (
	BlobTypeTLSDefault  BlobType = "fake_default_tls"
	BlobTypeTLSRandom   BlobType = "fake_random_tls"
	BlobTypeQUICDefault BlobType = "fake_default_quic"
	BlobTypeQUICRandom  BlobType = "fake_random_quic"
	BlobTypeQUICInitial BlobType = "fake_quic_initial"
	BlobTypeHTTPRequest BlobType = "fake_http_request"
)

type BlobPayload struct {
	Type        BlobType
	Data        []byte
	Description string
}

var blobRegistry = make(map[BlobType]BlobPayload)

func init() {
	registerDefaultBlobs()
}

func registerDefaultBlobs() {
	blobRegistry[BlobTypeTLSDefault] = BlobPayload{
		Type:        BlobTypeTLSDefault,
		Data:        generateDefaultTLSClientHello(),
		Description: "Standard TLS 1.3 ClientHello with common extensions",
	}

	blobRegistry[BlobTypeQUICDefault] = BlobPayload{
		Type:        BlobTypeQUICDefault,
		Data:        generateDefaultQUICInitial(),
		Description: "Standard QUIC Initial packet",
	}

	blobRegistry[BlobTypeHTTPRequest] = BlobPayload{
		Type:        BlobTypeHTTPRequest,
		Data:        generateDefaultHTTPRequest(),
		Description: "Standard HTTP/1.1 GET request",
	}
}

func generateDefaultTLSClientHello() []byte {
	return []byte{
		0x16, 0x03, 0x01, 0x02, 0x00, 0x01, 0x00, 0x01, 0xfc, 0x03, 0x03,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
		0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
		0x20,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
		0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
		0x00, 0x20,
		0x13, 0x01, 0x13, 0x02, 0x13, 0x03, 0xc0, 0x2c, 0xc0, 0x30, 0x00, 0x9f, 0xcc, 0xa9, 0xcc, 0xa8,
		0xcc, 0xaa, 0xc0, 0x2b, 0xc0, 0x2f, 0x00, 0x9e, 0xc0, 0x24, 0xc0, 0x28, 0x00, 0x6b, 0xc0, 0x23,
		0x01, 0x00, 0x01, 0x93,
		0x00, 0x00, 0x00, 0x12, 0x00, 0x10, 0x00, 0x00, 0x0d, 0x65, 0x78, 0x61, 0x6d, 0x70, 0x6c, 0x65,
		0x2e, 0x63, 0x6f, 0x6d,
		0x00, 0x0b, 0x00, 0x04, 0x03, 0x00, 0x01, 0x02,
		0x00, 0x0a, 0x00, 0x0c, 0x00, 0x0a, 0x00, 0x1d, 0x00, 0x17, 0x00, 0x1e, 0x00, 0x19, 0x00, 0x18,
		0x00, 0x23, 0x00, 0x00,
		0x00, 0x16, 0x00, 0x00,
		0x00, 0x17, 0x00, 0x00,
		0x00, 0x0d, 0x00, 0x1e, 0x00, 0x1c, 0x04, 0x03, 0x05, 0x03, 0x06, 0x03, 0x08, 0x07, 0x08, 0x08,
		0x08, 0x09, 0x08, 0x0a, 0x08, 0x0b, 0x08, 0x04, 0x08, 0x05, 0x08, 0x06, 0x04, 0x01, 0x05, 0x01,
		0x06, 0x01,
	}
}

func generateDefaultQUICInitial() []byte {
	return []byte{
		0xc0, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00,
		0x41, 0x00,
		0x06, 0x00, 0x40, 0x5a, 0x02, 0x00, 0x00, 0x56, 0x03, 0x03,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
		0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
	}
}

func generateDefaultHTTPRequest() []byte {
	return []byte("GET / HTTP/1.1\r\nHost: example.com\r\nUser-Agent: Mozilla/5.0\r\nAccept: */*\r\n\r\n")
}

func GenerateRandomTLSClientHello(sni string) []byte {
	random := make([]byte, 32)
	rand.Read(random)

	sessionID := make([]byte, 32)
	rand.Read(sessionID)

	base := generateDefaultTLSClientHello()

	copy(base[11:43], random)
	copy(base[44:76], sessionID)

	if sni != "" {
		sniBytes := []byte(sni)
		sniLen := len(sniBytes)

		sniExt := make([]byte, 9+sniLen)
		sniExt[0] = 0x00
		sniExt[1] = 0x00
		sniExt[2] = byte((5 + sniLen) >> 8)
		sniExt[3] = byte((5 + sniLen) & 0xff)
		sniExt[4] = byte((3 + sniLen) >> 8)
		sniExt[5] = byte((3 + sniLen) & 0xff)
		sniExt[6] = 0x00
		sniExt[7] = byte(sniLen >> 8)
		sniExt[8] = byte(sniLen & 0xff)
		copy(sniExt[9:], sniBytes)

		result := make([]byte, len(base)+len(sniExt))
		copy(result, base[:120])
		copy(result[120:], sniExt)
		copy(result[120+len(sniExt):], base[120:])

		return result
	}

	return base
}

func GenerateRandomQUICInitial() []byte {
	base := generateDefaultQUICInitial()

	if len(base) < 28 {
		return base
	}

	random := make([]byte, 32)
	rand.Read(random)

	if len(base) >= 60 {
		copy(base[28:60], random)
	}

	return base
}

func GetBlob(blobType BlobType) ([]byte, error) {
	if blob, ok := blobRegistry[blobType]; ok {
		return blob.Data, nil
	}
	return nil, fmt.Errorf("blob type not found: %s", blobType)
}

func RegisterCustomBlob(blobType BlobType, data []byte, description string) {
	blobRegistry[blobType] = BlobPayload{
		Type:        blobType,
		Data:        data,
		Description: description,
	}
}

func ListBlobs() []BlobPayload {
	blobs := make([]BlobPayload, 0, len(blobRegistry))
	for _, blob := range blobRegistry {
		blobs = append(blobs, blob)
	}
	return blobs
}

func GenerateBlobHex(blobType BlobType) (string, error) {
	data, err := GetBlob(blobType)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func ModifyTLSSessionID(clientHello []byte, newSessionID []byte) []byte {
	if len(clientHello) < 76 || len(newSessionID) != 32 {
		return clientHello
	}

	modified := make([]byte, len(clientHello))
	copy(modified, clientHello)
	copy(modified[44:76], newSessionID)

	return modified
}

func DuplicateTLSSessionID(clientHello []byte) []byte {
	if len(clientHello) < 76 {
		return clientHello
	}

	sessionID := make([]byte, 32)
	copy(sessionID, clientHello[44:76])

	return ModifyTLSSessionID(clientHello, sessionID)
}
