package srv

import (
	"io"

	"github.com/simplejia/clog"
	"github.com/simplejia/sfp/conf"
)

const (
	MaxRetry          = 3
	DemoteHeaderKey   = "__SFP-DEMOTE__"
	DemoteHeaderValue = "YES"
)

type Nop struct {
	io.ReadCloser
	io.Writer
}

type ChData struct {
	Uri    string
	Elem   *conf.BusiElem
	Body   []byte
	Yar    bool
	Demote bool
	Method string
}

type AsyncT struct {
	Ch       chan *ChData
	ChDemote chan *ChData
}

func (asyncT *AsyncT) Init() {
	c := conf.Get()
	asyncT.Ch = make(chan *ChData, c.Cons.ChanCap)
	asyncT.ChDemote = make(chan *ChData, c.Cons.ChanCap)

	for i := 0; i < c.Cons.RoutineNum; i++ {
		go asyncT.Proc()
	}
	go asyncT.ProcDemote()
}

func (asyncT *AsyncT) Proc() {
	for data := range asyncT.Ch {
		if data.Yar {
			TransYar(data)
		} else {
			TransHttp(data)
		}
	}
}

func (asyncT *AsyncT) ProcDemote() {
	for data := range asyncT.ChDemote {
		if data.Yar {
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

func (asyncT *AsyncT) AddDemote(chData *ChData) {
	fun := "srv.AsyncT.AddDemote"
	chData.Demote = true
	select {
	case asyncT.ChDemote <- chData:
	default:
		clog.Error("%s chan full, data: %v", fun, chData)
	}
}

var AT AsyncT

func init() {
	AT.Init()
}
