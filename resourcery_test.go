package resourcery

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestMultiPutGet(t *testing.T) {
	p := NewManagedPool(func(action Action) {
		//Here, we could do something to decide if the pool needs to grow / shrink.
		// log.Printf("Pool updated with action %d", action)
	})
	for i := 0; i < 5; i = i + 1 {
		err := p.AddResource(&testResource{index: i, healthy: true})
		if err != nil {
			t.Error(err)
			return
		}
	}

	done := make(chan struct{}, 20)

	for i := 0; i < 20; i++ {
		go func(i int, t *testing.T) {
			time.Sleep(time.Millisecond * 10)
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*4)
			defer cancel()
			res, err := p.GetResource(ctx)
			if err != nil {
				t.Error(err)
				done <- struct{}{}
				return
			}
			elapsed := time.Since(start)
			rsrc, ok := res.(*testResource)
			if !ok {
				t.Error("Got a non-testResource from the wizard!")
				return
			}
			rsrc.Exec(fmt.Sprintf("Get iteration %d, used recource %d - wait time: %f", i, rsrc.index, elapsed.Seconds()))
			err = p.AddResource(rsrc)
			if err != nil {
				t.Error(err)
				done <- struct{}{}
				return
			}
			done <- struct{}{}
		}(i, t)
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestAddUnhealthy(t *testing.T) {
	p := NewPool()
	err := p.AddResource(&testResource{index: 0, healthy: false})
	if err == nil {
		t.Error("Should have got an error as we shouldn't add unhealthy resources")
		return
	}
}

func TestResourceGoneBadInQueue(t *testing.T) {
	p := NewPool()
	//Add 2 resources
	bad := &testResource{index: 77, healthy: true}
	_ = p.AddResource(bad)
	time.Sleep(time.Millisecond * 10)
	bad2 := &testResource{index: 88, healthy: true}
	_ = p.AddResource(bad2)
	time.Sleep(time.Millisecond * 10)
	//Make the both unhealthy
	bad.healthy = false
	bad2.healthy = false
	//Add a healthy one
	_ = p.AddResource(&testResource{index: 99, healthy: true})
	time.Sleep(time.Millisecond * 10)
	res, err := p.GetResource(context.Background())
	if err != nil {
		t.Error(err)
		return
	}
	tstRes, ok := res.(*testResource)
	if !ok {
		t.Error("Failed to convert to *testResource")
		return
	}
	if tstRes.index != 99 {
		t.Errorf("Expected index 99. Got %d", tstRes.index)
	}
	if p.Size() != 0 {
		t.Errorf("Pool should be empty. Size is %d", p.Size())
	}
}

func TestShutdown(t *testing.T) {
	p := NewPool()
	shutdownCount := 0
	_ = p.AddResource(&testResource{index: 0, healthy: true, shutdownCount: &shutdownCount})
	_ = p.AddResource(&testResource{index: 0, healthy: true, shutdownCount: &shutdownCount})
	_ = p.AddResource(&testResource{index: 0, healthy: true, shutdownCount: &shutdownCount})
	time.Sleep(time.Millisecond * 10)
	p.Shutdown()
	if shutdownCount != 3 {
		t.Errorf("Expected shutdownCount 3. Got %d", shutdownCount)
	}
}

func TestTimeout(t *testing.T) {
	p := NewPool()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*5)
	defer cancel()
	_, err := p.GetResource(ctx)
	if err == nil {
		t.Error("Should have timed out")
		return
	}
	if ctx.Err().Error() != "context deadline exceeded" {
		t.Errorf("Unexpected error: %s", ctx.Err())
		return
	}
}

type testResource struct {
	index         int
	healthy       bool
	shutdownCount *int
}

func (r *testResource) IsHealthy() bool {
	return r.healthy
}

func (r *testResource) Terminate() {
	if r.shutdownCount != nil {
		*(r.shutdownCount)++
	}
}

func (r *testResource) Exec(val string) {
	time.Sleep(time.Millisecond * 10)
	// log.Printf("Resource ran with val: %s\n", val)
}
