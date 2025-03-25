package canal

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/juju/errors"
	"github.com/siddontang/go/ioutil2"
)

type masterInfo struct {
	sync.RWMutex
	GtidSet      string `toml:"gtid_set"`
	filePath     string
	lastSaveTime time.Time
}

func loadMasterInfo(dataDir string) (*masterInfo, error) {
	var m masterInfo

	if len(dataDir) == 0 {
		dataDir = "."
		//return &m, nil
	}

	m.filePath = path.Join(dataDir, "master.info")
	m.lastSaveTime = time.Now()

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, errors.Trace(err)
	}

	f, err := os.Open(m.filePath)
	if err != nil && !os.IsNotExist(errors.Cause(err)) {
		return nil, errors.Trace(err)
	} else if os.IsNotExist(errors.Cause(err)) {
		return &m, nil
	}
	defer f.Close()

	_, err = toml.NewDecoder(f).Decode(&m)
	return &m, errors.Trace(err)
}

func (m *masterInfo) Save(gset string) error {
	m.Lock()
	defer m.Unlock()
	m.GtidSet = gset
	if len(m.filePath) == 0 {
		return nil
	}

	n := time.Now()
	if n.Sub(m.lastSaveTime) < time.Second {
		return nil
	}

	m.lastSaveTime = n
	var buf bytes.Buffer
	e := toml.NewEncoder(&buf)

	e.Encode(m)

	var err error
	if err = ioutil2.WriteFileAtomic(m.filePath, buf.Bytes(), 0644); err != nil {
		fmt.Printf("canal save master info to file %s err %v", m.filePath, err)
	}

	return errors.Trace(err)
}

func (m *masterInfo) Gtidset() string {
	m.RLock()
	defer m.RUnlock()

	return m.GtidSet
}

func (m *masterInfo) Close() error {
	gset := m.Gtidset()
	return m.Save(gset)
}
