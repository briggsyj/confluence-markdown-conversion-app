package export

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ResolveInput returns a directory to read pages from, extracting path
// first if it's a .zip archive (matching what Confluence's "Export Space"
// download gives you). For a plain directory or single .html file, path is
// returned unchanged with a no-op cleanup.
//
// The returned cleanup func must be called only once the caller is fully
// done reading from the directory - including any deferred asset copying
// done later by internal/writer, not just the initial Load() call - since
// for a zip input the returned directory is a temp extraction that
// cleanup removes.
func ResolveInput(path string) (dir string, cleanup func() error, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, err
	}
	noop := func() error { return nil }

	if info.IsDir() || !strings.EqualFold(filepath.Ext(path), ".zip") {
		return path, noop, nil
	}
	return extractZip(path)
}

func extractZip(zipPath string) (string, func() error, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", nil, fmt.Errorf("opening zip %q: %w", zipPath, err)
	}
	defer r.Close()

	dir, err := os.MkdirTemp("", "confluence-md-export-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() error { return os.RemoveAll(dir) }

	for _, f := range r.File {
		if err := extractZipEntry(dir, f); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("extracting %q: %w", f.Name, err)
		}
	}
	return dir, cleanup, nil
}

// extractZipEntry writes a single zip entry under destDir, rejecting any
// entry whose name would resolve outside destDir ("Zip Slip") - a zip
// archive is untrusted input and can contain paths like "../../etc/passwd".
func extractZipEntry(destDir string, f *zip.File) error {
	target := filepath.Join(destDir, f.Name)
	destDirClean := filepath.Clean(destDir)
	if target != destDirClean && !strings.HasPrefix(target, destDirClean+string(os.PathSeparator)) {
		return fmt.Errorf("illegal file path escapes destination: %q", f.Name)
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}
