// Copyright 2015 Daniel Pupius

package cache

var NoDeps = []CacheKey{}

type FetchFn func(CacheKey) ([]byte, error)

type CacheKey interface {
	Dependencies() []CacheKey
	String() string
}

type StrKey string

func (str StrKey) Dependencies() []CacheKey {
	return NoDeps
}

func (str StrKey) String() string {
	return string(str)
}
