package resourcery

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

//Resource can be anything that is to be managed by resourcery.
type Resource interface {
	IsHealthy() bool
	Terminate()
}

//NewPool creates a new Pool to manage resources.
func NewPool() *Pool {
	return &Pool{
		resourceQueue: make(chan Resource),
		resourceCount: 0,
	}
}

//Action is used with MonitorFunc to indicate a Resource being Added to
//or Removed from a pool.
type Action int

const (
	//ResourceAdded indicates a resource was added to the pool
	ResourceAdded Action = iota
	//ResourceRequested indicates a resource was requested from the pool
	ResourceRequested
	//UnhealthyResourceTerminated indicates an unhealthy resourfce was terminated.
	UnhealthyResourceTerminated
	//Shutdown indicates that the pool is shutting down and will call terminate
	//on all resources.
	Shutdown
)

//ActionMsg is used with MonitorFunc to inform the wizard of an event.
type ActionMsg struct {
	Time   time.Time
	Action Action
}

//MonitorFunc is called when a Resource is added to or removed from the Pool.
type MonitorFunc func(msg ActionMsg)

//NewMonitoredPool can be used if you want to monitor Resources being added to,
//or removed from the Pool to allow alerting / increasing of the pool size.
func NewMonitoredPool(monitorFunc MonitorFunc) *Pool {
	p := NewPool()
	p.MonitorFunc = monitorFunc
	return p
}

//Pool manages Resources.
type Pool struct {
	MonitorFunc   MonitorFunc
	resourceQueue chan Resource
	mutex         sync.RWMutex
	resourceCount int
}

//AddResource puts a resource into the Pool control.
func (p *Pool) AddResource(res Resource) error {
	if !res.IsHealthy() {
		return errors.New("Cannot add unhealthy resources to the pool")
	}
	p.mutex.Lock()
	p.resourceCount++
	p.mutex.Unlock()
	//If there is a notifyFunc, send the notification
	if p.MonitorFunc != nil {
		go p.MonitorFunc(newActionMsg(ResourceAdded))
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
	//If there is a monitorFunc, send the notification
	if p.MonitorFunc != nil {
		go p.MonitorFunc(newActionMsg(ResourceRequested))
	}
resLoop:
	for res == nil {
		select {
		case res = <-p.resourceQueue:
			p.mutex.Lock()
			p.resourceCount--
			p.mutex.Unlock()

			if res.IsHealthy() {
				break resLoop
			}
			//Terminate unhealthy resources & set res back to nil to get the next one.
			if p.MonitorFunc != nil {
				go p.MonitorFunc(newActionMsg(UnhealthyResourceTerminated))
			}
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
	if p.MonitorFunc != nil {
		go p.MonitorFunc(newActionMsg(Shutdown))
	}
	for {
		select {
		case res := <-p.resourceQueue:
			//Don't terminate in goroutine or caller may close the program before we
			//have had chance to call Terminate on all resources.
			res.Terminate()
		default:
			return
		}
	}
}

//Size returns the current size of the pool.
func (p *Pool) Size() int {
	//Not bothering to use the mutex for reading as the value changes all the time
	//anyway - Size() only returns a snapshot of the size at a point in time.
	//
	return p.resourceCount
}

//newActionMsg just creates a new ActionMsg to reduce code above.
func newActionMsg(action Action) ActionMsg {
	return ActionMsg{
		Action: action,
		Time:   time.Now(),
	}
}

//ResourceFactory is a function for creating new Resource instances.
type ResourceFactory func() (Resource, error)

//Wizard manages a pool.
type Wizard struct {
	pool            *Pool
	resourceFactory ResourceFactory
	resourceCount   int
}

//Pool returns the pool managed by this wizard.
func (w *Wizard) Pool() *Pool {
	return w.pool
}

//NewWizard creates a new Wizard to manage a pool.
//This default Wizard will create the initial count of resources using the
//ResourceFactory function & will attempt to replace any unhealthy Resources
//it finds with new ones.
func NewWizard(resourceFactory ResourceFactory, resourceCount int) (*Wizard, error) {
	w := &Wizard{
		resourceCount:   resourceCount,
		resourceFactory: resourceFactory,
	}

	w.pool = NewMonitoredPool(MonitorFunc(func(msg ActionMsg) {
		switch msg.Action {
		case UnhealthyResourceTerminated:
			//Create a replacement resource.
			res, err := w.resourceFactory()
			if err != nil {
				//Can only log the error as we are in a goroutine trying to recover.
				log.Printf("Error creating replacement resource: %s", err)
				return
			}
			///and add it to the pool
			err = w.pool.AddResource(res)
			if err != nil {
				//Can only log the error as we are in a goroutine trying to recover.
				log.Printf("Error adding replacement resource to pool: %s", err)
				return
			}
		}
	}))

	for i := 0; i < resourceCount; i++ {
		res, err := w.resourceFactory()
		if err != nil {
			return nil, err
		}
		err = w.pool.AddResource(res)
		if err != nil {
			return nil, err
		}
	}
	return w, nil
}
