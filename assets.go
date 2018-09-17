package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Luzifer/go_helpers/str"

	"github.com/gofrs/uuid"
)

func shouldPackFile(extension string) bool {
	return str.StringInSlice(extension, []string{
		".log",
		".pdf",
	})
}

func buildAssetsZIP(uid uuid.UUID) (io.Reader, error) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	basePath := pathFromUUID(uid, filenameOutputDir)
	err := filepath.Walk(basePath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !shouldPackFile(path.Ext(info.Name())) {
			return nil
		}

		zipInfo, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		zipInfo.Name = strings.TrimLeft(strings.Replace(p, basePath, "", 1), "/\\")
		zipFile, err := w.CreateHeader(zipInfo)
		if err != nil {
			return err
		}
		osFile, err := os.Open(p) // #nosec G304
		if err != nil {
			return err
		}

		if _, err := io.Copy(zipFile, osFile); err != nil {
			return err
		}
		osFile.Close() // #nosec G104

		return nil
	})

	if err != nil {
		return nil, err
	}

	return buf, w.Close()
}

func buildAssetsTAR(uid uuid.UUID) (io.Reader, error) {
	buf := new(bytes.Buffer)
	w := tar.NewWriter(buf)

	basePath := pathFromUUID(uid, filenameOutputDir)
	err := filepath.Walk(basePath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !shouldPackFile(path.Ext(info.Name())) {
			return nil
		}

		tarInfo, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		tarInfo.Name = strings.TrimLeft(strings.Replace(p, basePath, "", 1), "/\\")
		err = w.WriteHeader(tarInfo)
		if err != nil {
			return err
		}
		osFile, err := os.Open(p) // #nosec G304
		if err != nil {
			return err
		}

		if _, err := io.Copy(w, osFile); err != nil {
			return err
		}
		osFile.Close() // #nosec G104

		return nil
	})

	if err != nil {
		return nil, err
	}

	return buf, w.Close()
}
