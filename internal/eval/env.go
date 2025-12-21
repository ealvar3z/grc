package eval

import (
	"strconv"

	"grc/internal/parse"
)

// Env holds rc-style environment variables with list values.
type Env struct {
	parent *Env
	vars   map[string][]string
	funcs  map[string]FuncDef
}

// FuncDef stores a function definition.
type FuncDef struct {
	Name string
	Body *parse.Node
}

// NewEnv constructs an environment, optionally inheriting from parent.
func NewEnv(parent *Env) *Env {
	return &Env{parent: parent, vars: make(map[string][]string)}
}

// NewChild creates a child environment that inherits from parent.
func NewChild(parent *Env) *Env {
	return NewEnv(parent)
}

// Get returns the value for name, searching parents if needed.
func (e *Env) Get(name string) []string {
	if e == nil {
		return nil
	}
	if v, ok := e.vars[name]; ok {
		return v
	}
	if e.parent != nil {
		return e.parent.Get(name)
	}
	return nil
}

// Set assigns the variable to the provided list.
func (e *Env) Set(name string, vals []string) {
	if e == nil {
		return
	}
	if e.vars == nil {
		e.vars = make(map[string][]string)
	}
	e.vars[name] = vals
}

// Set1 assigns a single value to the variable.
func (e *Env) Set1(name, value string) {
	e.Set(name, []string{value})
}

// Unset removes a variable from the current environment.
func (e *Env) Unset(name string) {
	if e == nil || e.vars == nil {
		return
	}
	delete(e.vars, name)
}

// SetFunc defines a function in the current environment.
func (e *Env) SetFunc(name string, body *parse.Node) {
	if e == nil {
		return
	}
	if e.funcs == nil {
		e.funcs = make(map[string]FuncDef)
	}
	e.funcs[name] = FuncDef{Name: name, Body: body}
}

// GetFunc looks up a function, searching parent environments.
func (e *Env) GetFunc(name string) (FuncDef, bool) {
	if e == nil {
		return FuncDef{}, false
	}
	if e.funcs != nil {
		if def, ok := e.funcs[name]; ok {
			return def, true
		}
	}
	if e.parent != nil {
		return e.parent.GetFunc(name)
	}
	return FuncDef{}, false
}

// UnsetFunc removes a function from the current environment.
func (e *Env) UnsetFunc(name string) {
	if e == nil || e.funcs == nil {
		return
	}
	delete(e.funcs, name)
}

// SetStatus sets the status variable to the numeric exit code.
func (e *Env) SetStatus(code int) {
	e.Set("status", []string{strconv.Itoa(code)})
}

// GetStatus returns the numeric status value or 0 if unset/invalid.
func (e *Env) GetStatus() int {
	vals := e.Get("status")
	if len(vals) == 0 {
		return 0
	}
	n, err := strconv.Atoi(vals[0])
	if err != nil {
		return 0
	}
	return n
}
