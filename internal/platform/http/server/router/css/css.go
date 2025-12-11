// package css provides a hashed CSS file path and data.
package css

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
)

//go:embed output.css
var cssFS embed.FS

var (
	cssPath string // e.g. "/output.abcd1234.css"
	cssData []byte
)

func Init() error {
	data, err := cssFS.ReadFile("output.css")
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:8]) // shorten if needed

	cssData = data
	cssPath = "/output." + hash + ".css"
	return nil
}

func Path() string {
	return cssPath
}

func Data() []byte {
	return cssData
}
