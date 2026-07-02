package main

import "strings"

type Context struct {
	Data map[string]any
}

func (c *Context) Get(key string) (any, bool) {
	parts := strings.Split(key, ".")
	current := c.Data
	for i, part := range parts {
		if i == len(parts)-1 { // final part (key)
			value, ok := current[part]
			return value, ok
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			return nil, false
		}
		current = next
	}
	return nil, false
}
