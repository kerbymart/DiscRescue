package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func hash16(data []byte) [16]byte {
	sum := sha256.Sum256(data)
	var out [16]byte
	copy(out[:], sum[:16])
	return out
}
func verifyDigest(data []byte, digest Digest) (bool, error) {
	switch digest.Provider {
	case ProviderSHA256:
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:]) == strings.ToLower(digest.Value), nil
	case ProviderNone:
		return false, fmt.Errorf("verify digest: provider none is not verifiable")
	default:
		return false, fmt.Errorf("verify digest: unsupported provider %q", digest.Provider)
	}
}
