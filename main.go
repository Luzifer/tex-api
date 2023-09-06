package main

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/Luzifer/rconfig/v2"
	"github.com/pkg/errors"

	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	filenameInput      = "input.zip"
	filenameStatus     = "status.json"
	filenameStatusTemp = "status.tmp.json"
	filenameOutputDir  = "output"
	sleepBase          = 1.5
)

var (
	cfg = struct {
		DefaultEnv     string `flag:"default-env" default:"" description:"Environment to copy to the job before unpacking"`
		Script         string `flag:"script" default:"tex-build.sh" description:"Script to execute (needs to generate output directory)"`
		Listen         string `flag:"listen" default:":3000" description:"IP/Port to listen on"`
		StorageDir     string `flag:"storage-dir" default:"/storage" description:"Where to store uploaded ZIPs and resulting files"`
		VersionAndExit bool   `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	version = "dev"
	router  = mux.NewRouter()
)

func initApp() error {
	rconfig.AutoEnv(true)
	if err := rconfig.Parse(&cfg); err != nil {
		return errors.Wrap(err, "parsing cli options")
	}

	return nil
}

func main() {
	var err error
	if err = initApp(); err != nil {
		logrus.WithError(err).Fatal("app initialization failed")
	}

	if cfg.VersionAndExit {
		logrus.WithField("version", version).Info("tex-api")
		os.Exit(0)
	}

	router.HandleFunc("/job", startNewJob).
		Methods("POST").
		Name("startNewJob")

	router.HandleFunc("/job/{uid:[0-9a-z-]{36}}", getJobStatus).
		Methods("GET").
		Name("getJobStatus")

	router.HandleFunc("/job/{uid:[0-9a-z-]{36}}/wait", waitForJob).
		Methods("GET").
		Name("waitForJob")

	router.HandleFunc("/job/{uid:[0-9a-z-]{36}}/download", downloadAssets).
		Methods("GET").
		Name("downloadAssets")

	server := &http.Server{
		Addr:              cfg.Listen,
		Handler:           router,
		ReadHeaderTimeout: time.Second,
	}

	logrus.WithFields(logrus.Fields{
		"addr":    cfg.Listen,
		"version": version,
	}).Info("tex-api started")
	if err := server.ListenAndServe(); err != nil {
		logrus.WithError(err).Fatal("HTTP server exited with error")
	}
}

func downloadAssets(res http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uid, err := uuid.FromString(vars["uid"])
	if err != nil {
		http.Error(res, "UUID had unexpected format!", http.StatusBadRequest)
		return
	}

	var (
		content     io.Reader
		contentType = "application/zip"
		filename    string
	)

	switch r.Header.Get("Accept") {
	case "application/tar", "application/x-tar", "application/x-gtar", "multipart/x-tar", "application/x-compress", "application/x-compressed":
		contentType = "application/tar"
		content, err = buildAssetsTAR(uid)
		filename = uid.String() + ".tar"

	case "application/pdf":
		contentType = "application/pdf"
		filename = uid.String() + ".pdf"
		content, err = getAssetsFile(uid, ".pdf")

		if errors.Is(err, fs.ErrNotExist) && r.URL.Query().Has("log-on-error") {
			contentType = "application/octet-stream"
			filename = uid.String() + ".log"
			content, err = getAssetsFile(uid, ".log")
		}

	default:
		content, err = buildAssetsZIP(uid)
		filename = uid.String() + ".zip"
	}

	if err != nil {
		serverErrorf(res, err, "generating downloadable asset")
		return
	}

	res.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	res.Header().Set("Content-Type", contentType)
	res.WriteHeader(http.StatusOK)

	if _, err = io.Copy(res, content); err != nil {
		serverErrorf(res, err, "writing content")
		return
	}
}
