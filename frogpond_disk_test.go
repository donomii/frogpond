package frogpond

import (
	"testing"
	"time"
)

func TestDiskNode_ExtentKV_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	n, err := NewDiskNode(tmpDir, 10*1024*1024)
	if err != nil {
		t.Fatalf("failed to create disk node: %v", err)
	}
	defer n.Close()

	key := "foo"
	val := []byte("bar")
	n.SetDataPoint(key, val)

	dp := n.GetDataPoint(key)
	if string(dp.Value) != "bar" {
		t.Errorf("expected value 'bar', got %v", string(dp.Value))
	}
}

func TestDiskNode_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create node, write data, close
	{
		n, err := NewDiskNode(tmpDir, 10*1024*1024)
		if err != nil {
			t.Fatalf("failed to create disk node: %v", err)
		}
		n.SetDataPoint("p1", []byte("v1"))
		n.Close()
	}

	// 2. Re-open node, verify data exists
	{
		n, err := NewDiskNode(tmpDir, 10*1024*1024)
		if err != nil {
			t.Fatalf("failed to create disk node: %v", err)
		}
		defer n.Close()

		dp := n.GetDataPoint("p1")
		if string(dp.Value) != "v1" {
			t.Errorf("expected persistent value 'v1', got %v", string(dp.Value))
		}
	}
}

func TestDiskNode_Megapool_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	n, err := NewMegapoolNode(tmpDir, 10*1024*1024)
	if err != nil {
		t.Fatalf("failed to create megapool node: %v", err)
	}
	defer n.Close()

	key := "mega"
	val := []byte("pool")
	n.SetDataPoint(key, val)

	dp := n.GetDataPoint(key)
	if string(dp.Value) != "pool" {
		t.Errorf("expected value 'pool', got %v", string(dp.Value))
	}
}

func TestDiskNode_UpdateResolution(t *testing.T) {
	tmpDir := t.TempDir()
	n, err := NewDiskNode(tmpDir, 10*1024*1024)
	if err != nil {
		t.Fatalf("failed to create disk node: %v", err)
	}
	defer n.Close()

	// 1. Set initial value
	oldTime := time.Now().Add(-1 * time.Hour)
	dpOld := DataPoint{Key: []byte("k"), Value: []byte("old"), Updated: oldTime}
	n.AppendDataPoint(dpOld)

	// 2. Overwrite with newer value
	newTime := time.Now()
	dpNew := DataPoint{Key: []byte("k"), Value: []byte("new"), Updated: newTime}
	n.AppendDataPoint(dpNew)

	// 3. Verify new value wins
	got := n.GetDataPoint("k")
	if string(got.Value) != "new" {
		t.Errorf("expected 'new' to win, got %v", string(got.Value))
	}

	// 4. Try to write OLDER value again (should be rejected)
	n.AppendDataPoint(dpOld)
	got = n.GetDataPoint("k")
	if string(got.Value) != "new" {
		t.Errorf("expected 'new' to persist, got %v", string(got.Value))
	}
}
