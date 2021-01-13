package eval

import "io"

type Context struct {
	Vars map[string]interface{}
}

func NewContext() *Context {
	return &Context{
		Vars: make(map[string]interface{}),
	}
}

func (c *Context) SetVar(name string, value interface{}) {
	c.Vars[name] = value
}

func (c *Context) Var(varName string) interface{} {
	return c.Vars[varName]
}

func (c *Context) Close() {
	for _, v := range c.Vars {
		if vv, ok := v.(io.Closer); ok {
			vv.Close()
		}
	}
}
