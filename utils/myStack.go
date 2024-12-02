package utils

import (
	"fmt"
	"sync"
)

type MyStack struct {
	Stack []interface{}
	CType string
	lock  sync.RWMutex
	Len   int
}

func (c *MyStack) Push(obj interface{}) {
	c.lock.Lock()
	if c.Len == len(c.Stack) {
		c.Stack = c.Stack[1:]
	}
	defer c.lock.Unlock()
	c.Stack = append(c.Stack, obj)
}

func (c *MyStack) Pop() error {
	len := len(c.Stack)
	if len > 0 {
		c.lock.Lock()
		defer c.lock.Unlock()
		c.Stack = c.Stack[:len-1]
		return nil
	}
	return fmt.Errorf("Pop Error: Stack is empty")
}

func (c *MyStack) Front() (interface{}, error) {
	len := len(c.Stack)
	if len > 0 {
		c.lock.Lock()
		defer c.lock.Unlock()
		return c.Stack[len-1], nil
	}
	return "", fmt.Errorf("Peep Error: Stack is empty")
}

func (c *MyStack) Size() int {
	return len(c.Stack)
}

func (c *MyStack) Empty() bool {
	return len(c.Stack) == 0
}
