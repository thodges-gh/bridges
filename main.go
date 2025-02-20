package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/linkpoolio/bridges/bridge"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/utahta/go-openuri"
	"io/ioutil"
	"net/http"
	"os"
)

// Bridge is the struct to represent the bridge JSON
// files to used as bridges
type Bridge struct {
	Name   string          `json:"name"`
	Method string          `json:"method"`
	URL    string          `json:"url"`
	Path   string          `json:"path"`
	Auth   BridgeCallAuth  `json:"auth"`
	Opts   bridge.CallOpts `json:"opts"`
}

// BridgeCallAuth represents the type of authentication to be used
// on the call to the bridges API
type BridgeCallAuth struct {
	Type string `json:"type"`
	Key  string `json:"key"`
	Env  string `json:"env"`
}

// JSON is the Bridge implementation that is used for the bridges
// cli to start bridge
type JSON struct {
	bridge Bridge
}

// NewJSONBridges parses the uri and returns an array of initialised
// bridges based from the JSON body of the given URI.
func NewJSONBridges(uri string) ([]bridge.Bridge, error) {
	var bs []Bridge
	var js []bridge.Bridge
	if len(uri) == 0 {
		return nil, errors.New("Empty bridge URI given")
	} else if o, err := openuri.Open(uri); err != nil {
		return nil, err
	} else if b, err := ioutil.ReadAll(o); err != nil {
		return nil, err
	} else if err := json.Unmarshal(b, &bs); err != nil {
		return nil, err
	}
	for _, a := range bs {
		a.Opts.Auth = bridge.NewAuth(
			a.Auth.Type,
			a.Auth.Key,
			os.Getenv(a.Auth.Env),
		)
		js = append(js, &JSON{a})
	}
	return js, nil
}

// Run is the Bridge implementation which takes the JSON version of an adaptor
// makes a call based on whats defined in the model, and returns the response
func (ja *JSON) Run(h *bridge.Helper) (interface{}, error) {
	r := make(map[string]interface{})
	p := make(map[string]interface{})
	for k, v := range ja.bridge.Opts.Query {
		p[k] = h.GetParam(fmt.Sprintf("%s", v))
	}
	ja.bridge.Opts.Query = p
	var url string
	if len(ja.bridge.URL) == 0 {
		url = h.GetParam("url")
	} else {
		url = ja.bridge.URL
	}
	err := h.HTTPCallWithOpts(ja.bridge.Method, url, &r, ja.bridge.Opts)
	return r, err
}

// Opts returns a bridge options type that has values set
// based on the given JSON file
func (ja *JSON) Opts() *bridge.Opts {
	return &bridge.Opts{
		Name:   ja.bridge.Name,
		Path:   ja.bridge.Path,
		Lambda: true,
	}
}

// Handler is the entrypoint for GCP functions
func Handler(w http.ResponseWriter, r *http.Request) {
	env := os.Getenv("BRIDGE")
	if len(env) == 0 {
		w.Write([]byte("No bridge set"))
		return
	} else if b, err := NewJSONBridges(env); err != nil {
		w.Write([]byte(err.Error()))
		return
	} else {
		bridge.NewServer(b...).Handler(w, r)
	}
}

func main() {
	var uri string
	var port int
	pflag.StringVarP(&uri, "bridge", "b", "", "Filepath/URL of bridge JSON file")
	pflag.IntVarP(&port, "port", "p", 8080, "Server port")
	pflag.Parse()

	env := os.Getenv("BRIDGE")
	if len(uri) == 0 && len(env) != 0 {
		uri = env
	}
	if b, err := NewJSONBridges(uri); err != nil {
		logrus.Fatalf("Failed to load bridge: %v", err)
	} else {
		bridge.NewServer(b...).Start(port)
	}
}
