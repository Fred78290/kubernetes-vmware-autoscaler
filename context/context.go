package context

import (
	"context"
	"time"
)

// Context wrapper
type Context struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func TODO() context.Context {
	return context.TODO()
}

func Background() context.Context {
	return context.Background()
}

// NewContext return a context. Timeout is in seconds
func NewContext(timeout time.Duration) *Context {
	ctx, cancel := context.WithTimeout(context.Background(), timeout*time.Second)
	return &Context{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Deadline returns the time when work done on behalf of this context
// should be canceled. Deadline returns ok==false when no deadline is
// set. Successive calls to Deadline return the same results.
func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return c.ctx.Deadline()
}

// Done returns a channel that's closed when work done on behalf of this
// context should be canceled. Done may return nil if this context can
// never be canceled. Successive calls to Done return the same value.
//
// WithCancel arranges for Done to be closed when cancel is called;
// WithDeadline arranges for Done to be closed when the deadline
// expires; WithTimeout arranges for Done to be closed when the timeout
// elapses.
//
// Done is provided for use in select statements:
//
//  // Stream generates values with DoSomething and sends them to out
//  // until DoSomething returns an error or ctx.Done is closed.
//  func Stream(ctx context.Context, out chan<- Value) error {
//  	for {
//  		v, err := DoSomething(ctx)
//  		if err != nil {
//  			return err
//  		}
//  		select {
//  		case <-ctx.Done():
//  			return ctx.Err()
//  		case out <- v:
//  		}
//  	}
//  }
//
// See https://blog.golang.org/pipelines for more examples of how to use
// a Done channel for cancelation.
func (c *Context) Done() <-chan struct{} {
	return c.ctx.Done()
}

// Err as string
// If Done is not yet closed, Err returns nil.
// If Done is closed, Err returns a non-nil error explaining why:
// Canceled if the context was canceled
// or DeadlineExceeded if the context's deadline passed.
// After Err returns a non-nil error, successive calls to Err return the same error.
func (c *Context) Err() error {
	return c.ctx.Err()
}

// Value returns the value associated with this context for key, or nil
// if no value is associated with key. Successive calls to Value with
// the same key returns the same result.
//
// Use context values only for request-scoped data that transits
// processes and API boundaries, not for passing optional parameters to
// functions.
//
// A key identifies a specific value in a Context. Functions that wish
// to store values in Context typically allocate a key in a global
// variable then use that key as the argument to context.WithValue and
// Context.Value. A key can be any type that supports equality;
// packages should define keys as an unexported type to avoid
// collisions.
//
// Packages that define a Context key should provide type-safe accessors
// for the values stored using that key:
//
// 	// Package user defines a User type that's stored in Contexts.
// 	package user
//
// 	import "context"
//
// 	// User is the type of value stored in the Contexts.
// 	type User struct {...}
//
// 	// key is an unexported type for keys defined in this package.
// 	// This prevents collisions with keys defined in other packages.
// 	type key int
//
// 	// userKey is the key for user.User values in Contexts. It is
// 	// unexported; clients use user.NewContext and user.FromContext
// 	// instead of using this key directly.
// 	var userKey key
//
// 	// NewContext returns a new Context that carries value u.
// 	func NewContext(ctx context.Context, u *User) context.Context {
// 		return context.WithValue(ctx, userKey, u)
// 	}
//
// 	// FromContext returns the User value stored in ctx, if any.
// 	func FromContext(ctx context.Context) (*User, bool) {
// 		u, ok := ctx.Value(userKey).(*User)
// 		return u, ok
// 	}
func (c *Context) Value(key interface{}) interface{} {
	return c.ctx.Value(key)
}

// Cancel the current context
func (c *Context) Cancel() {
	c.cancel()
}

// WithValue returns a copy of parent in which the value associated with key is
// val.
//
// Use context Values only for request-scoped data that transits processes and
// APIs, not for passing optional parameters to functions.
//
// The provided key must be comparable and should not be of type
// string or any other built-in type to avoid collisions between
// packages using context. Users of WithValue should define their own
// types for keys. To avoid allocating when assigning to an
// interface{}, context keys often have concrete type
// struct{}. Alternatively, exported context key variables' static
// type should be a pointer or interface.
func (c *Context) WithValue(key, val interface{}) {
	c.ctx = context.WithValue(c.ctx, key, val)
}

// Context returns the embed context object
func (c *Context) Context() context.Context {
	return c.ctx
}
