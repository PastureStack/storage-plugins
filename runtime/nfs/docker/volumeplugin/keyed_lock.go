package volumeplugin

import "sync"

type keyedLocker struct {
	mu    sync.Mutex
	locks map[string]*keyedLock
}

type keyedLock struct {
	mu    sync.Mutex
	users int
}

func newKeyedLocker() *keyedLocker {
	return &keyedLocker{locks: make(map[string]*keyedLock)}
}

func (l *keyedLocker) Lock(key string) {
	l.mu.Lock()
	lock := l.locks[key]
	if lock == nil {
		lock = &keyedLock{}
		l.locks[key] = lock
	}
	lock.users++
	l.mu.Unlock()
	lock.mu.Lock()
}

func (l *keyedLocker) Unlock(key string) {
	l.mu.Lock()
	lock := l.locks[key]
	if lock == nil {
		l.mu.Unlock()
		panic("unlock of an unlocked volume key")
	}
	lock.users--
	if lock.users == 0 {
		delete(l.locks, key)
	}
	lock.mu.Unlock()
	l.mu.Unlock()
}
