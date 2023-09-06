package main

import (
	"net/http"
	"net/url"
	"path"

	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
)

func pathFromUUID(uid uuid.UUID, filename string) string {
	return path.Join(cfg.StorageDir, uid.String(), filename)
}

func serverErrorf(res http.ResponseWriter, err error, tpl string, args ...interface{}) {
	logrus.WithError(err).Errorf(tpl, args...)
	http.Error(res, "An error occurred. See details in log.", http.StatusInternalServerError)
}

func urlMust(u *url.URL, err error) *url.URL {
	if err != nil {
		logrus.WithError(err).Fatal("Unable to retrieve URL from router")
	}
	return u
}
