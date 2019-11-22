package resourcery

import (
	"context"
	"errors"
)

//Action is used with NotifyFunc to indicate a Resource being Added to
//or Removed from a pool.
type Action int

const (
	//ResourceAdded indicates a resource was added to the pool
	ResourceAdded Action = iota
	//ResourceRequested indicates a resource was requested from the pool
	ResourceRequested
)

//Resource can be anything that is to be managed by resourcery.
type Resource interface {
	IsHealthy() bool
	Terminate()
}

//NewPool creates a new ResourceWizard to manage resources.
func NewPool() *Pool {
	return &Pool{
		resourceQueue: make(chan Resource),
		resourceCount: 0,
	}
}

//NotifyFunc is called when a Resource is added to or removed from the Pool.
type NotifyFunc func(action Action)

//NewManagedPool is used by ResourceWizard internally.
func NewManagedPool(notifyFunc NotifyFunc) *Pool {
	p := NewPool()
	p.NotifyFunc = notifyFunc
	return p
}

//Pool manages Resources.
type Pool struct {
	NotifyFunc    NotifyFunc
	resourceQueue chan Resource
	resourceCount int
}

//AddResource puts a resource into the Pool control.
func (p *Pool) AddResource(res Resource) error {
	if !res.IsHealthy() {
		return errors.New("Cannot add unhealthy resources to the pool")
	}
	p.resourceCount++
	//If there is a notifyFunc, send the notification
	if p.NotifyFunc != nil {
		go p.NotifyFunc(ResourceAdded)
	}
	go func() {
		p.resourceQueue <- res
	}()
	return nil
}

//GetResource blocks until a Resource is available & returns the next healthy
//Resource it finds. It also calls terminate on any unhealthy Resources found.
//ctx is a context.Context to allow deadlines / timeouts to be specified.
func (p *Pool) GetResource(ctx context.Context) (res Resource, err error) {
	//If there is a notifyFunc, send the notification
	if p.NotifyFunc != nil {
		go p.NotifyFunc(ResourceRequested)
	}
resLoop:
	for res == nil {
		select {
		case res = <-p.resourceQueue:
			p.resourceCount--
			if res.IsHealthy() {
				break resLoop
			}
			//Terminate unhealthy resources & set res back to nil to get the next one.
			res.Terminate()
			res = nil

		case <-ctx.Done():
			err = ctx.Err()
			break resLoop
		}
	}
	return res, err
}

//Shutdown calls Terminate on all resources.
func (p *Pool) Shutdown() {
	for {
		select {
		case res := <-p.resourceQueue:
			res.Terminate()
		default:
			return
		}
	}
}

//Size returns the current size of the pool.
func (p *Pool) Size() int {
	return p.resourceCount
}

//ResourceFactory is a function for creating new Resource instances.
type ResourceFactory func() (Resource, error)

//Wizard manages a Pool
type Wizard interface {
	GetPool() *Pool
}
