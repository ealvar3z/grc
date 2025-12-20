package eval

// Env holds rc-style environment variables with list values.
type Env struct {
	parent *Env
	vars   map[string][]string
}

// NewEnv constructs an environment, optionally inheriting from parent.
func NewEnv(parent *Env) *Env {
	return &Env{parent: parent, vars: make(map[string][]string)}
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

// Unset removes a variable from the current environment.
func (e *Env) Unset(name string) {
	if e == nil || e.vars == nil {
		return
	}
	delete(e.vars, name)
}
