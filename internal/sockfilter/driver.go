package sockfilter

import (
	"errors"
	"sync"
)

const managedDriverName = "sgfg_sockfilter"

// Kept separate from SCM so rollback and teardown order are testable without
// installing a kernel driver. Remove must never run if Stop failed.
type driverBackend interface {
	Install() error
	Start() error
	Stop() error
	Remove() error
	Close()
}

type driverLease struct {
	backend driverBackend
	once    sync.Once
	err     error
}

func startManagedDriver(b driverBackend) (*driverLease, error) {
	if err := b.Install(); err != nil {
		b.Close()
		return nil, err
	}
	l := &driverLease{backend: b}
	if err := b.Start(); err != nil {
		return nil, errors.Join(err, l.Close())
	}
	return l, nil
}

func (l *driverLease) Close() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		defer l.backend.Close()
		if err := l.backend.Stop(); err != nil {
			l.err = err
			return
		}
		l.err = l.backend.Remove()
	})
	return l.err
}
