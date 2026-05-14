package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
)

func buildAssetsTAR(uid uuid.UUID) (io.Reader, error) {
	buf := new(bytes.Buffer)
	w := tar.NewWriter(buf)

	basePath := pathFromUUID(uid, filenameOutputDir)
	root, err := os.OpenRoot(basePath)
	if err != nil {
		return nil, fmt.Errorf("opening source root: %w", err)
	}
	defer closeSourceRoot(root)

	err = filepath.WalkDir(basePath, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || !shouldPackFile(path.Ext(entry.Name())) {
			return nil
		}

		relPath, err := filepath.Rel(basePath, p)
		if err != nil {
			return fmt.Errorf("resolving relative path: %w", err)
		}

		osFile, err := root.Open(relPath)
		if err != nil {
			return fmt.Errorf("opening source file: %w", err)
		}

		info, err := osFile.Stat()
		if err != nil {
			closeOutputFile(osFile)
			return fmt.Errorf("stat source file: %w", err)
		}
		if !info.Mode().IsRegular() {
			closeOutputFile(osFile)
			return nil
		}

		tarInfo, err := tar.FileInfoHeader(info, "")
		if err != nil {
			closeOutputFile(osFile)
			return fmt.Errorf("creating tar entry: %w", err)
		}
		tarInfo.Name = filepath.ToSlash(relPath)
		err = w.WriteHeader(tarInfo)
		if err != nil {
			closeOutputFile(osFile)
			return fmt.Errorf("writing tar header: %w", err)
		}

		if _, err := io.Copy(w, osFile); err != nil {
			closeOutputFile(osFile)
			return fmt.Errorf("copying source file: %w", err)
		}
		closeOutputFile(osFile)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking source dir: %w", err)
	}

	if err = w.Close(); err != nil {
		return nil, fmt.Errorf("closing tar file: %w", err)
	}

	return buf, nil
}

func buildAssetsZIP(uid uuid.UUID) (io.Reader, error) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	basePath := pathFromUUID(uid, filenameOutputDir)
	root, err := os.OpenRoot(basePath)
	if err != nil {
		return nil, fmt.Errorf("opening source root: %w", err)
	}
	defer closeSourceRoot(root)

	err = filepath.WalkDir(basePath, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || !shouldPackFile(path.Ext(entry.Name())) {
			return nil
		}

		relPath, err := filepath.Rel(basePath, p)
		if err != nil {
			return fmt.Errorf("resolving relative path: %w", err)
		}

		osFile, err := root.Open(relPath)
		if err != nil {
			return fmt.Errorf("opening source file: %w", err)
		}

		info, err := osFile.Stat()
		if err != nil {
			closeOutputFile(osFile)
			return fmt.Errorf("stat source file: %w", err)
		}
		if !info.Mode().IsRegular() {
			closeOutputFile(osFile)
			return nil
		}

		zipInfo, err := zip.FileInfoHeader(info)
		if err != nil {
			closeOutputFile(osFile)
			return fmt.Errorf("creating zip header: %w", err)
		}
		zipInfo.Name = filepath.ToSlash(relPath)
		zipFile, err := w.CreateHeader(zipInfo)
		if err != nil {
			closeOutputFile(osFile)
			return fmt.Errorf("writing zip header: %w", err)
		}

		if _, err := io.Copy(zipFile, osFile); err != nil {
			closeOutputFile(osFile)
			return fmt.Errorf("copying source file: %w", err)
		}
		closeOutputFile(osFile)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking source dir: %w", err)
	}

	if err = w.Close(); err != nil {
		return nil, fmt.Errorf("closing zip file: %w", err)
	}

	return buf, nil
}

func closeOutputFile(osFile *os.File) {
	if err := osFile.Close(); err != nil {
		logrus.WithError(err).Error("closing output file (leaked fd)")
	}
}

func closeSourceRoot(root *os.Root) {
	if err := root.Close(); err != nil {
		logrus.WithError(err).Error("closing source root")
	}
}

func getAssetsFile(uid uuid.UUID, ext string) (io.Reader, error) {
	var (
		buf   = new(bytes.Buffer)
		found bool
	)

	basePath := pathFromUUID(uid, filenameOutputDir)
	root, err := os.OpenRoot(basePath)
	if err != nil {
		return nil, fmt.Errorf("opening source root: %w", err)
	}
	defer closeSourceRoot(root)

	err = filepath.WalkDir(basePath, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || path.Ext(entry.Name()) != ext {
			return nil
		}

		relPath, err := filepath.Rel(basePath, p)
		if err != nil {
			return fmt.Errorf("resolving relative path: %w", err)
		}

		osFile, err := root.Open(relPath)
		if err != nil {
			return fmt.Errorf("opening file: %w", err)
		}

		info, err := osFile.Stat()
		if err != nil {
			closeOutputFile(osFile)
			return fmt.Errorf("stat source file: %w", err)
		}
		if !info.Mode().IsRegular() {
			closeOutputFile(osFile)
			return nil
		}

		if _, err := io.Copy(buf, osFile); err != nil {
			closeOutputFile(osFile)
			return fmt.Errorf("reading file: %w", err)
		}
		closeOutputFile(osFile)

		found = true
		return filepath.SkipAll
	})
	if err != nil {
		return nil, fmt.Errorf("walking source dir: %w", err)
	}

	if !found {
		// We found no file
		return nil, fs.ErrNotExist
	}

	return buf, nil
}

func shouldPackFile(extension string) bool {
	return slices.Contains([]string{
		".log",
		".pdf",
	}, extension)
}
