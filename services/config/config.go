package config

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/digitalcircle-com-br/foundation/lib/core"
	"github.com/digitalcircle-com-br/foundation/lib/model"
	"github.com/digitalcircle-com-br/foundation/lib/redismgr"

	"github.com/digitalcircle-com-br/foundation/lib/routemgr"
	"gopkg.in/yaml.v3"
)

var root = "db"

//json2Yaml parses json to yaml
func json2Yaml(bs []byte) ([]byte, error) {
	var root map[string]interface{}
	err := json.Unmarshal(bs, &root)
	if err != nil {
		return nil, err
	}
	nbs, err := yaml.Marshal(root)
	return nbs, err
}

//yaml2Json parses yaml to json
func yaml2Json(bs []byte) ([]byte, error) {
	var root map[string]interface{}
	err := yaml.Unmarshal(bs, &root)
	if err != nil {
		return nil, err
	}
	nbs, err := json.Marshal(root)
	return nbs, err
}

//fixPath parse "/w/x/y/z" to "y/z"
func fixPath(r *http.Request) string {
	pathparts := strings.Split(r.URL.Path, "/")
	npath := strings.Join(pathparts[2:], "/")
	return npath
}

//get returns the file identified by the request's URI
func get(w http.ResponseWriter, r *http.Request) error {
	npath := fixPath(r)
	bs, err := os.ReadFile(filepath.Join(root, npath+".yaml"))
	if err != nil {
		return err
	}
	if r.URL.Query().Get("fmt") == "json" {
		bs, err = yaml2Json(bs)
		if err != nil {
			return err
		}
	}
	w.Write(bs)
	return nil
}

//post writes the configuration sent by client via Request.Body to file
func post(w http.ResponseWriter, r *http.Request) error {
	npath := fixPath(r)
	dir := filepath.Dir(npath)
	os.MkdirAll(filepath.Join(root, dir), os.ModePerm)
	defer r.Body.Close()
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if r.URL.Query().Get("fmt") == "json" {
		bs, err = json2Yaml(bs)
		if err != nil {
			return err
		}
	}
	err = os.WriteFile(filepath.Join(root, npath+".yaml"), bs, 0600)
	redismgr.Pub("config", npath)
	return err
}

//delete removes the file identified by the Request URI
func delete(w http.ResponseWriter, r *http.Request) error {
	npath := fixPath(r)
	err := os.Remove(filepath.Join(root, npath))
	redismgr.Pub("config", r.URL.Path)
	return err
}

//list scans root directory, returning the present filenames to client
func list(w http.ResponseWriter, r *http.Request) error {
	var ret []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		npath := strings.Replace(path, ext, "", 1)
		npath = strings.Replace(npath, "db", "", 1)
		ret = append(ret, npath)
		return nil
	})
	if err != nil {
		return err
	}
	err = json.NewEncoder(w).Encode(ret)

	return err
}

//Setup adds configuration API paths to mux.Router
func Setup() {
	routemgr.HandleHttp("/list", http.MethodGet, model.PERM_ALL, list)
	routemgr.HandleHttp("/k/", http.MethodGet, model.PERM_ALL, get)
	routemgr.HandleHttp("/k/", http.MethodPost, model.PERM_ALL, post)
	routemgr.HandleHttp("/k/", http.MethodDelete, model.PERM_ALL, delete)

	core.Log("Config running - keys:")
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			core.Log(" - %s", path)
		}

		return nil
	})
}

//Run configures mux.Router with configuration routes and start listening requests to 0.0.0.0:8080
func Run() error {
	Setup()
	return http.ListenAndServe(":8080", routemgr.Router())
}
