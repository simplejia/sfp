package srv

import (
	"net/http"

	"time"

	"runtime/debug"

	"strings"

	"github.com/simplejia/clog"
	"github.com/simplejia/sfp/conf"
)

func Srv(w http.ResponseWriter, r *http.Request) {
	fun := "srv.Srv"
	uri := r.URL.RequestURI()
	path := strings.TrimSuffix(r.URL.Path, "/")
	code := http.StatusOK
	defer func(btime time.Time) {
		if err := recover(); err != nil {
			clog.Error("%s uri: %s, err: %v, stack: %s", fun, uri, err, debug.Stack())
			code = http.StatusInternalServerError
			http.Error(w, http.StatusText(code), code)
		} else {
			clog.Info("%s uri: %s, code: %d, elapse: %s", fun, uri, code, time.Since(btime))
			if code != http.StatusOK {
				http.Error(w, http.StatusText(code), code)
			}
		}
	}(time.Now())

	c := conf.Get()

	switch {
	case c.Busi4Http[path] != nil: // http
		code = ReqHttp(w, r)
	case c.Busi4Yar[path] != nil: // yar
		code = ReqYar(w, r)
	default:
		clog.Error("%s uri: %s, not found", fun, uri)
		code = http.StatusNotFound
	}

	return
}
