package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
)

type Build struct {
	Source    string `json:"source"`
	Commit    string `json:"commit"`
	Artifacts []string `json:"artifacts"`
	Project   string `json:"project"`
}

//How to test:
// Post:
//    curl -v -X POST http://127.0.0.1:8090/build -H 'Content-Type: multipart/form-data' -F file=go1.15.1.linux-ppc64le.tar.gz -F source=github.com/golang/go.git -F commit=1234 -F project=golang/master
// Get:
//    curl -v -X GET http://127.0.0.1:8090/build\?project\=golang/master\&commit\=1234 -o "file1.tar.gz"
func build(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		if err := handlePostBuild(req); err != nil {
			http.Error(w, fmt.Sprintf("failed to handle the build: %v", err), http.StatusInternalServerError)
		}
	case http.MethodGet:
		if err := handleGetBuild(w, req); err != nil {
			http.Error(w, fmt.Sprintf("failed to handle the build: %v", err), http.StatusInternalServerError)
		}
	default:
		http.Error(w, fmt.Sprintf("Bad Request: %s method not supported\n", req.Method), http.StatusMethodNotAllowed)
	}
}

func health(w http.ResponseWriter, req *http.Request) {
	fmt.Fprint(w, "ok")
}

func main() {
	klog.Info("Start: build-bot")
	r := mux.NewRouter()
	r.HandleFunc("/build", build).Methods("GET", "POST")
	r.HandleFunc("/health", health).Methods("GET")
	err := r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		pathTemplate, err := route.GetPathTemplate()
		if err == nil {
			fmt.Println("ROUTE:", pathTemplate)
		}
		pathRegexp, err := route.GetPathRegexp()
		if err == nil {
			fmt.Println("Path regexp:", pathRegexp)
		}
		queriesTemplates, err := route.GetQueriesTemplates()
		if err == nil {
			fmt.Println("Queries templates:", strings.Join(queriesTemplates, ","))
		}
		queriesRegexps, err := route.GetQueriesRegexp()
		if err == nil {
			fmt.Println("Queries regexps:", strings.Join(queriesRegexps, ","))
		}
		methods, err := route.GetMethods()
		if err == nil {
			fmt.Println("Methods:", strings.Join(methods, ","))
		}
		fmt.Println()
		return nil
	})

	if err != nil {
		fmt.Println(err)
	}

	if err := http.ListenAndServe(":8090", r); err != nil {
		log.Fatalf("Failed to start the webserver: %v", err)
	}
}
