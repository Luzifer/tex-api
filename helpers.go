package main

import (
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/Luzifer/go_helpers/v2/str"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

const zipHeaderLength = 4

func hasZipHeader(data []byte) bool {
	if len(data) < zipHeaderLength {
		// ZIP header must be 4 chars
		return false
	}

	return str.StringInSlice(string(data[:4]), []string{
		// https://en.wikipedia.org/wiki/ZIP_(file_format)
		"PK\x03\x04",
		"PK\x05\x06",
		"PK\x07\x08",
	})
}

func pathFromUUID(uid uuid.UUID, filename string) string {
	return path.Join(cfg.StorageDir, uid.String(), filename)
}

func serverErrorf(res http.ResponseWriter, err error, tpl string, args ...interface{}) {
	logrus.WithError(err).Errorf(tpl, args...)
	http.Error(res, "An error occurred. See details in log.", http.StatusInternalServerError)
}

func syncFilesToOverlay(base, overlay afero.Fs) error {
	cow := afero.NewCopyOnWriteFs(base, overlay)
	return errors.Wrap(
		afero.Walk(cow, "", func(path string, info fs.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}

			return errors.Wrapf(
				cow.Chtimes(path, time.Now(), time.Now()),
				"triggering copy for %s by changing time", path,
			)
		}),
		"copying files to overlay",
	)
}

func urlMust(u *url.URL, err error) *url.URL {
	if err != nil {
		logrus.WithError(err).Fatal("Unable to retrieve URL from router")
	}
	return u
}
