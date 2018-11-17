package conf

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"bytes"

	"github.com/simplejia/clog/api"
	"github.com/simplejia/utils"
)

type Conf struct {
	App *struct {
		Name   string
		Port   int
		Demote bool
	}
	Clog *struct {
		Mode  int
		Level int
	}
	Cons *struct {
		RoutineNum int //
		ChanCap    int //
	}
	VarHost *struct {
		Addr string
		Cgi  string
	}
	Busi      map[string]map[string]*BusiElem
	Busi4Yar  map[string]map[string]*BusiElem `json:"-"`
	Busi4Http map[string]*BusiElem            `json:"-"`
}

type BusiElem struct {
	Addr            string
	Path            string
	Action          string
	Yar             bool
	Ret             interface{}
	Read            bool
	Retry           int
	Timeout         int
	Async           bool
	Demote          bool
	UseGlobalDemote bool
	Expire          string
}

func (busiElem *BusiElem) GetDemote() bool {
	if busiElem.UseGlobalDemote {
		c := Get()
		return c.App.Demote
	}
	return busiElem.Demote
}

func Get() *Conf {
	return C.Load().(*Conf)
}

func Set(c *Conf) {
	C.Store(c)
}

var (
	Env string
	C   atomic.Value
)

func reloadConf() {
	var lastbody []byte
	for {
		time.Sleep(time.Second * 3)

		body, err := getcontents()
		if err != nil || len(body) == 0 {
			clog.Error("getcontents err: %v, body: %s", err, body)
			continue
		}

		if lastbody != nil && bytes.Compare(lastbody, body) == 0 {
			continue
		}

		if err := parse(body); err != nil {
			clog.Error("parse err: %v, body: %s", err, body)
			continue
		}

		if err := savecontents(body); err != nil {
			clog.Error("savecontents err: %v, body: %s", err, body)
			continue
		}

		lastbody = body
	}
}

func getcontents() (fcontent []byte, err error) {
	dir := "conf"
	for i := 0; i < 3; i++ {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			break
		}
		dir = filepath.Join("..", dir)
	}
	fcontent, err = ioutil.ReadFile(filepath.Join(dir, "conf.json"))
	if err != nil {
		return
	}
	return
}

func savecontents(fcontent []byte) (err error) {
	dir := "conf"
	for i := 0; i < 3; i++ {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			break
		}
		dir = filepath.Join("..", dir)
	}
	err = ioutil.WriteFile(filepath.Join(dir, "conf.json"), fcontent, 0644)
	if err != nil {
		return
	}
	return
}

func parse(fcontent []byte) (err error) {
	fcontent = utils.RemoveAnnotation(fcontent)

	var envs map[string]*Conf
	if err = json.Unmarshal(fcontent, &envs); err != nil {
		return
	}

	c := envs[Env]
	if c == nil {
		return fmt.Errorf("env not right: %s", Env)
	}

	c.Busi4Yar = map[string]map[string]*BusiElem{}
	c.Busi4Http = map[string]*BusiElem{}
	for _, v := range c.Busi {
		for _, vv := range v {
			vv.Path = strings.TrimSuffix(vv.Path, "/")
			vv.Action = strings.Trim(vv.Action, "/")
			if vv.Yar {
				if c.Busi4Yar[vv.Path] == nil {
					c.Busi4Yar[vv.Path] = map[string]*BusiElem{}
				}
				c.Busi4Yar[vv.Path][vv.Action] = vv
			} else {
				if vv.Action == "" {
					c.Busi4Http[vv.Path] = vv
				} else {
					c.Busi4Http[vv.Path+"/"+vv.Action] = vv
				}
			}
		}
	}
	for k := range c.Busi4Yar {
		if c.Busi4Http[k] != nil {
			return fmt.Errorf("busi have same key, please check: %s", k)
		}
	}

	Set(c)

	log.Printf("Env: %s\nC: %s\n", Env, utils.Iprint(c))
	return
}

func Cgi(w http.ResponseWriter, r *http.Request) {
	fun := "conf.Cgi"
	fcontent := []byte{}
	defer func() {
		w.Write(fcontent)
	}()

	fcontent, err := getcontents()
	if err != nil {
		log.Printf("%s read file error: %v", fun, err)
		return
	}

	return
}

func init() {
	flag.StringVar(&Env, "env", "prod", "set env")
	flag.Parse()

	fcontent, err := getcontents()
	if err != nil {
		log.Printf("get conf file contents error: %v\n", err)
		os.Exit(-1)
	}
	err = parse(fcontent)
	if err != nil {
		log.Printf("parse conf file error: %v\n", err)
		os.Exit(-1)
	}

	go reloadConf()
}
