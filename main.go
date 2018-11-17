// 超级代理服务.
// author: simplejia
// date: 2017/4/27
package main

import (
	"fmt"
	"net/http"

	"github.com/simplejia/clog/api"
	"github.com/simplejia/lc"
	"github.com/simplejia/namecli/api"
	"github.com/simplejia/sfp/conf"
	"github.com/simplejia/sfp/srv"
	"github.com/simplejia/utils"
)

func init() {
	lc.Init(1e5)

	clog.AddrFunc = func() (string, error) {
		return api.Name("clog.srv.ns")
	}
	c := conf.Get()
	clog.Init(c.App.Name, "", c.Clog.Level, c.Clog.Mode)
}

func main() {
	clog.Info("main() start")

	http.HandleFunc("/", srv.Srv)
	http.HandleFunc("/sfp/multi", srv.Multi)

	c := conf.Get()
	addr := fmt.Sprintf(":%d", c.App.Port)
	err := utils.ListenAndServe(addr, nil)
	if err != nil {
		clog.Error("main() err: %v", err)
	}
}
