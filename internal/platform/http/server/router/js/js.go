package js

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
)

//go:embed utils.js
var utilsFS embed.FS

var (
	utilsPath string // e.g. "/utils.abcd1234.js"
	utilsData []byte
)

func Init() error {
	data, err := utilsFS.ReadFile("utils.js")
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:8]) // shorten if needed

	utilsData = data
	utilsPath = "/utils." + hash + ".js"
	return nil
}

func Path() string {
	return utilsPath
}

func Data() []byte {
	return utilsData
}
