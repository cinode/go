package graph

import (
	"io"
	"sync"

	log "github.com/Sirupsen/logrus"
)

// beNodeBase is a base class representing a node stored in blenc layer.
//
// Since this data structure must be highly concurrent and protected against
// simultaneous use, it's important to clearify locking schemes here.
// Iportant fact: a node update could simultaneously happen from multiple
// independent threads, for example imagine two http clients uploading equally
// named file. Spurious decision about which one is genuine is simply
// resolved by selecting the one which has finished the upload later.
//
// Also keep in mind that even though the whole file has been uploaded, this
// ends up being a new blob in blenc layer. Changing the entry is finalized by
// propagating the change of blob name and encryption info up through directory
// node chain until the root directory is found (at that point, blob name and
// encryption key is stored in current persistence object).
//
// The blob name and the key is not a property of the node itself. Instead, it's
// hold by directory parent node or the persistence object. Because of that,
// updating blob name and encryption info is done inside parent node and while
// holding lock of the parent.
//
type beNodeBase struct {
	mutex  sync.RWMutex // Local mutex
	ep     *epBE        // Entry point object, can not change after initialization
	isRoot bool         // True if this is the root node for this entrypoint
	parent *beDirNode   // Parent dir node
	bid    string       // currently known bid of blob hosting node's data
	key    string       // currently known key for the blob hosting node's data`
	path   string       // Path of the node (for debug purposes)
}

func (n *beNodeBase) String() string {
	return n.path
}

// toBase is a helper function to quickly convert to beNodeBase instance
func (n *beNodeBase) toBase() *beNodeBase {
	return n
}

// toBase fetches beNodeBase object from the interface given, nil is returned
// if could not get beNodeBase pointer
func toBase(instance interface{}) *beNodeBase {
	be, _ := instance.(interface {
		toBase() *beNodeBase
	})
	if be == nil {
		return nil
	}
	return be.toBase()
}

// rlock locks node's mutext for read, returns functor that will unlock the
// mutext when called
func (n *beNodeBase) rlock() func() {
	log.Infof("rlock start: %v", n)
	n.mutex.RLock()
	log.Infof("rlock acquired: %v", n)
	return func() {
		log.Infof("runlock: %v", n)
		n.mutex.RUnlock()
	}
}

// wlock locks node's mutext for write, returns functor that will unlock the
// mutext when called
func (n *beNodeBase) wlock() func() {
	log.Infof("wlock start: %v", n)
	n.mutex.Lock()
	log.Infof("wlock acquired: %v", n)
	return func() {
		log.Infof("wunlock: %v", n)
		n.mutex.Unlock()
	}
}

// isEmpty determines if this node is a special case empty node.
// This should be fixed by using well-known empty bids / keys
// Note: requires mutext to be rlocked
func (n *beNodeBase) isEmpty() bool {
	return n.bid == ""
}

// rawReader opens raw reader object for this blob
func (n *beNodeBase) rawReader() (io.ReadCloser, error) {
	//log.Infof("rawReader: %v %s %s", n, n.bid, n.key)
	return n.ep.be.Open(n.bid, n.key)
}

// blobUpdated is called to notify that the blob of this node has been updated
func (n *beNodeBase) blobUpdated(node Node, bid string, key string, pendingUnsavedEpoch *beEpochSet) error {

	log.Infof("blobUpdated: %v %s %s %s", n, bid, key, *pendingUnsavedEpoch)

	// Save bid and key in the blob structure
	n.bid, n.key = bid, key

	if n.isRoot {
		// Root node is persisted to persistence object
		return n.ep.p.Set(bid, key)
	}

	if n.parent != nil {
		// Non-root node does persist itself in parent directory node
		return n.parent.persistChildChange(node, n, pendingUnsavedEpoch)
	}

	// Detached node, nothing else to do
	return nil
}
