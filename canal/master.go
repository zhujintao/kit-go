package canal

import (
	"io"
	"os"
	"path"
	"sync"
	"time"

	"tailscale.com/atomicfile"
)

type MasterInfoInterface interface {
	Load() (string, error)
	Save(string) error
	Init(path *string, id string) error
	Close() error
}

type masterInfo struct {
	sync.RWMutex
	Gtid         string
	lastsaveTime time.Time
	filePath     string
}

func (m *masterInfo) Save(set string) error {

	m.Lock()
	defer m.Unlock()

	m.Gtid = set
	now := time.Now()
	if now.Sub(m.lastsaveTime) < time.Second {
		return nil
	}
	m.lastsaveTime = now

	return atomicfile.WriteFile(m.filePath, []byte(m.Gtid), 0644)

}

func (m *masterInfo) Close() error {
	return m.Save(m.Gtid)
}

func (m *masterInfo) Init(dir *string, id string) error {

	pdir := path.Join(*dir, id)
	m.filePath = path.Join(*dir, id, "master.info")
	if err := os.MkdirAll(pdir, 0755); err != nil {
		return err
	}

	t, err := os.MkdirTemp(pdir, "check")
	if err != nil {
		return err
	}
	*dir = pdir
	os.Remove(t)
	return nil
}
func (m *masterInfo) Load() (string, error) {

	f, err := os.Open(m.filePath)
	if err != nil {
		return "", nil
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	m.Gtid = string(b)
	return m.Gtid, nil

}
