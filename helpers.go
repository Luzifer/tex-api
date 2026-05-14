package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"slices"
	"time"

	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

const zipHeaderLength = 4

func hasZipHeader(data []byte) bool {
	if len(data) < zipHeaderLength {
		// ZIP header must be 4 chars
		return false
	}

	return slices.Contains([]string{
		// https://en.wikipedia.org/wiki/ZIP_(file_format)
		"PK\x03\x04",
		"PK\x05\x06",
		"PK\x07\x08",
	}, string(data[:4]))
}

func pathFromUUID(uid uuid.UUID, filename string) string {
	return path.Join(cfg.StorageDir, uid.String(), filename)
}

func serverErrorf(res http.ResponseWriter, err error, tpl string, args ...any) {
	logrus.WithError(err).Errorf(tpl, args...)
	http.Error(res, "An error occurred. See details in log.", http.StatusInternalServerError)
}

func syncFilesToOverlay(base, overlay afero.Fs) (err error) {
	cow := afero.NewCopyOnWriteFs(base, overlay)
	if err = afero.Walk(cow, "", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if err = cow.Chtimes(path, time.Now(), time.Now()); err != nil {
			return fmt.Errorf("triggering copy for %s by changing time: %w", path, err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("copying files to overlay: %w", err)
	}

	return nil
}

func urlMust(u *url.URL, err error) *url.URL {
	if err != nil {
		logrus.WithError(err).Fatal("Unable to retrieve URL from router")
	}
	return u
}
