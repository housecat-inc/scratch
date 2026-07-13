package sql

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/cockroachdb/errors"
)

const sqlsVersion = "0.2.45"

func (s *Server) sqlsBinary(ctx context.Context) (string, error) {
	s.sqlsMu.Lock()
	defer s.sqlsMu.Unlock()
	if s.sqlsBin != "" {
		return s.sqlsBin, nil
	}
	if p, err := exec.LookPath("sqls"); err == nil {
		s.sqlsBin = p
		return p, nil
	}
	if s.deps.BinDir == "" {
		return "", errors.New("sqls not installed and no bin dir configured")
	}
	managed := filepath.Join(s.deps.BinDir, "sqls")
	if isExecutable(managed) {
		s.sqlsBin = managed
		return managed, nil
	}
	if !s.deps.AutoInstall {
		return "", errors.New("sqls not installed")
	}
	if err := downloadSqls(ctx, s.deps.BinDir); err != nil {
		return "", err
	}
	s.sqlsBin = managed
	return managed, nil
}

func downloadSqls(ctx context.Context, dir string) error {
	url, err := sqlsDownloadURL()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return errors.Wrap(err, "mkdir bin dir")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return errors.Wrap(err, "sqls request")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "download sqls")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("download sqls: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "read sqls archive")
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return errors.Wrap(err, "open sqls archive")
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) != "sqls" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return errors.Wrap(err, "open archive entry")
		}
		defer rc.Close()
		dst := filepath.Join(dir, "sqls")
		tmp := dst + ".tmp"
		out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return errors.Wrap(err, "create sqls")
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			return errors.Wrap(err, "write sqls")
		}
		if err := out.Close(); err != nil {
			return errors.Wrap(err, "close sqls")
		}
		if err := os.Rename(tmp, dst); err != nil {
			return errors.Wrap(err, "install sqls")
		}
		return nil
	}
	return errors.New("sqls binary not found in release archive")
}

func sqlsDownloadURL() (string, error) {
	switch runtime.GOOS {
	case "darwin", "linux":
		return fmt.Sprintf("https://github.com/sqls-server/sqls/releases/download/v%s/sqls-%s-%s.zip", sqlsVersion, runtime.GOOS, sqlsVersion), nil
	default:
		return "", errors.Errorf("sqls: unsupported OS %q", runtime.GOOS)
	}
}

func isExecutable(path string) bool {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return false
	}
	return fi.Mode()&0o111 != 0
}
