package srv

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"

	"encoding/json"

	"github.com/simplejia/clog"
	"github.com/simplejia/lc"
	"github.com/simplejia/sfp/conf"
	"github.com/simplejia/utils"
)

func ReqHttp(w http.ResponseWriter, r *http.Request) (code int) {
	fun := "srv.ReqHttp"
	code = http.StatusOK
	uri := r.RequestURI
	path := strings.TrimSuffix(r.URL.Path, "/")
	c := conf.Get()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		clog.Error("%s uri: %s, err: %v", fun, uri, err)
		code = http.StatusBadRequest
		return
	}

	busiElem := c.Busi4Http[path]
	if busiElem == nil {
		clog.Error("%s uri: %s, err: not found", fun, uri)
		code = http.StatusNotFound
		return
	}

	chData := &ChData{
		Uri:    uri,
		Elem:   busiElem,
		Body:   body,
		Method: r.Method,
	}
	var value interface{}
	if busiElem.Read { // read
		x := body
		if busiElem.GetDemote() {
			if excludeKey := busiElem.DemoteExcludeKey; len(excludeKey) > 0 {
				var y map[string]interface{}
				err := json.Unmarshal(body, &y)
				if err != nil {
					clog.Debug("%s uri: %s, err: %s, req body: %s", fun, uri, err, body)
				} else {
					if y != nil {
						for _, ek := range excludeKey {
							delete(y, ek)
						}
						x = []byte(fmt.Sprintf("%v", y))
					}
				}
			}
		}
		ps := []byte(string(x))
		sort.Slice(ps, func(i, j int) bool { return ps[i] < ps[j] })
		key := fmt.Sprintf(
			"%s_%s",
			uri,
			ps,
		)
		valueLc, ok := lc.Get(key)
		if !ok {
			_value := TransHttp(chData)
			if _value != nil {
				value = _value
			} else {
				value = valueLc
			}
			expire, _ := time.ParseDuration(busiElem.Expire)
			lc.Set(key, value, expire)
		} else {
			value = valueLc
		}
	} else { // write
		if !busiElem.Async {
			value = TransHttp(chData)
		} else {
			if busiElem.GetDemote() {
				AT.AddDemote(chData)
			} else {
				AT.Add(chData)
			}
		}
	}

	if value == nil {
		value = busiElem.Ret
	}

	switch v := value.(type) {
	case []byte:
		_, err = w.Write(v)
	default:
		err = json.NewEncoder(w).Encode(v)
	}
	if err != nil {
		clog.Error("%s write rsp error, uri: %s, value: %v, err: %v", fun, uri, value, err)
		code = http.StatusBadRequest
		return
	}
	return
}

func TransHttp(data *ChData) interface{} {
	fun := "srv.TransHttp"
	defer func() {
		if err := recover(); err != nil {
			clog.Error("%s http error, err: %v, data: %v", fun, err, data)
		}
	}()

	var value interface{}
	newuri := fmt.Sprintf("http://%s/%s", data.Elem.Addr, strings.TrimPrefix(data.Uri, "/"))
	gpp := &utils.GPP{
		Uri:     newuri,
		Timeout: time.Millisecond * time.Duration(data.Elem.Timeout),
		Params:  data.Body,
	}
	if data.Elem.GetDemote() {
		gpp.Headers = map[string]string{DemoteHeaderKey: DemoteHeaderValue}
	}
	for step := -1; step < data.Elem.Retry && step < MaxRetry; step++ {
		var rsp []byte
		var err error
		if data.Method == http.MethodPost {
			rsp, err = utils.Post(gpp)
		} else {
			rsp, err = utils.Get(gpp)
		}
		if err != nil {
			clog.Error("%s http error, err: %v, gpp: %v, step: %d", fun, err, gpp, step)
			continue
		}

		clog.Debug("%s uri: %s, rsp: %s", fun, data.Uri, rsp)

		value = rsp
		break
	}
	return value
}
