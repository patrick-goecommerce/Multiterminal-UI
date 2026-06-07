// Package backend — archive extractors for STT installer downloads.
package backend

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// extractZip extracts entries from zipPath into destDir.
// keep returns the destination filename for a given zip entry (or "" to skip).
func extractZip(zipPath, destDir string, keep func(name string) string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		outName := keep(f.Name)
		if outName == "" {
			continue
		}
		outPath := filepath.Join(destDir, outName)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("zip open %s: %w", f.Name, err)
		}
		out, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return fmt.Errorf("zip extract %s: %w", f.Name, err)
		}
		out.Close()
		rc.Close()
		os.Chmod(outPath, 0o755) //nolint:errcheck // best-effort
	}
	return nil
}

// extractTarBz2 extracts entries from a tar.bz2 file into destDir.
func extractTarBz2(tarPath, destDir string, keep func(name string) string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("open tar.bz2: %w", err)
	}
	defer f.Close()
	tr := tar.NewReader(bzip2.NewReader(f))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar.bz2 read: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		outName := keep(hdr.Name)
		if outName == "" {
			continue
		}
		outPath := filepath.Join(destDir, outName)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return fmt.Errorf("tar.bz2 extract %s: %w", hdr.Name, err)
		}
		out.Close()
		os.Chmod(outPath, 0o755) //nolint:errcheck // best-effort
	}
	return nil
}
