package main

import (
	"reflect"
	"strings"
)

type Context struct {
	Data map[string]any
}

func (c *Context) Get(key string) (any, bool) {
	parts := strings.Split(key, ".")

	current := reflect.ValueOf(c.Data)

	for _, part := range parts {
		val := current.MapIndex(reflect.ValueOf(part))
		if !val.IsValid() {
			return nil, false
		}
		current = reflect.ValueOf(val.Interface())
	}
	return current.Interface(), true
}
