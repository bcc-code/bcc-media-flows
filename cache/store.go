package cache

import (
	"sync"
	"time"
)

const DefaultTTL = time.Minute * 5

type entry struct {
	Expiry time.Time
	Value  any
}

var lock = sync.Mutex{}
var store = map[string]entry{}

var startJanitor = sync.OnceFunc(func() {
	j := &janitor{
		interval: time.Minute * 1,
	}
	go j.start()
})

type janitor struct {
	interval time.Duration
}

func (j *janitor) start() {
	ticker := time.NewTicker(j.interval)
	for {
		select {
		case <-ticker.C:
			j.clear()
		}
	}
}

func (j *janitor) clear() {
	lock.Lock()
	defer lock.Unlock()
	var deleteKeys []string
	for k, v := range store {
		if v.Expiry.Before(time.Now()) {
			deleteKeys = append(deleteKeys, k)
		}
	}
	for _, k := range deleteKeys {
		delete(store, k)
	}
}

func Get[T any](key string) *T {
	lock.Lock()
	i, ok := store[key]
	lock.Unlock()
	if ok && i.Expiry.After(time.Now()) {
		if v, ok := i.Value.(*T); ok {
			return v
		}
	}
	return nil
}

func Set[T any](key string, value *T, ttl time.Duration) {
	startJanitor()
	lock.Lock()
	defer lock.Unlock()
	store[key] = entry{
		Expiry: time.Now().Add(ttl),
		Value:  value,
	}
}

func GetOrSet[T any](key string, ttl time.Duration, factory func() (*T, error)) (*T, error) {
	v := Get[T](key)
	if v != nil {
		return v, nil
	}
	v, err := factory()
	if err != nil {
		return nil, err
	}
	Set(key, v, ttl)
	return v, nil
}
