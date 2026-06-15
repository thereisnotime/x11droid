// SPDX-License-Identifier: GPL-3.0-only

// Package image resolves a custom Waydroid system/vendor image from a local
// path or an http(s) URL into a raw .img on disk: it downloads if needed,
// extracts the image out of a .zip, and validates the result before use.
package image

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// MinImageBytes is the floor below which a file is rejected as "not an image".
// A real Waydroid system/vendor image is hundreds of MB to several GB; this only
// catches obviously-wrong inputs (an HTML error page, a truncated download). It's
// a var so tests can lower it.
var MinImageBytes int64 = 8 << 20 // 8 MiB

// sparseMagic is the Android sparse-image header. Waydroid wants a raw (unsparsed)
// ext image; a sparse one must be run through simg2img first.
var sparseMagic = []byte{0x3a, 0xff, 0x26, 0xed}

// httpClient is used for URL downloads (overridable in tests).
var httpClient = &http.Client{Timeout: 30 * time.Minute}

// IsURL reports whether src is an http(s) URL (vs a local path).
func IsURL(src string) bool {
	return strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")
}

// Resolve fetches the image referenced by src (a local path or http(s) URL),
// extracting imgName ("system.img" / "vendor.img") if src is a .zip, validates
// it, and writes the raw image to dst. dst's parent directory must exist.
func Resolve(src, dst, imgName string) error {
	local := src
	if IsURL(src) {
		tmp, err := download(src)
		if err != nil {
			return err
		}
		defer os.Remove(tmp)
		local = tmp
	}
	if _, err := os.Stat(local); err != nil {
		return fmt.Errorf("image source %q: %w", src, err)
	}

	if isZip(local) {
		if err := extractImg(local, imgName, dst); err != nil {
			return err
		}
	} else if err := copyFile(local, dst); err != nil {
		return err
	}
	return validate(dst)
}

// download streams a URL to a temp file and returns its path.
func download(url string) (string, error) {
	resp, err := httpClient.Get(url) //nolint:noctx // long-running download, client has a timeout
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: HTTP %s", url, resp.Status)
	}
	f, err := os.CreateTemp("", "x11droid-img-*")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// isZip reports whether the file at p is a ZIP archive (by magic bytes, falling
// back to the extension).
func isZip(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return strings.EqualFold(filepath.Ext(p), ".zip")
	}
	defer f.Close()
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return strings.EqualFold(filepath.Ext(p), ".zip")
	}
	// "PK\x03\x04" (normal) or "PK\x05\x06" (empty archive).
	return bytes.Equal(magic, []byte{'P', 'K', 0x03, 0x04}) || bytes.Equal(magic, []byte{'P', 'K', 0x05, 0x06})
}

// extractImg pulls the wanted .img out of a zip into dst. It matches an entry
// whose base name is exactly want, ends with "-want", or is any *.img containing
// the image keyword (e.g. "system" for system.img).
func extractImg(zipPath, want, dst string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	defer func() { _ = r.Close() }()

	keyword := strings.TrimSuffix(want, ".img")
	var entry *zip.File
	for _, f := range r.File {
		base := path.Base(f.Name)
		if base == want || strings.HasSuffix(base, "-"+want) ||
			(strings.HasSuffix(base, ".img") && strings.Contains(base, keyword)) {
			entry = f
			break
		}
	}
	if entry == nil {
		return fmt.Errorf("no %s found in %s", want, zipPath)
	}

	rc, err := entry.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	return writeFile(rc, dst)
}

// validate sanity-checks a resolved image: it must be a regular, non-trivial
// file, and not a still-sparse Android image (Waydroid needs it unsparsed).
func validate(p string) error {
	fi, err := os.Stat(p)
	if err != nil {
		return err
	}
	if fi.Size() < MinImageBytes {
		return fmt.Errorf("%s is only %d bytes — not a valid Waydroid image (download truncated or wrong file?)", p, fi.Size())
	}
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err == nil && bytes.Equal(magic, sparseMagic) {
		return fmt.Errorf("%s is an Android sparse image — convert it to raw first: simg2img <in> <out>", p)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	return writeFile(in, dst)
}

func writeFile(r io.Reader, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	return out.Close()
}
