package srv

import (
	"io"

	"github.com/simplejia/clog/api"
	"github.com/simplejia/sfp/conf"
)

const (
	MaxRetry = 3
)

type Nop struct {
	io.ReadCloser
	io.Writer
}

type ChData struct {
	Uri    string
	Body   []byte
	Method string
	Elem   *conf.BusiElem
}

type AsyncT struct {
	Ch chan *ChData
}

func (asyncT *AsyncT) Init() {
	c := conf.Get()
	asyncT.Ch = make(chan *ChData, c.Cons.ChanCap)

	for i := 0; i < c.Cons.RoutineNum; i++ {
		go asyncT.Proc()
	}
}

func (asyncT *AsyncT) Proc() {
	for data := range asyncT.Ch {
		if data.Elem.Yar {
			TransYar(data)
		} else {
			TransHttp(data)
		}
	}
}

func (asyncT *AsyncT) Add(chData *ChData) {
	fun := "srv.AsyncT.Add"
	select {
	case asyncT.Ch <- chData:
	default:
		clog.Error("%s chan full, data: %v", fun, chData)
	}
}

var AT AsyncT

func init() {
	AT.Init()
}
