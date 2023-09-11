package cache

import (
	"sync"
	"time"
)

type entry struct {
	Expiry time.Time
	Value  any
}

var lock = sync.Mutex{}
var store = map[string]entry{}

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

func init() {
	j := &janitor{
		interval: time.Minute * 1,
	}
	go j.start()
}

func Get[T any](key string) *T {
	i, ok := store[key]
	if ok && i.Expiry.After(time.Now()) {
		return i.Value.(*T)
	}
	return nil
}

func Set[T any](key string, value *T) {
	lock.Lock()
	defer lock.Unlock()
	store[key] = entry{
		Expiry: time.Now().Add(time.Minute * 5),
		Value:  value,
	}
}

func GetOrSet[T any](key string, factory func() (*T, error)) (*T, error) {
	v := Get[T](key)
	if v != nil {
		return v, nil
	}
	v, err := factory()
	if err != nil {
		return nil, err
	}
	Set(key, v)
	return v, nil
}
