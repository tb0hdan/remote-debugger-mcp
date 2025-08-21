package profiler

import (
	"net/http"
	"net/http/pprof"
)

func Index(w http.ResponseWriter, r *http.Request) {
	pprof.Index(w, r)
}

func Cmdline(w http.ResponseWriter, r *http.Request) {
	pprof.Cmdline(w, r)
}

func Profile(w http.ResponseWriter, r *http.Request) {
	pprof.Profile(w, r)
}

func Symbol(w http.ResponseWriter, r *http.Request) {
	pprof.Symbol(w, r)
}

func Trace(w http.ResponseWriter, r *http.Request) {
	pprof.Trace(w, r)
}
