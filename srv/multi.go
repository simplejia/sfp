package srv

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/rpc"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"encoding/json"

	"bytes"

	"github.com/gyf19/yar-go/yar"
	"github.com/simplejia/clog/api"
	"github.com/simplejia/sfp/conf"
)

func map2map(v interface{}) (ret interface{}) {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)
	typ := val.Type()

	switch typ.Kind() {
	case reflect.Map:
		m := make(map[string]interface{})
		for _, mk := range val.MapKeys() {
			key := fmt.Sprintf("%v", mk.Interface())
			m[key] = map2map(val.MapIndex(mk).Interface())
		}
		ret = m
	default:
		ret = val.Interface()
	}

	return
}

type FakeWrite struct {
	http.ResponseWriter
	Buf bytes.Buffer
}

func (fw *FakeWrite) Write(data []byte) (int, error) {
	return fw.Buf.Write(data)
}

func (fw *FakeWrite) Bytes() []byte {
	return fw.Buf.Bytes()
}

func Multi(w http.ResponseWriter, r *http.Request) {
	fun := "srv.Multi"
	uri := r.URL.RequestURI()
	code := http.StatusOK
	c := conf.Get()
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

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		clog.Error("%s uri: %s, err: %v", fun, uri, err)
		code = http.StatusBadRequest
		return
	}

	var m map[string]interface{}
	err = json.Unmarshal(body, &m)
	if err != nil {
		clog.Error("%s uri: %s, body: %s, err: %v", fun, uri, body, err)
		code = http.StatusBadRequest
		return
	}

	keys := make([]string, 0, len(m))
	for path := range m {
		keys = append(keys, path)
	}

	vars := make([]json.RawMessage, len(m))

	var wg sync.WaitGroup
	wg.Add(len(m))
	for kpos := range keys {
		go func(kpos int) {
			path := keys[kpos]
			data := m[path]
			w := &FakeWrite{}

			defer func() {
				if err := recover(); err != nil {
					clog.Error("%s uri: %s, path: %s, data: %s, err: %v, stack: %s", fun, uri, path, data, err, debug.Stack())
				}
				wg.Done()
			}()

			if busi := c.Busi4Http[path]; busi != nil { // http
				_data, _ := json.Marshal(data)
				r, _ := http.NewRequest(http.MethodPost, path, bytes.NewReader(_data)) // TODO
				code = ReqHttp(w, r)
				if code != http.StatusOK {
					return
				}
				vars[kpos] = w.Bytes()
				return
			}
			if pos := strings.LastIndexByte(path, '/'); pos != -1 {
				_path := path[:pos]
				if c.Busi4Yar[_path] != nil { // yar
					serviceMethod := path[pos+1:]
					buf := &bytes.Buffer{}
					codec := yar.NewClientCodec(Nop{nil, buf}, "msgpack")
					rpcReq := &rpc.Request{
						ServiceMethod: serviceMethod,
					}
					_data := []interface{}{data}
					codec.WriteRequest(rpcReq, _data)
					r, _ := http.NewRequest(http.MethodPost, _path, buf)
					code = ReqYar(w, r)
					if code != http.StatusOK {
						return
					}

					codec = yar.NewClientCodec(Nop{ioutil.NopCloser(bytes.NewReader(w.Bytes())), nil}, "msgpack")
					rpcRsp := &rpc.Response{}
					codec.ReadResponseHeader(rpcRsp)
					var x interface{}
					codec.ReadResponseBody(&x)
					_x, _ := json.Marshal(map2map(x))
					vars[kpos] = _x
					return
				}
			}
			clog.Error("%s uri: %s, not found, path: %s", fun, uri, path)
			return
		}(kpos)
	}
	wg.Wait()

	rsps := map[string]json.RawMessage{}
	for kpos := range keys {
		rsps[keys[kpos]] = vars[kpos]
	}

	json.NewEncoder(w).Encode(rsps)

	return
}
