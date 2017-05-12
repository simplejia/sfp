// 超级代理服务.
// author: simplejia
// date: 2017/4/27
package main

import (
	"fmt"
	"net/http"

	"github.com/simplejia/clog"
	"github.com/simplejia/lc"
	"github.com/simplejia/sfp/conf"
	"github.com/simplejia/sfp/srv"
)

func init() {
	lc.Init(1e5)
}

func main() {
	clog.Info("main() start")

	http.HandleFunc("/", srv.Srv)
	http.HandleFunc("/sfp/conf/get", conf.Cgi)

	c := conf.Get()
	addr := fmt.Sprintf("%s:%d", "0.0.0.0", c.App.Port)
	clog.Error("main() err: %v", http.ListenAndServe(addr, nil))
}
