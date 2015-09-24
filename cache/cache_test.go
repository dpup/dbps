// Copyright 2015 Daniel Pupius

package cache

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

type original struct {
	Name string
}

func (o original) Dependencies() []CacheKey {
	return NoDeps
}

func (o original) String() string {
	return o.Name
}

type derived struct {
	Name  string
	Times int
}

func (d derived) Dependencies() []CacheKey {
	return []CacheKey{original{d.Name}}
}

func (d derived) String() string {
	return fmt.Sprintf("%s x %d", d.Name, d.Times)
}

func TestGetInvalidate(t *testing.T) {
	c := New("test1")
	i := 0
	c.RegisterFetcher(func(key original) ([]byte, error) {
		i++
		return []byte(key.Name + "xxxx"), nil
	})

	rv1, _ := c.Get(original{"Hello"})
	rv2, _ := c.Get(original{"Goodbye"})

	if string(rv1) != "Helloxxxx" {
		t.Errorf("rv1 was %s", rv1)
	}
	if string(rv2) != "Goodbyexxxx" {
		t.Errorf("rv2 was %s", rv2)
	}

	c.Get(original{"Hello"})
	c.Get(original{"Goodbye"})

	if i != 2 {
		t.Errorf("Expected fetcher to be called twice, was called %d times", i)
	}

	c.Invalidate(original{"Hello"})

	c.Get(original{"Hello"})
	c.Get(original{"Goodbye"})

	if i != 3 {
		t.Errorf("Expected fetcher to be called twice, was called %d times", i)
	}
}

func TestDependentGet(t *testing.T) {
	c := New("test2")
	oi := 0
	di := 0
	c.RegisterFetcher(func(key original) ([]byte, error) {
		oi++
		return []byte(key.Name + "x"), nil
	})
	c.RegisterFetcher(func(key derived) ([]byte, error) {
		di++
		o, _ := c.Get(original{key.Name})
		return []byte(strings.Repeat(string(o), key.Times)), nil
	})

	rv1, _ := c.Get(derived{"HI", 2})
	rv2, _ := c.Get(derived{"HI", 4})

	if string(rv1) != "HIxHIx" {
		t.Errorf("rv1 was %s", rv1)
	}

	if string(rv2) != "HIxHIxHIxHIx" {
		t.Errorf("rv2 was %s", rv2)
	}

	if oi != 1 {
		t.Errorf("Expected original fetcher to be called once, was called %d times", oi)
	}
	if di != 2 {
		t.Errorf("Expected derived fetcher to be called twice, was called %d times", di)
	}

	// Invalidating 'original' should also invalidate entry for 'derived'.
	c.Invalidate(original{"HI"})
	c.Get(derived{"HI", 2})

	if oi != 2 {
		t.Errorf("Expected original fetcher to be called twice, was called %d times", oi)
	}
	if di != 3 {
		t.Errorf("Expected derived fetcher to be called thrice, was called %d times", di)
	}
}

func TestBadFetcher_noArgs(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Error("There was no panic")
		}
	}()
	c := New("test3")
	c.RegisterFetcher(func() ([]byte, error) { return []byte{}, nil })
}

func TestBadFetcher_2args(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Error("There was no panic")
		}
	}()
	c := New("test4")
	c.RegisterFetcher(func(a, b original) ([]byte, error) { return []byte{}, nil })
}

func TestBadFetcher_badReturn1(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Error("There was no panic")
		}
	}()
	c := New("test5")
	c.RegisterFetcher(func(a original) (int, error) { return 1, nil })
}

func TestBadFetcher_badReturn2(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Error("There was no panic")
		}
	}()
	c := New("test6")
	c.RegisterFetcher(func(a original) ([]byte, int) { return []byte{}, 2 })
}

func TestBadFetcher_badReturn3(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Error("There was no panic")
		}
	}()
	c := New("test7")
	c.RegisterFetcher(func(a original) {})
}

func TestBadFetcher_badReturn4(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Error("There was no panic")
		}
	}()
	c := New("test8")
	c.RegisterFetcher(func(a original) []byte { return []byte{} })
}

var runs = 0

func BenchmarkCacheWithMisses(b *testing.B) {
	runs++
	c := New("bench" + strconv.Itoa(runs))
	c.RegisterFetcher(func(key original) ([]byte, error) {
		return []byte(key.Name), nil
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(original{strconv.Itoa(i)})
	}
}

func BenchmarkCacheWithHits(b *testing.B) {
	runs++
	c := New("bench" + strconv.Itoa(runs))
	c.RegisterFetcher(func(key original) ([]byte, error) {
		return []byte(key.Name), nil
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(original{"1"})
	}
}

func BenchmarkNormalMapWithMisses(b *testing.B) {
	m := make(map[original][]byte)
	for i := 0; i < b.N; i++ {
		name := strconv.Itoa(i)
		m[original{name}] = []byte(name)
	}
}
