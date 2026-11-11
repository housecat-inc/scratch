package app

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"

	"github.com/housecat-inc/scratch/ui/dist"
)

var Version = func() string {
	h := sha256.New()
	for _, name := range []string{"ui.css", "ui.js"} {
		b, _ := fs.ReadFile(dist.FS, name)
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}()
