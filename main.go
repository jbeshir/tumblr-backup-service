package main

import (
	"encoding/json"
	"fmt"
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

	name := req.URL.Query().Get("tumblr")
	if name == "" {
		http.Error(w, "Bad Request: tumblr parameter required", 400)
	}
	if !tumblrNameValidator.MatchString(name) {
		http.Error(w, "Bad Request: tumblr parameter must match " + tumblrNameValidator.String(), 400)
	}

	mutex, exist := mutexMap[name]
	if !exist {
		mutex = new(sync.Mutex)
		mutexMap[name] = mutex
	}
	mutex.Lock()
	defer mutex.Unlock()

	err := os.RemoveAll(name)
	if err != nil && !os.IsNotExist(err) {
		http.Error(w, "Internal Server Error: " + err.Error(), 500)
	}

	cmd := exec.Command("python", "tumblr-utils/tumblr_backup.py", name)
	err = cmd.Run()
	if err != nil {
		http.Error(w, "Internal Server Error: " + err.Error(), 500)
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, name))
	err = archivefile.Archive(name, w, nil)
	if err != nil {
		http.Error(w, "Internal Server Error: " + err.Error(), 500)
	}

	_ = os.RemoveAll(name)
}