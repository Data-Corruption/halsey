// Package assets provides functionality for backing up hosting assets.
package assets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"halsey/go/storage/database"
	"halsey/go/storage/storagepath"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/go-chi/chi/v5"
)

/*
Assets stored in a directory using a 2-level scheme
e.g. `assets/83/fb/83fbcf83c1387...`
file server router handles simple conversation from `/a/<hash>.ext` to path.
*/
const ASSET_DIR = "assets"

func init() {
	mime.AddExtensionType(".mp4", "video/mp4") // just in case idk
	mime.AddExtensionType(".webm", "video/webm")
}

// handle /a/{hash} requests
func AssetFS(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// get hash from URL
		hash := chi.URLParam(r, "hash")
		if len(hash) < 4 {
			http.Error(w, "Invalid asset hash", http.StatusBadRequest)
			return
		}

		// get asset path
		storagePath := storagepath.FromContext(ctx)
		if storagePath == "" {
			http.Error(w, "Storage path not set in context", http.StatusInternalServerError)
			return
		}
		assetPath := filepath.Join(storagePath, ASSET_DIR, hash[:2], hash[2:4], hash)

		// get file
		f, err := os.Open(assetPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if fi.IsDir() {
			http.NotFound(w, r)
			return
		}

		// add headers
		w.Header().Set("Cache-Control", "max-age=31536000, public")
		w.Header().Set("Pragma", "public")

		// serve content
		http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
	}
}

func Add(ctx context.Context, filePath string) (string, error) {
	// get and ensure asset directory
	storagePath := storagepath.FromContext(ctx)
	if storagePath == "" {
		return "", fmt.Errorf("storage path not set in context")
	}
	assetDir := filepath.Join(storagePath, ASSET_DIR)
	if err := os.MkdirAll(assetDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create asset directory: %w", err)
	}

	// read data
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	// hash
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	name := hash + filepath.Ext(filePath)

	// copy file to assets directory
	assetPath := filepath.Join(assetDir, hash[:2], hash[2:4], name)
	if err := os.MkdirAll(filepath.Dir(assetPath), os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create asset subdirectory: %w", err)
	}
	if err := os.WriteFile(assetPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write asset file: %w", err)
	}

	return name, nil
}

// AddRef adds a reference to a message ID for a given hash in the assets database.
// Not really used for anything atm, but maybe in the future.
func AddRef(ctx context.Context, hash string, messageID string) error {
	db, dbi, err := database.GetDbAndDBI(ctx, database.AssetsDBIName)
	if err != nil {
		return fmt.Errorf("database not found in context")
	}

	return db.Update(func(txn *lmdb.Txn) error {
		// get refs
		buf, err := txn.Get(dbi, []byte("refs."+hash))
		if err != nil {
			if lmdb.IsNotFound(err) {
				// no refs yet, create new
				refs := []string{messageID}
				if err := database.MarshalAndPut(txn, dbi, []byte("refs."+hash), refs); err != nil {
					return fmt.Errorf("failed to create new refs for hash %s: %w", hash, err)
				}
				xlog.Debugf(ctx, "Created new refs for hash %s with message ID %s", hash, messageID)
				return nil
			}
			return fmt.Errorf("failed to get refs for hash %s: %w", hash, err)
		}

		// unmarshal existing refs
		var refs []string
		if err := json.Unmarshal(buf, &refs); err != nil {
			return fmt.Errorf("failed to unmarshal refs for hash %s: %w", hash, err)
		}

		// add messageID to refs if not already present
		if slices.Contains(refs, messageID) {
			xlog.Debugf(ctx, "Message ID %s is already referenced for hash %s", messageID, hash)
			return nil
		}
		refs = append(refs, messageID)

		// marshal and put back
		if err := database.MarshalAndPut(txn, dbi, []byte("refs."+hash), refs); err != nil {
			return fmt.Errorf("failed to update refs for hash %s: %w", hash, err)
		}
		xlog.Debugf(ctx, "Updated refs for hash %s with message ID %s", hash, messageID)
		return nil
	})
}
