package clog

import (
	"github.com/simplejia/clog"
	"github.com/simplejia/sfp/conf"
)

func init() {
	c := conf.Get()
	clog.Init(c.Clog.Name, "", c.Clog.Level, c.Clog.Mode)
}
