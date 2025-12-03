package frogpond

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/tchap/go-patricia/patricia"
)

type DataPoint struct {
	Key     []byte
	Value   []byte
	Name    string
	Updated time.Time
	Deleted bool
}

// DataPoint represents a single key/value entry.
//
// JSON encoding of []byte fields (Key, Value) uses base64 per Go's encoding/json rules.
// Helper methods on Node are provided for common string use cases.

// DeepCopy creates a copy of the DataPoint with Key and Value buffers duplicated
func (dp DataPoint) DeepCopy() DataPoint {
	var kCopy, vCopy []byte
	if dp.Key != nil {
		kCopy = make([]byte, len(dp.Key))
		copy(kCopy, dp.Key)
	}
	if dp.Value != nil {
		vCopy = make([]byte, len(dp.Value))
		copy(vCopy, dp.Value)
	}
	return DataPoint{
		Key:     kCopy,
		Value:   vCopy,
		Name:    dp.Name,
		Updated: dp.Updated,
		Deleted: dp.Deleted,
	}
}

type DataPoolList []DataPoint

func (a DataPoolList) Len() int           { return len(a) }
func (a DataPoolList) Less(i, j int) bool { return bytes.Compare(a[i].Key, a[j].Key) < 0 }
func (a DataPoolList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

type DataPoolMap struct {
	mu   sync.RWMutex
	trie *patricia.Trie
}

func NewDataPoolMap() *DataPoolMap {
	return &DataPoolMap{trie: patricia.NewTrie()}
}

func (dpm *DataPoolMap) Get(key string) (DataPoint, bool) {
	dpm.mu.RLock()
	defer dpm.mu.RUnlock()
	item := dpm.trie.Get(patricia.Prefix(key))
	if item == nil {
		return DataPoint{}, false
	}
	v, ok := item.(DataPoint)
	if !ok {
		panic("invalid data entered into data pool")
	}
	return v.DeepCopy(), true
}

func (dpm *DataPoolMap) Set(key string, value DataPoint) {
	dpm.mu.Lock()
	defer dpm.mu.Unlock()
	dpm.trie.Set(patricia.Prefix(key), value.DeepCopy())
}

func (dpm *DataPoolMap) Delete(key string) {
	dpm.mu.Lock()
	defer dpm.mu.Unlock()
	dpm.trie.Delete(patricia.Prefix(key))
}

func (dpm *DataPoolMap) ForEach(f func(string, DataPoint)) {
	dpm.mu.RLock()
	defer dpm.mu.RUnlock()
	_ = dpm.trie.Visit(func(prefix patricia.Prefix, item patricia.Item) error {
		v, ok := item.(DataPoint)
		if !ok {
			panic("invalid data entered into data pool")
		}
		f(string(prefix), v.DeepCopy())
		return nil
	})
}

func (dpm *DataPoolMap) ToList() DataPoolList {
	dpm.mu.RLock()
	defer dpm.mu.RUnlock()
	out := DataPoolList{}
	_ = dpm.trie.Visit(func(_ patricia.Prefix, item patricia.Item) error {
		v, ok := item.(DataPoint)
		if !ok {
			panic("invalid data entered into data pool")
		}
		out = append(out, v.DeepCopy())

		return nil
	})
	sort.Sort(out)
	return out
}

func (dpm *DataPoolMap) FromList(dl DataPoolList) {
	dpm.mu.Lock()
	defer dpm.mu.Unlock()
	for _, v := range dl {
		dpm.trie.Set(patricia.Prefix(v.Key), v.DeepCopy())
	}
}

func (dpm *DataPoolMap) ForEachPrefix(prefix string, f func(string, DataPoint)) {
	dpm.mu.RLock()
	defer dpm.mu.RUnlock()
	_ = dpm.trie.VisitSubtree(patricia.Prefix(prefix), func(p patricia.Prefix, item patricia.Item) error {
		v := item.(DataPoint)
		f(string(p), v.DeepCopy())
		return nil
	})
}

type Node struct {
	DataPool       *DataPoolMap
	Debug          bool
	EnablePullData bool // If false, disables active pulling of data from peers (saves memory/processing)
}

// Create a new frogpond node
func NewNode() *Node {
	return &Node{DataPool: NewDataPoolMap()}
}

// Convert a map of data points to a list of data points
func DataMap2DataList(dm *DataPoolMap) DataPoolList {
	return dm.ToList()
}

// Convert a list of data points to a map of data points
func DataList2DataMap(dl DataPoolList) *DataPoolMap {
	dpm := NewDataPoolMap()
	dpm.FromList(dl)
	return dpm
}

// The data comes in as a list, so apply each update in turn
func (n *Node) applyUpdate(dl DataPoolList) DataPoolList {
	delta := DataPoolList{}
	for _, v := range dl {
		old, ok := n.DataPool.Get(string(v.Key))
		if !ok {
			n.DataPool.Set(string(v.Key), v)
			delta = append(delta, v)
			n.debugf("%v does not exist, adding", string(v.Key))
		} else {
			patchTime := v.Updated.Unix()
			datumTime := old.Updated.Unix()
			timeDiff := patchTime - datumTime
			if timeDiff > 0 {
				n.DataPool.Set(string(v.Key), v)
				delta = append(delta, v)
				n.debugf("%v changed (candidate copy %v is newer than current copy  %v) diff: %v", string(v.Key), patchTime, datumTime, timeDiff)
			} else {
				n.debugf("%v NOT changed (candidate copy %v is older than current copy  %v) diff: %v", string(v.Key), patchTime, datumTime, timeDiff)
			}
		}
	}
	return delta
}

func (n *Node) debugf(f string, v ...interface{}) {
	if n.Debug {
		log.Printf(f, v...)
	}
}

// Dump the entire data pool as a json array
func (n *Node) JsonDump() []byte {
	out, err := json.Marshal(n.DataPool.ToList())
	if err != nil {
		log.Println("Failed to marshal data pool", err)
		return nil
	}
	// Return a copy to avoid exposing internal buffer
	copied := make([]byte, len(out))
	copy(copied, out)
	return copied
}

// Set a single data point
func (n *Node) SetDataPoint(key string, val []byte) []DataPoint {

	return n.AppendDataPoint(DataPoint{Key: []byte(key), Value: val, Updated: time.Now()})
}

// Set a single data point with a prefix, e.g. "/foo/" and "bar" becomes "/foo/bar"
func (n *Node) SetDataPointWithPrefix(prefix, key string, val []byte) []DataPoint {
	keyStr := fmt.Sprintf("%v%v", prefix, key)
	return n.SetDataPoint(keyStr, val)
}

// Set a single data point with a prefix, e.g. "/foo/" and "bar" becomes "/foo/bar"
func (n *Node) SetDataPointWithPrefix_str(prefix, key string, val string) []DataPoint {
	keyStr := fmt.Sprintf("%v%v", prefix, key)
	return n.SetDataPoint(keyStr, []byte(val))
}

// Set a single data point with a prefix, e.g. "/foo/" and "bar" becomes "/foo/bar"
func (n *Node) SetDataPointWithPrefix_iface(prefix, key string, val interface{}) []DataPoint {
	keyStr := fmt.Sprintf("%v%v", prefix, key)
	valStr := fmt.Sprintf("%v", val)
	return n.SetDataPoint(keyStr, []byte(valStr))
}

// Get a single data point
func (n *Node) GetDataPoint(keyStr string) DataPoint {
	dp, _ := n.DataPool.Get(keyStr)
	return dp
}

// Get a single data point with a prefix, e.g. "/foo/" and "bar" becomes "/foo/bar"
func (n *Node) GetDataPointWithPrefix(prefix, key string) DataPoint {
	keyStr := fmt.Sprintf("%v%v", prefix, key)
	return n.GetDataPoint(keyStr)
}

// Delete a data point, and backdate it by timeDuration.  Backdating allows for a grace period,
// to allow replication to propogate a recent update.
//
//	e.g. It is naturally hard to detect when a node drops out of the network.  So the nodes constantly
//
// try to delete each other, backdated by 30 minutes.  If a node has published an update in that 30
// minutes, the delete will be ignored.  This is a natural way to handle partitions.  The partitioned
// nodes keep updating, and when the network rejoins, the deletes will not remove valid nodes
func (n *Node) DeleteDataPoint(keyStr string, backdateDuration time.Duration) []DataPoint {
	dp := DataPoint{Key: []byte(keyStr), Deleted: true, Updated: time.Now().Add(-backdateDuration)}
	n.DataPool.Set(keyStr, dp)
	return []DataPoint{dp}
}

func (n *Node) DeleteDataPointWithPrefix(prefix, key string, backdateDuration time.Duration) []DataPoint {
	keyStr := fmt.Sprintf("%v%v", prefix, key)
	dp := DataPoint{Key: []byte(keyStr), Deleted: true, Updated: time.Now().Add(-backdateDuration)}
	n.DataPool.Set(keyStr, dp)
	return []DataPoint{dp}
}

func (n *Node) DeleteAllMatchingPrefix(keyStr string) []DataPoint {
	out := []DataPoint{}
	n.DataPool.mu.Lock()
	defer n.DataPool.mu.Unlock()
	_ = n.DataPool.trie.VisitSubtree(patricia.Prefix(keyStr), func(p patricia.Prefix, item patricia.Item) error {
		v := item.(DataPoint)
		v.Deleted = true
		v.Updated = time.Now()
		n.DataPool.trie.Set(p, v.DeepCopy())
		out = append(out, v.DeepCopy())
		return nil
	})
	return out
}

func (n *Node) GetAllMatchingPrefix(keyStr string) []DataPoint {
	out := []DataPoint{}
	n.DataPool.ForEachPrefix(keyStr, func(_ string, v DataPoint) {
		out = append(out, v.DeepCopy())
	})
	return out
}

// Add or update a list of data points to the data pool
func (n *Node) AppendDataPoints(dataPoints []DataPoint) []DataPoint {

	return n.applyUpdate(dataPoints)

}

// Add or update a single data point to the data pool
func (n *Node) AppendDataPoint(dataPoint DataPoint) []DataPoint {

	return n.applyUpdate(DataPoolList{dataPoint})

}

type Config struct {
	HttpPort           uint
	StartPagePort      uint
	Name               string
	MaxUploadSize      uint
	Networks           []string
	KnownPeers         []string
	ArpCheckInterval   int
	PeerUpdateInterval int
	Debug              bool
}
