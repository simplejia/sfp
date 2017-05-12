package srv

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/rpc"
	"sort"
	"strings"
	"time"

	"github.com/gyf19/yar-go/yar"
	"github.com/simplejia/clog"
	"github.com/simplejia/lc"
	"github.com/simplejia/sfp/conf"
	"github.com/simplejia/utils"
)

func ReqYar(w http.ResponseWriter, r *http.Request) (code int) {
	fun := "srv.ReqYar"
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
	codec := yar.NewServerCodec(Nop{ioutil.NopCloser(bytes.NewReader(body)), w})
	rpcReq := &rpc.Request{}
	if err := codec.ReadRequestHeader(rpcReq); err != nil {
		clog.Error("%s codec.ReadRequestHeader uri: %s, err: %v", fun, uri, err)
		code = http.StatusBadRequest
		return
	}

	clog.Debug("%s uri: %s, req: %v", fun, uri, rpcReq)

	busi := c.Busi4Yar[path]
	if busi == nil {
		clog.Error("%s uri: %s, err: not found", fun, uri)
		code = http.StatusNotFound
		return
	}

	busiElem := busi[rpcReq.ServiceMethod]
	if busiElem == nil {
		clog.Error("%s uri: %s, req: %v, err: not found", fun, uri, rpcReq)
		code = http.StatusNotFound
		return
	}

	demote := busiElem.GetDemote()

	var value interface{}
	if !demote {
		chData := &ChData{
			Uri:    uri,
			Elem:   busiElem,
			Body:   body,
			Method: r.Method,
		}
		if busiElem.Read { // read
			var x interface{}
			if err := codec.ReadRequestBody(&x); err != nil {
				clog.Error("%s codec.ReadRequestBody uri: %s, err: %v", fun, uri, err)
				code = http.StatusBadRequest
				return
			}
			clog.Debug("%s uri: %s, req body: %v", fun, uri, x)

			y := strings.Split(fmt.Sprintf("%v", x), " ")
			sort.Strings(y)
			key := fmt.Sprintf(
				"%s_%s_%s",
				uri,
				rpcReq.ServiceMethod,
				strings.Join(y, " "),
			)
			valueLc, ok := lc.Get(key)
			if !ok {
				_value := TransYar(chData)
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
			if busiElem.Async {
				AT.Add(chData)
			} else {
				value = TransYar(chData)
			}
		}
	}

	if value == nil {
		value = busiElem.Ret
	}
	rpcRsp := &rpc.Response{
		Seq: rpcReq.Seq,
	}
	err = codec.WriteResponse(rpcRsp, value)
	if err != nil {
		clog.Error("%s codec.WriteResponse uri: %s, value: %v, err: %v", fun, uri, value, err)
		code = http.StatusBadRequest
		return
	}
	return
}

func stringYar(body []byte) string {
	codec := yar.NewServerCodec(Nop{ioutil.NopCloser(bytes.NewReader(body)), ioutil.Discard})
	rpcReq := &rpc.Request{}
	if err := codec.ReadRequestHeader(rpcReq); err != nil {
		// not expected
		return ""
	}
	var x interface{}
	if err := codec.ReadRequestBody(&x); err != nil {
		// not expected
		return ""
	}
	return fmt.Sprintf("[%s] %v", rpcReq.ServiceMethod, x)
}

func TransYar(data *ChData) interface{} {
	fun := "srv.TransYar"
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

	for step := -1; step < data.Elem.Retry && step < MaxRetry; step++ {
		var rsp []byte
		var err error
		if data.Method == http.MethodPost {
			rsp, err = utils.Post(gpp)
		} else {
			rsp, err = utils.Get(gpp)
		}

		if err != nil {
			clog.Error("%s http error, err: %v, uri: %s, params: %s, step: %d", fun, err, data.Uri, stringYar(data.Body), step)
			continue
		}
		codec := yar.NewClientCodec(Nop{ioutil.NopCloser(bytes.NewReader(rsp)), ioutil.Discard}, "msgpack")
		rpcRsp := &rpc.Response{}
		if err := codec.ReadResponseHeader(rpcRsp); err != nil {
			clog.Error("%s codec.ReadRequestHeader uri: %s, params: %s, err: %v", fun, data.Uri, stringYar(data.Body), err)
			break
		}

		clog.Debug("%s uri: %s, params: %s, rsp: %v", fun, data.Uri, stringYar(data.Body), rpcRsp)

		var x interface{}
		if err := codec.ReadResponseBody(&x); err != nil {
			clog.Error("%s codec.ReadRequestBody uri: %s, params: %s, err: %v", fun, data.Uri, stringYar(data.Body), err)
			break
		}

		clog.Debug("%s uri: %s, params: %s, rsp body: %v", fun, data.Uri, stringYar(data.Body), x)

		value = x
		break
	}
	return value
}
