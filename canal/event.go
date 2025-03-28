package canal

import (
	"sync"
	"time"

	"github.com/go-mysql-org/go-mysql/canal"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/siddontang/go-log/loggers"
)

type EventHeader = replication.EventHeader
type QueryEvent = replication.QueryEvent
type Position = mysql.Position
type GTIDSet = mysql.GTIDSet
type RowsEvent = canal.RowsEvent

// set event logic
func DefaultHandler() *defaultEventHandler {
	return &defaultEventHandler{ch: make(chan any, 4096)}
}

type gtidSave struct {
	gtidSet string
	force   bool
}

type defaultEventHandler struct {
	onRow       func(e *canal.RowsEvent) error
	onDDL       func(header *EventHeader, nextPos Position, queryEvent *QueryEvent) error
	onPosSynced func(header *EventHeader, pos Position, set GTIDSet, force bool) error
	canal       *canal.Canal
	ch          chan any
	*canal.DummyEventHandler
	MasterInfo MasterInfoInterface
	log        loggers.Advanced
}

func (h *defaultEventHandler) work(wd *sync.WaitGroup, log loggers.Advanced, interval int) {
	h.log = log
	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()
	defer wd.Done()
	lastsaveTime := time.Now()
	var gtidSet string
	for {
		var write bool
		select {
		case <-h.canal.Ctx().Done():
			time.Sleep(1 * time.Second)
			h.log.Info("work exit")

			return
		case <-ticker.C:
			write = true
		case v := <-h.ch:
			switch v := v.(type) {
			case gtidSave:
				now := time.Now()
				if v.force || now.Sub(lastsaveTime) > 3*time.Second {
					lastsaveTime = now
					write = v.force
					gtidSet = v.gtidSet
				}
			}
		}
		if write {
			err := h.MasterInfo.Save(gtidSet)
			if err != nil {
				h.log.Errorln("work MasterInfo.save", err)
				h.canal.Close()

			}

		}

	}

}
func (h *defaultEventHandler) setCanal(c *Canal) {
	h.canal = c
}

func (h *defaultEventHandler) SetOnDDl(fn func(header *EventHeader, nextPos Position, queryEvent *QueryEvent) error) {

	h.onDDL = fn
}
func (h *defaultEventHandler) SetOnRow(fn func(e *canal.RowsEvent) error) {
	h.onRow = fn
}

func (h *defaultEventHandler) SetOnPosSynced(fn func(header *EventHeader, pos Position, set GTIDSet, force bool) error) {
	h.onPosSynced = fn
}

func (h *defaultEventHandler) String() string { return "DefaultEventHandler" }

func (h *defaultEventHandler) OnPosSynced(header *EventHeader, pos Position, set GTIDSet, force bool) error {
	h.ch <- gtidSave{set.String(), force}
	if h.onPosSynced == nil {
		return h.canal.Ctx().Err()
	}
	return h.onPosSynced(header, pos, set, force)
}
func (h *defaultEventHandler) OnRow(e *canal.RowsEvent) error {

	if h.onRow == nil {
		return h.canal.Ctx().Err()
	}
	return h.onRow(e)
}

func (h *defaultEventHandler) OnDDL(header *replication.EventHeader, nextPos Position, queryEvent *QueryEvent) error {

	h.ch <- gtidSave{queryEvent.GSet.String(), true}

	if h.onDDL == nil {
		return h.canal.Ctx().Err()
	}

	return h.onDDL(header, nextPos, queryEvent)
}
