// SPDX-License-Identifier: GPL-3.0-only

package image

import (
	"archive/zip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// withSmallMin lowers the size floor so tests can use tiny fixtures.
func withSmallMin(t *testing.T) {
	t.Helper()
	old := MinImageBytes
	MinImageBytes = 4
	t.Cleanup(func() { MinImageBytes = old })
}

func writeImg(t *testing.T, dir, name string, payload []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, payload, 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// makeZip writes a zip at dir/zipName containing one entry (entryName -> payload).
func makeZip(t *testing.T, dir, zipName, entryName string, payload []byte) string {
	t.Helper()
	p := filepath.Join(dir, zipName)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create(entryName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestIsURL(t *testing.T) {
	cases := map[string]bool{
		"https://example.com/a.zip": true,
		"http://example.com/a.img":  true,
		"/local/path/system.img":    false,
		"./rel.zip":                 false,
		"ftp://example.com/x":       false,
	}
	for in, want := range cases {
		if got := IsURL(in); got != want {
			t.Errorf("IsURL(%q)=%v want %v", in, got, want)
		}
	}
}

func TestResolve_LocalRawImage(t *testing.T) {
	withSmallMin(t)
	dir := t.TempDir()
	src := writeImg(t, dir, "system.img", []byte("RAWIMAGEDATA"))
	dst := filepath.Join(dir, "out.img")

	if err := Resolve(src, dst, "system.img"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "RAWIMAGEDATA" {
		t.Errorf("dst content = %q", got)
	}
}

func TestResolve_LocalZip(t *testing.T) {
	withSmallMin(t)
	dir := t.TempDir()
	// Mirror real Waydroid naming: lineage-22.2-...-waydroid_x86_64-system.img inside a zip.
	src := makeZip(t, dir, "lineage-22.2-waydroid_x86_64-system.zip",
		"lineage-22.2-20260224-UNOFFICIAL-waydroid_x86_64-system.img", []byte("SYSTEMIMGFROMZIP"))
	dst := filepath.Join(dir, "system.img")

	if err := Resolve(src, dst, "system.img"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "SYSTEMIMGFROMZIP" {
		t.Errorf("extracted content = %q", got)
	}
}

func TestResolve_URLRaw(t *testing.T) {
	withSmallMin(t)
	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("DOWNLOADEDIMG"))
	}))
	defer srv.Close()

	dst := filepath.Join(dir, "vendor.img")
	if err := Resolve(srv.URL+"/vendor.img", dst, "vendor.img"); err != nil {
		t.Fatalf("Resolve URL: %v", err)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "DOWNLOADEDIMG" {
		t.Errorf("downloaded content = %q", got)
	}
}

func TestResolve_URLZip(t *testing.T) {
	withSmallMin(t)
	dir := t.TempDir()
	zipPath := makeZip(t, dir, "v.zip", "vendor.img", []byte("VENDORZIP"))
	data, _ := os.ReadFile(zipPath)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	dst := filepath.Join(dir, "vendor.img")
	if err := Resolve(srv.URL+"/vendor.zip", dst, "vendor.img"); err != nil {
		t.Fatalf("Resolve URL zip: %v", err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "VENDORZIP" {
		t.Errorf("content = %q", got)
	}
}

func TestResolve_Errors(t *testing.T) {
	withSmallMin(t)
	dir := t.TempDir()

	t.Run("missing local path", func(t *testing.T) {
		if err := Resolve(filepath.Join(dir, "nope.img"), filepath.Join(dir, "o.img"), "system.img"); err == nil {
			t.Fatal("expected error for missing source")
		}
	})

	t.Run("too small", func(t *testing.T) {
		MinImageBytes = 1024
		defer func() { MinImageBytes = 4 }()
		src := writeImg(t, dir, "tiny.img", []byte("small"))
		if err := Resolve(src, filepath.Join(dir, "o.img"), "system.img"); err == nil {
			t.Fatal("expected error for too-small image")
		}
	})

	t.Run("sparse image rejected", func(t *testing.T) {
		src := writeImg(t, dir, "sparse.img", append([]byte{0x3a, 0xff, 0x26, 0xed}, []byte("rest")...))
		err := Resolve(src, filepath.Join(dir, "o.img"), "system.img")
		if err == nil {
			t.Fatal("expected error for sparse image")
		}
	})

	t.Run("zip missing wanted img", func(t *testing.T) {
		src := makeZip(t, dir, "wrong.zip", "boot.img", []byte("notsystem"))
		if err := Resolve(src, filepath.Join(dir, "o.img"), "system.img"); err == nil {
			t.Fatal("expected error when zip lacks the wanted image")
		}
	})

	t.Run("http 404", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "nope", http.StatusNotFound)
		}))
		defer srv.Close()
		if err := Resolve(srv.URL+"/x.img", filepath.Join(dir, "o.img"), "system.img"); err == nil {
			t.Fatal("expected error for HTTP 404")
		}
	})
}
