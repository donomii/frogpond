package frogpond

import (
	"sync"
	"testing"
	"time"
)

func TestDataPoolMap_BasicSetGet(t *testing.T) {
	dpm := NewDataPoolMap()
	dp := DataPoint{Key: []byte("foo"), Value: []byte("bar"), Updated: time.Now()}
	dpm.Set("foo", dp)
	got, ok := dpm.Get("foo")
	if !ok || string(got.Value) != "bar" {
		t.Errorf("expected to get value 'bar', got %v, ok=%v", got.Value, ok)
	}
}

func TestDataPoolMap_Delete(t *testing.T) {
	dpm := NewDataPoolMap()
	dp := DataPoint{Key: []byte("foo"), Value: []byte("bar"), Updated: time.Now()}
	dpm.Set("foo", dp)
	dpm.Delete("foo")
	_, ok := dpm.Get("foo")
	if ok {
		t.Errorf("expected key to be deleted")
	}
}

func TestDataPoolMap_ConcurrentAccess(t *testing.T) {
	dpm := NewDataPoolMap()
	wg := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + i%26))
			dpm.Set(key, DataPoint{Key: []byte(key), Value: []byte("val"), Updated: time.Now()})
			_, _ = dpm.Get(key)
			dpm.Delete(key)
		}(i)
	}
	wg.Wait()
}

func TestDataPoolMap_ToListAndFromList(t *testing.T) {
	dpm := NewDataPoolMap()
	dpm.Set("a", DataPoint{Key: []byte("a"), Value: []byte("1"), Updated: time.Now()})
	dpm.Set("b", DataPoint{Key: []byte("b"), Value: []byte("2"), Updated: time.Now()})
	list := dpm.ToList()
	if len(list) != 2 {
		t.Errorf("expected list length 2, got %d", len(list))
	}
	dpm2 := NewDataPoolMap()
	dpm2.FromList(list)
	if _, ok := dpm2.Get("a"); !ok {
		t.Errorf("expected key 'a' in dpm2")
	}
}

func TestNode_SetGetDeleteDataPoint(t *testing.T) {
	n := NewNode()
	key := "foo"
	val := []byte("bar")
	n.SetDataPoint(key, val)
	dp := n.GetDataPoint(key)
	if string(dp.Value) != "bar" {
		t.Errorf("expected value 'bar', got %v", dp.Value)
	}
	n.DeleteDataPoint(key, 0)
	dp = n.GetDataPoint(key)
	if !dp.Deleted {
		t.Errorf("expected data point to be marked deleted")
	}
}

func TestNode_ApplyUpdate(t *testing.T) {
	n := NewNode()
	dp1 := DataPoint{Key: []byte("foo"), Value: []byte("bar"), Updated: time.Now()}
	dp2 := DataPoint{Key: []byte("foo"), Value: []byte("baz"), Updated: time.Now().Add(time.Second)}
	delta := n.AppendDataPoints([]DataPoint{dp1})
	if len(delta) != 1 {
		t.Errorf("expected 1 update, got %d", len(delta))
	}
	delta = n.AppendDataPoints([]DataPoint{dp2})
	if len(delta) != 1 || string(delta[0].Value) != "baz" {
		t.Errorf("expected update to 'baz', got %v", delta)
	}
}
