// Package images provides embedded background images with cycling support.
package images

import (
	"embed"
	"io/fs"
	"math/rand"
	"path"
	"sync/atomic"
)

//go:embed bg_art/*.avif
var bgFS embed.FS

// bgImage holds the name and data for a single background image.
type bgImage struct {
	name string
	data []byte
}

var (
	backgrounds []bgImage
	bgIndex     atomic.Uint64
)

// Init loads all embedded background images.
func Init() error {
	entries, err := fs.ReadDir(bgFS, "bg_art")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		data, err := bgFS.ReadFile(path.Join("bg_art", name))
		if err != nil {
			return err
		}
		backgrounds = append(backgrounds, bgImage{name: name, data: data})
	}

	// Shuffle for initial randomness
	rand.Shuffle(len(backgrounds), func(i, j int) {
		backgrounds[i], backgrounds[j] = backgrounds[j], backgrounds[i]
	})

	return nil
}

// Count returns the number of available background images.
func Count() int {
	return len(backgrounds)
}

// Names returns all available background image names.
func Names() []string {
	names := make([]string, len(backgrounds))
	for i, bg := range backgrounds {
		names[i] = bg.name
	}
	return names
}

// Get returns the path and data for a background image by name.
// Returns empty values if not found.
func Get(name string) (path string, data []byte) {
	for _, bg := range backgrounds {
		if bg.name == name {
			return "/bg/" + bg.name, bg.data
		}
	}
	return "", nil
}

// Next returns the path for the next background image in the cycle.
// Thread-safe and wraps around automatically.
func Next() string {
	if len(backgrounds) == 0 {
		return ""
	}
	idx := bgIndex.Add(1) - 1 // get previous value before add
	return "/bg/" + backgrounds[idx%uint64(len(backgrounds))].name
}

// Random returns the path for a random background image.
func Random() string {
	if len(backgrounds) == 0 {
		return ""
	}
	idx := rand.Intn(len(backgrounds))
	return "/bg/" + backgrounds[idx].name
}

// Current returns the path for the current background image (without advancing).
func Current() string {
	if len(backgrounds) == 0 {
		return ""
	}
	idx := bgIndex.Load()
	return "/bg/" + backgrounds[idx%uint64(len(backgrounds))].name
}

// Data returns the image data for a given filename.
// Returns nil if not found.
func Data(name string) []byte {
	for _, bg := range backgrounds {
		if bg.name == name {
			return bg.data
		}
	}
	return nil
}
