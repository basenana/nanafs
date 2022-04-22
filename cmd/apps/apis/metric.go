package apis

import (
	"fmt"
	"github.com/basenana/nanafs/config"
	"net/http"
	"net/http/pprof"
	pf "runtime/pprof"
)

func initMetric(cfg config.Api, route map[string]http.HandlerFunc) {
	if cfg.Pprof {
		route["/metric/pprof/"] = pprof.Index

		for _, onePf := range pf.Profiles() {
			route[fmt.Sprintf("/metric/pprof/%s", onePf.Name())] = pprof.Handler(onePf.Name()).ServeHTTP
		}
		return
	}
}
