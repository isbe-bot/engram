package citations

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
)

func ID(kind, path string) string {
	kind = strings.TrimSpace(strings.ToLower(kind))
	path = strings.TrimSpace(path)
	sum := sha1.Sum([]byte(kind + "|" + path))
	return fmt.Sprintf("cit-%s", hex.EncodeToString(sum[:8]))
}

func Make(kind, path string) map[string]any {
	return map[string]any{
		"id":   ID(kind, path),
		"kind": strings.TrimSpace(strings.ToLower(kind)),
		"path": strings.TrimSpace(path),
	}
}
