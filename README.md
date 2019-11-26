[![Go Report Card](https://goreportcard.com/badge/github.com/rbrumby/resourcery)](https://goreportcard.com/report/github.com/rbrumby/resourcery)
[![Build Status](https://travis-ci.com/rbrumby/resourcery.svg?branch=master)](https://travis-ci.com/rbrumby/resourcery)
[![Coverage Status](https://coveralls.io/repos/github/rbrumby/resourcery/badge.svg?branch=master)](https://coveralls.io/github/rbrumby/resourcery?branch=master)

# resourcery
A resource manager for managing resources such as connection pools.
Resourcery lets you add any number of resources to a pool (as part of your start-up routine) & then have multiple parallel processes share that pool of resources. If no resources are available (because they are all in use), resourcery will manage blocking until a resource is ready. Timeouts & cancellations are supported (through context.Context).

## Get the code
`go get github.com/rbrumby/resourcery`

## Use a pool (self managed)
### Create a Resource implementation with IsHealthy() and Terminate() methods.
- `IsHealthy() bool` should return the state of the resource. Your resource should run a background goroutine to keep its health state up to date.
- `Terminate()` will be called by resourcery if an unhealthy resource is being removed from the pool or if resourcery is being shut down.

### Create a pool
```
p := resourceery.NewPool()
```

### Add as many resource instances to the pool as you need
Adding a resource to a Pool causes a goroutine to block on sending the resource
to a channel which is read by GetResource (see below).
```
err := p.AddResource(mypkg.NewConnectionResource(...))
...
```

### Get a resource from the pool when needed
Use a `context.Context` to manage timeout / cancellation.
If the pool finds a resource which isn't healthy (`IsHealthy()` returns false), it will terminate that resource (by calling its `Terminate()` method) and will attempt to get another resource from the pool for you. The pool will not try to create a replacement resource - the pool size will be reduced. You either have to implement your own method of adding replacement resources or use a wizard (see below).
```
ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
defer cancel()
res, err := p.GetResource(ctx)
```

### Cast the resource to your local type & use as required
```
myres, ok := res.(*MyResource)
if !ok {
  t.Error("Failed to convert resource to *MyResource")
	return
}
err := myres.Execute(...)
```

### Put the resource back in the pool when you are done with it
```
p.AddResource(res)
```

## Use a wizard to manage the pool for you
A wizard will create your pool & populate it with resources using a ResourceFactory function that you provide.
The wizard will call your ResourceFactory function tot populate the pool with as manay resources as you sepcify in resourceCount so you don't have to. The other advantage a wizard provides is that if the pool finds an unhealthy resource & terminates it, the wizard will attempt to create a replacement (using the same ResourceFactory function) & add it to the pool.
```
w, err := NewWizard(
  ResourceFactory(func() (Resource, error) {
    res = &MyResource{healthy: true}
    return res, nil
  }),
  1, //number of resources to add to the pool
)
if err != nil {
  t.Error(err)
  return
}
```
From there you can get resources, use them & return them to the pool just as with using a pool directly. You can access the pool ,using the wizards Pool() method.
