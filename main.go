package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	"github.com/Luzifer/rconfig/v2"

	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var (
	cfg = struct {
		Script         string `flag:"script" default:"tex-build.sh" description:"Script to execute (needs to generate output directory)"`
		Listen         string `flag:"listen" default:":3000" description:"IP/Port to listen on"`
		StorageDir     string `flag:"storage-dir" default:"/storage" description:"Where to store uploaded ZIPs and resulting files"`
		VersionAndExit bool   `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	version = "dev"
	router  = mux.NewRouter()
)

type status string

const (
	statusCreated  = "created"
	statusStarted  = "started"
	statusError    = "error"
	statusFinished = "finished"
)

const (
	filenameInput     = "input.zip"
	filenameStatus    = "status.json"
	filenameOutputDir = "output"
	sleepBase         = 1.5
)

type jobStatus struct {
	UUID      string    `json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Status    status    `json:"status"`
}

func loadStatusByUUID(uid uuid.UUID) (*jobStatus, error) {
	statusFile := pathFromUUID(uid, filenameStatus)

	status := jobStatus{}
	// #nosec G304
	if f, err := os.Open(statusFile); err == nil {
		defer f.Close()
		if err = json.NewDecoder(f).Decode(&status); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	return &status, nil
}

func (s *jobStatus) UpdateStatus(st status) {
	s.Status = st
	s.UpdatedAt = time.Now()
}

func (s jobStatus) Save() error {
	uid, _ := uuid.FromString(s.UUID) // #nosec G104
	f, err := os.Create(pathFromUUID(uid, filenameStatus))
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(s)
}

func urlMust(u *url.URL, err error) *url.URL {
	if err != nil {
		log.WithError(err).Fatal("Unable to retrieve URL from router")
	}
	return u
}

func init() {
	rconfig.AutoEnv(true)
	if err := rconfig.Parse(&cfg); err != nil {
		log.WithError(err).Fatal("Unable to parse commandline options")
	}

	if cfg.VersionAndExit {
		fmt.Printf("tex-api %s\n", version)
		os.Exit(0)
	}
}

func main() {
	router.HandleFunc("/job", startNewJob).Methods("POST").Name("startNewJob")
	router.HandleFunc("/job/{uid:[0-9a-z-]{36}}", getJobStatus).Methods("GET").Name("getJobStatus")
	router.HandleFunc("/job/{uid:[0-9a-z-]{36}}/wait", waitForJob).Methods("GET").Name("waitForJob")
	router.HandleFunc("/job/{uid:[0-9a-z-]{36}}/download", downloadAssets).Methods("GET").Name("downloadAssets")

	if err := http.ListenAndServe(cfg.Listen, router); err != nil {
		log.WithError(err).Fatal("HTTP server exited with error")
	}
}

func serverErrorf(res http.ResponseWriter, err error, tpl string, args ...interface{}) {
	log.WithError(err).Errorf(tpl, args...)
	http.Error(res, "An error occurred. See details in log.", http.StatusInternalServerError)
}

func pathFromUUID(uid uuid.UUID, filename string) string {
	return path.Join(cfg.StorageDir, uid.String(), filename)
}

func startNewJob(res http.ResponseWriter, r *http.Request) {
	jobUUID := uuid.Must(uuid.NewV4())
	inputFile := pathFromUUID(jobUUID, filenameInput)
	statusFile := pathFromUUID(jobUUID, filenameStatus)

	if err := os.Mkdir(path.Dir(inputFile), 0750); err != nil {
		log.WithError(err).Errorf("Unable to create job dir %q", path.Dir(inputFile))
	}

	if f, err := os.Create(inputFile); err == nil {
		defer f.Close()
		if _, copyErr := io.Copy(f, r.Body); copyErr != nil {
			serverErrorf(res, copyErr, "Unable to copy input file %q", inputFile)
			return
		}
		f.Sync() // #nosec G104
	} else {
		serverErrorf(res, err, "Unable to write input file %q", inputFile)
		return
	}

	status := jobStatus{
		UUID:      jobUUID.String(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Status:    statusCreated,
	}
	if err := status.Save(); err != nil {
		serverErrorf(res, err, "Unable to create status file %q", statusFile)
		return
	}

	go jobProcessor(jobUUID)

	u := urlMust(router.Get("waitForJob").URL("uid", jobUUID.String()))
	http.Redirect(res, r, u.String(), http.StatusFound)
}

func getJobStatus(res http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uid, err := uuid.FromString(vars["uid"])
	if err != nil {
		http.Error(res, "UUID had unexpected format!", http.StatusBadRequest)
		return
	}

	if status, err := loadStatusByUUID(uid); err == nil {
		if encErr := json.NewEncoder(res).Encode(status); encErr != nil {
			serverErrorf(res, encErr, "Unable to serialize status file")
			return
		}
	} else {
		serverErrorf(res, err, "Unable to read status file")
		return
	}
}

func waitForJob(res http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uid, err := uuid.FromString(vars["uid"])
	if err != nil {
		http.Error(res, "UUID had unexpected format!", http.StatusBadRequest)
		return
	}

	var loop int
	if v := r.URL.Query().Get("loop"); v != "" {
		if pv, convErr := strconv.Atoi(v); convErr == nil {
			loop = pv
		}
	}
	loop++

	status, err := loadStatusByUUID(uid)
	if err != nil {
		serverErrorf(res, err, "Unable to read status file")
		return
	}

	switch status.Status {
	case statusCreated:
		fallthrough

	case statusStarted:
		u := urlMust(router.Get("waitForJob").URL("uid", uid.String()))
		u.Query().Set("loop", strconv.Itoa(loop))

		<-time.After(time.Duration(math.Pow(sleepBase, float64(loop))) * time.Second)

		http.Redirect(res, r, u.String(), http.StatusFound)
		return

	case statusError:
		http.Error(res, "Processing ran into an error.", http.StatusInternalServerError)

	case statusFinished:
		u := urlMust(router.Get("downloadAssets").URL("uid", uid.String()))
		http.Redirect(res, r, u.String(), http.StatusFound)
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
	default:
		content, err = buildAssetsZIP(uid)
		filename = uid.String() + ".zip"
	}

	if err != nil {
		serverErrorf(res, err, "Unable to generate downloadable asset")
		return
	}

	res.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	res.Header().Set("Content-Type", contentType)
	res.WriteHeader(http.StatusOK)

	io.Copy(res, content) // #nosec G104
}

func jobProcessor(uid uuid.UUID) {
	logger := log.WithField("uuid", uid)
	logger.Info("Started processing")

	processingDir := path.Dir(pathFromUUID(uid, filenameStatus))
	status, err := loadStatusByUUID(uid)
	if err != nil {
		logger.WithError(err).Error("Unable to load status file in processing job")
		return
	}

	cmd := exec.Command("/bin/bash", cfg.Script) // #nosec G204
	cmd.Dir = processingDir
	cmd.Stderr = logger.WriterLevel(log.InfoLevel) // Bash uses stderr for `-x` parameter

	status.UpdateStatus(statusStarted)
	if err := status.Save(); err != nil {
		logger.WithError(err).Error("Unable to save status file")
		return
	}

	if err := cmd.Run(); err != nil {
		logger.WithError(err).Error("Processing failed")
		status.UpdateStatus(statusError)
		if err := status.Save(); err != nil {
			logger.WithError(err).Error("Unable to save status file")
			return
		}
		return
	}

	status.UpdateStatus(statusFinished)
	if err := status.Save(); err != nil {
		logger.WithError(err).Error("Unable to save status file")
		return
	}
	logger.Info("Finished processing")
}
