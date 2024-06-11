package etcd

import (
	"context"
	"fmt"
	"net"
	"time"

	etcdv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

const lastIPPrefix = "last_reserved_ip."

type Store struct {
	l      *concurrency.Mutex
	etcd   *concurrency.Session
	prefix string
	ctx    context.Context
}

func New(vlanId int, etcdEndpoint []string) (*Store, error) {

	cli, err := etcdv3.New(etcdv3.Config{Endpoints: etcdEndpoint, DialTimeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	prefix := fmt.Sprintf("ipam/%d/", vlanId)
	s, err := concurrency.NewSession(cli)
	if err != nil {
		return nil, err
	}
	l := concurrency.NewMutex(s, prefix)

	return &Store{etcd: s, l: l, prefix: prefix, ctx: context.Background()}, nil

}

func (s *Store) LastReservedIP(rangeID string) (net.IP, error) {
	fpath := s.prefix + lastIPPrefix + rangeID
	result, err := s.etcd.Client().Get(s.ctx, fpath)
	if err != nil {
		return nil, err
	}
	if len(result.Kvs) == 0 {
		return nil, fmt.Errorf("not found")
	}
	return net.ParseIP(string(result.Kvs[0].Value)), nil
}

func (s *Store) Close() error {
	return s.etcd.Close()
}
func (s *Store) Lock() error {
	return s.l.Lock(s.ctx)
}
func (s *Store) Unlock() error {
	return s.l.Unlock(s.ctx)
}

func (s *Store) Reserve(id string, ifname string, ip net.IP, rangeID string) (bool, error) {

	fpath := s.prefix + id + "/" + ifname
	_, err := s.etcd.Client().Put(s.ctx, fpath, ip.String())
	if err != nil {
		return false, err
	}

	ippath := s.prefix + lastIPPrefix + rangeID
	_, err = s.etcd.Client().Put(s.ctx, ippath, ip.String())
	if err != nil {
		return false, err
	}
	return true, nil

}

func (s *Store) GetByID(id string, ifname string) []net.IP {

	fpath := s.prefix + id + "/" + ifname
	var ips []net.IP
	result, err := s.etcd.Client().Get(s.ctx, fpath)
	if err != nil {
		return nil
	}

	if len(result.Kvs) == 0 {
		return nil
	}

	if ip := net.ParseIP(string(result.Kvs[0].Value)); ip != nil {
		ips = append(ips, ip)
	}
	return ips
}

func (s *Store) ReleaseByID(id string, ifname string) error {

	return nil
}
func (s *Store) FindByID(id string, ifname string) bool {

	s.l.Lock(s.ctx)
	defer s.l.Unlock(s.ctx)
	fpath := s.prefix + id + "/" + ifname

	_, err := s.etcd.Client().Get(s.ctx, fpath)
	if err != nil {
		return false
	}

	return true

}
