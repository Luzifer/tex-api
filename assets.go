package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Luzifer/go_helpers/v2/str"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/gofrs/uuid"
)

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
			return errors.Wrap(err, "creating tar entry")
		}
		tarInfo.Name = strings.TrimLeft(strings.Replace(p, basePath, "", 1), "/\\")
		err = w.WriteHeader(tarInfo)
		if err != nil {
			return errors.Wrap(err, "writing tar header")
		}
		osFile, err := os.Open(p) // #nosec G304
		if err != nil {
			return errors.Wrap(err, "opening source file")
		}
		defer func() {
			if err := osFile.Close(); err != nil {
				logrus.WithError(err).Error("closing output file (leaked fd)")
			}
		}()

		if _, err := io.Copy(w, osFile); err != nil {
			return errors.Wrap(err, "copying source file")
		}

		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "walking source dir")
	}

	return buf, errors.Wrap(w.Close(), "closing tar file")
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
			return errors.Wrap(err, "creating zip header")
		}
		zipInfo.Name = strings.TrimLeft(strings.Replace(p, basePath, "", 1), "/\\")
		zipFile, err := w.CreateHeader(zipInfo)
		if err != nil {
			return errors.Wrap(err, "writing zip header")
		}
		osFile, err := os.Open(p) // #nosec G304
		if err != nil {
			return errors.Wrap(err, "opening source file")
		}
		defer func() {
			if err := osFile.Close(); err != nil {
				logrus.WithError(err).Error("closing output file (leaked fd)")
			}
		}()

		if _, err := io.Copy(zipFile, osFile); err != nil {
			return errors.Wrap(err, "copying source file")
		}

		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "walking source dir")
	}

	return buf, errors.Wrap(w.Close(), "closing zip file")
}

func getAssetsFile(uid uuid.UUID, ext string) (io.Reader, error) {
	var (
		buf   = new(bytes.Buffer)
		found bool
	)

	basePath := pathFromUUID(uid, filenameOutputDir)
	err := filepath.Walk(basePath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path.Ext(info.Name()) != ext {
			return nil
		}

		osFile, err := os.Open(p) // #nosec G304
		if err != nil {
			return errors.Wrap(err, "opening file")
		}
		defer func() {
			if err := osFile.Close(); err != nil {
				logrus.WithError(err).Error("closing output file (leaked fd)")
			}
		}()

		if _, err := io.Copy(buf, osFile); err != nil {
			return errors.Wrap(err, "reading file")
		}

		found = true
		return filepath.SkipAll
	})

	if !found {
		// We found no file
		return nil, fs.ErrNotExist
	}

	return buf, errors.Wrap(err, "walking source dir")
}

func shouldPackFile(extension string) bool {
	return str.StringInSlice(extension, []string{
		".log",
		".pdf",
	})
}
