package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"golang.org/x/crypto/acme/autocert"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sync"

	archivefile "github.com/pierrre/archivefile/zip"
)

var config map[string]string
var mutexMap = make(map[string]*sync.Mutex)

var tumblrNameValidator = regexp.MustCompile("^[A-Za-z0-9_-]+$")

func main() {
	flag.Parse()

	configBytes, err := ioutil.ReadFile("config.json")
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/download", handle)

	l := autocert.NewListener(config["hostname"])
	err = http.Serve(l, nil)
	if err != nil {
		panic(err)
	}
}

func handle(w http.ResponseWriter, req *http.Request) {

	glog.Info(fmt.Sprintf("%p", req), " new request: ", req.URL)

	name := req.URL.Query().Get("tumblr")
	if name == "" {
		http.Error(w, "Bad Request: tumblr parameter required", 400)
		glog.Error(fmt.Sprintf("%p", req), " bad request: ", "tumblr parameter required")
		return
	}
	if !tumblrNameValidator.MatchString(name) {
		http.Error(w, "Bad Request: tumblr parameter must match "+tumblrNameValidator.String(), 400)
		glog.Error(fmt.Sprintf("%p", req), " bad request: ", "tumblr parameter did not match validator regexp")
		return
	}

	mutex, exist := mutexMap[name]
	if !exist {
		mutex = new(sync.Mutex)
		mutexMap[name] = mutex
	}

	glog.Info(fmt.Sprintf("%p", req), " acquiring name mutex for ", name)
	mutex.Lock()
	defer mutex.Unlock()
	glog.Info(fmt.Sprintf("%p", req), " acquired name mutex for ", name)

	err := os.RemoveAll(name)
	if err != nil && !os.IsNotExist(err) {
		http.Error(w, "Internal Server Error: "+err.Error(), 500)
		glog.Error(fmt.Sprintf("%p", req), " couldn't remove folder: ", err)
		return
	}
	defer os.RemoveAll(name)

	cmd := exec.Command("python", "tumblr-utils/tumblr_backup.py", name)
	err = cmd.Run()
	if err != nil {
		http.Error(w, "Internal Server Error: "+err.Error(), 500)
		glog.Error(fmt.Sprintf("%p", req), " couldn't retrieve tumblr: ", err)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, name))
	err = archivefile.Archive(name, w, nil)
	if err != nil {
		glog.Error(fmt.Sprintf("%p", req), " couldn't write zip: ", err)
		return
	}

	glog.Info(fmt.Sprintf("%p", req), " complete")
}
