package graph

import (
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
)

//go:generate stringer -type=beDirNodeState -output blenc_bedirnodestate.generated.go
type beDirNodeState int

const (
	beDirNodeStateUnloaded beDirNodeState = iota
	beDirNodeStateLoading
	beDirNodeStateIdle
	beDirNodeStateSaveRequested
	beDirNodeStateSaving
	beDirNodeStateLoadError
)

const (
	beDirMaxEntries  = 1024 // TODO: Take out, allow split directories
	beMaxBlobNameLen = 128
	beMaxKeyLen      = 1024
)

type beDirNodeEntry struct {
	bid             string
	key             string
	node            Node
	unsavedEpochSet beEpochSet
	// TODO: Metadata
}

type beNodeToNameMap map[Node]string
type beEntriesMap map[string]*beDirNodeEntry

type beDirNode struct {
	beNodeBase

	nodeToName beNodeToNameMap
	entries    beEntriesMap

	loadFinishedCondition *sync.Cond
	state                 beDirNodeState

	// Set of epochs that this node and all it's children did not yet save,
	// this includes changes being saved at the moment
	unsavedGlobalEpochSet beEpochSet

	// Set of epochs that were changes made externally on this dir node (i.e.
	// new entries added or removed), this does not include epoch sets inside
	// children
	unsavedLocalEpochSet beEpochSet

	// Condition that broadcasts whenever the global epoch set in this node
	// changes.
	unsavedGlobalEpochChange *sync.Cond
}

// rlockLoad prepares node's data for read. Once returned, we can be sure the
// data is either correctly loaded or an error happened. Returned functor is
// an unlock routine that must always be called to unlock the node (even if
// error is returned)
func (d *beDirNode) rlockLoad() (func(), error) {
	for {
		unlock := d.rlock()
		switch d.state {
		case beDirNodeStateIdle,
			beDirNodeStateSaveRequested,
			beDirNodeStateSaving:
			// We're already loaded (could be dirty, but that's not that
			// important now), all done, can return
			return unlock, nil

		case beDirNodeStateLoadError:
			// There was an error while loading blob, return the error to
			// prevent further corruption of data
			return unlock, ErrMalformedDirectoryBlob

		case beDirNodeStateUnloaded,
			beDirNodeStateLoading:
			//log.Infof("rlockLoad: %s needs loading, state %s", d, d.state)

			// Data not yet loaded, use wlockLoad to read the data / wait for
			// the load first. Note we have to release the read lock first since
			// wlockLoad() does acquire write lock - this allows the state to be
			// changed in the meantime, wlockLoad() should handle all such cases
			// well
			unlock()
			unlock, err := d.wlockLoad()
			if err != nil {
				return unlock, err
			}
			// Once wlockLoad returns, it's write lock must be released (we want
			// read only to allow other simultaneous reads). Since we have to do
			// unlock followed by lock, the state could change in the meantime.
			// That's why the lock is done in repeated loop - we basically start
			// locking from scratch there. It could end up in restart of the
			// loading routine if the resource has been unloaded at that time
			// (note: resource freeing not yet implemented).
			unlock()

		default:
			panic(fmt.Sprintf("Invalid state: %v", d.state))
		}
	}
}

// wlockLoad prepares node's data for write. Once it's done, we can be sure the
// data is either correctly loaded or an error happened. Returned functor is
// an unlock routine that must always be called to unlock the node (even if
// error is returned)
func (d *beDirNode) wlockLoad() (func(), error) {
	unlock := d.wlock()
	for {
		switch d.state {
		case beDirNodeStateIdle,
			beDirNodeStateSaveRequested,
			beDirNodeStateSaving:
			// We're already loaded (could be dirty, but that's not that
			// important now), all done, can return
			return unlock, nil

		case beDirNodeStateLoadError:
			// There was an error while loading blob, return the error to
			// prevent further corruption of data
			return unlock, ErrMalformedDirectoryBlob

		case beDirNodeStateUnloaded:
			// The data has not been loaded yet, we're the first thread to
			// notice that and hold write lock, let's start loading here.
			// We'll analyze loading result in next loop iteration.
			d.load()

		case beDirNodeStateLoading:
			// The data is being loaded in another thread now, let's wait for it
			// to finish. We'll analyze loading result in next loop iteration.
			log.Infof("lockLoad %v: waiting for load")
			d.loadFinishedCondition.Wait()
			log.Infof("lockLoad %v: waiting finished")

		default:
			panic(fmt.Sprintf("Invalid state: %v", d.state))
		}
	}
}

// rlockSync does sync the state of current dir node - this means all changes
// that were not saved at the time of entering the sync function must be
// persisted in the existing bid/key
func (d *beDirNode) rlockSync() (func(), error) {
	// ensure blob is correctly loaded
	unlock, err := d.rlockLoad()
	if err != nil {
		return unlock, err
	}

	// The node may have some set of pending changes. We want to sync to a time
	// when all those changes are cleared. The simplest way to do this is to
	// wait untill node's min unsaved change epoch is greater that max
	// unsaved change epoch currently held. A corner case is when blob is
	// fully clered - no unsaved pending changes remain, we don't have to do
	// an extra exception for this here since the value of min epoch change
	// should be MaxInt64 which should be greater than any epoch ever
	// (it's very unlikely someone would run this software long enough to
	// get even close to int64 boundary, solar system my die sooner)
	epochsToClear := d.unsavedGlobalEpochSet

	//log.Infof("rlockSync: %v unsavedEpochSet: %v", d, d.unsavedEpochSet)
	for d.unsavedGlobalEpochSet.overlaps(&epochsToClear) {
		log.Infof("rlockSync: %v waiting for sync, "+
			"epochToClear: %v, unsavedGlobalEpochSet: %v",
			d, epochsToClear, d.unsavedGlobalEpochSet)
		d.unsavedGlobalEpochChange.Wait()
		log.Infof("rlockSync: %v got signal", d)
	}

	return unlock, err
}

// load tries to read the data from associated blob
// Requires: wlock
func (d *beDirNode) load() {
	d.state = beDirNodeStateLoading

	entries := make(beEntriesMap)
	nodeToName := make(beNodeToNameMap)

	err := func() error {
		if d.isEmpty() {
			return nil
		}

		// No mutex held during load, we're exclusive here anyway since only one
		// thread can set beDirNodeStateLoading at the same time.
		d.mutex.Unlock()
		defer d.mutex.Lock()

		rc, err := d.rawReader()
		if err != nil {
			return err
		}
		defer rc.Close()

		entries, err = beDirBlobFormatDeserialize(rc, d.ep)
		if err != nil {
			return err
		}

		for name, entry := range entries {
			// Fill in missing data
			base := toBase(entry.node)
			base.parent = d
			base.ep = d.ep
			base.path = d.path + "/" + name

			// Build reverse map
			nodeToName[entry.node] = name
		}

		return nil
	}()

	if err != nil {
		d.state = beDirNodeStateLoadError
	} else {
		d.entries = entries
		d.nodeToName = nodeToName
		d.state = beDirNodeStateIdle
	}
	d.loadFinishedCondition.Broadcast()

}

func (d *beDirNode) GetEntry(name string) (Node, error) {
	f, err := d.rlockLoad()
	defer f()
	if err != nil {
		return nil, err
	}

	ret, ok := d.entries[name]
	if !ok {
		return nil, ErrEntryNotFound
	}

	return ret.node, nil
}

func (d *beDirNode) HasEntry(name string) (bool, error) {
	f, err := d.rlockLoad()
	defer f()
	if err != nil {
		return false, err
	}

	_, ok := d.entries[name]
	return ok, nil
}

func (d *beDirNode) SetEntry(name string, node Node) (Node, error) {

	clone, old, err := func() (Node, *beNodeBase, error) {

		base := toBase(node)
		if base == nil || base.ep != d.ep {
			return nil, nil, ErrIncompatibleNode
		}

		// Note: clone must be done outside directory's lock to preserve
		//  child -> parent locking order
		clone, err := node.clone()
		if err != nil {
			return nil, nil, err
		}
		f, err := d.wlockLoad()
		defer f()
		if err != nil {
			return nil, nil, err
		}
		epoch := d.ep.generateEpoch()
		if err = d.parent.propagateUnsavedEpoch(epoch); err != nil {
			return nil, nil, err
		}
		d.unsavedLocalEpochSet.add(epoch)
		d.unsavedGlobalEpochSet.add(epoch)

		// Note: Don't have to lock clone to change it, we're the only owner now
		cloneBase := toBase(clone)
		cloneBase.parent = d
		cloneBase.path = d.path + "/" + name

		var oldChild *beNodeBase
		entry := d.entries[name]
		if entry != nil {
			oldChild = toBase(entry.node)
		} else {
			entry = &beDirNodeEntry{}
			d.entries[name] = entry
		}

		entry.node = clone
		entry.bid = cloneBase.bid
		entry.key = cloneBase.key
		entry.unsavedEpochSet.clear()

		d.nodeToName[clone] = name

		d.scheduleUpdate()
		return clone, oldChild, nil
	}()

	if old != nil {
		defer old.wlock()()
		old.parent = nil
	}

	return clone, err
}

func (d *beDirNode) DeleteEntry(name string) error {
	node, err := func() (*beNodeBase, error) {
		// This sub-scope is needed to automatically unlock the lock taken below
		f, err := d.wlockLoad()
		defer f()
		if err != nil {
			return nil, err
		}

		entry, found := d.entries[name]
		if !found {
			return nil, ErrEntryNotFound
		}

		epoch := d.ep.generateEpoch()
		if err = d.parent.propagateUnsavedEpoch(epoch); err != nil {
			return nil, err
		}
		d.unsavedLocalEpochSet.add(epoch)

		delete(d.nodeToName, entry.node)
		delete(d.entries, name)

		d.scheduleUpdate()
		return toBase(entry.node), nil
	}()

	if node != nil {
		// Updating the child node must be done outside of parent's lock, we're
		// always locking in child -> parent order to prevent deadlocks
		defer node.wlock()()
		node.parent = nil
	}
	return err
}

func (d *beDirNode) ListEntries() EntriesIterator {
	f, err := d.rlockLoad()
	defer f()
	if err != nil {
		return newErrorEntriesIterator(err)
	}

	nodes := make([]Node, len(d.entries))
	names := make([]string, len(d.entries))
	i := 0
	for name, node := range d.entries {
		nodes[i] = node.node
		names[i] = name
		i++
	}

	return newArrayEntriesIterator(nodes, names)
}

func (d *beDirNode) clone() (Node, error) {
	// sync and acquire read lock, sync is needed since we need to know the
	// correct up-to-date (relative to the time sync was requested) bid and key
	f, err := d.rlockSync()
	defer f()
	if err != nil {
		return nil, err
	}

	ret := beDirNodeNew(d.ep)
	ret.bid, ret.key = d.bid, d.key
	// The clone ends up in state beDirNodeStateUnloaded so it'll have to be
	// loaded on the first data access. Maybe we could optimize this
	// and clone entries if those are loaded now?
	// Also if there was load error, we could end up in same error state to
	// prevent reparse of broken data.
	return ret, nil
}

func beDirNodeNew(ep *epBE) *beDirNode {
	ret := &beDirNode{
		beNodeBase: beNodeBase{
			ep: ep,
		},
		state: beDirNodeStateUnloaded,
		unsavedGlobalEpochSet: beEpochSetEmpty,
		unsavedLocalEpochSet:  beEpochSetEmpty,
	}
	ret.loadFinishedCondition = sync.NewCond(&ret.mutex)
	ret.unsavedGlobalEpochChange = sync.NewCond(ret.mutex.RLocker())
	return ret
}

func (d *beDirNode) persistChildChange(n Node, b *beNodeBase, pendingUnsavedEpoch *beEpochSet) error {
	f, err := d.wlockLoad()
	defer f()
	if err != nil {
		return err
	}

	name, ok := d.nodeToName[n]
	if !ok {
		// node being detached
		return nil
	}

	entry := d.entries[name]
	entry.bid = b.bid
	entry.key = b.key
	entry.unsavedEpochSet = *pendingUnsavedEpoch

	d.scheduleUpdate()
	return nil
}

// scheduleUpdate ensures current directory contents will be persisted.
// This function must consider scenarios when the update is currently
// in progress, can also delay update process to gather more changes at once.
// Note: requires wlock to be held
func (d *beDirNode) scheduleUpdate() {

	switch d.state {
	case beDirNodeStateIdle:
		// Nothing happening with the blob now, start saving immediately
		// TODO: Shouldn't we add some small delay here to increase the
		//       probability of gathering more changes at the same time?
		d.state = beDirNodeStateSaveRequested
		go d.save()

	case beDirNodeStateSaving:
		// Save is in progress now, change the state so that if it ends,
		// another update will be executed. Pending changes will be saved
		// once the current save proces ends
		d.state = beDirNodeStateSaveRequested

	case beDirNodeStateSaveRequested:
		// Ok, already waiting for the update

	default:
		panic(fmt.Sprintf("Invalid state: %v", d.state))
	}
}

func (d *beDirNode) save() {

	defer d.wlock()()
	log.Infof("startUpdate: %v starting update, unsavedEpochSet: %v",
		d, d.unsavedGlobalEpochSet)

	if d.state != beDirNodeStateSaveRequested {
		panic(fmt.Sprintf("Invalid state: %v", d.state))
	}

	// Clear local pending change epoch range, all local changes are being
	// persisted right now, let other local changes be added while we're saving
	d.unsavedLocalEpochSet.clear()

	// Inform that we're saving and there are no other pending changes to save
	d.state = beDirNodeStateSaving

	// Prepare blob data reader
	rdr, err := beDirBlobFormatSerialize(d.entries)
	if err != nil {
		// TODO: Support this, maybe some retries?
		panic(fmt.Sprintf("Couldn't gerenate dir blob contents: %v", err))
	}

	// Save the blob with mutex unlocked, we already did the reservation by
	// changing the state to beDirNodeStateSaving so nobody else can start
	// saving goroutine to save this node.
	//d.mutex.Unlock()
	bid, key, err := d.ep.be.Save(rdr, d.ep.kg)
	//d.mutex.Lock()
	if err != nil {
		// TODO: Support this, maybe some retries?
		panic(fmt.Sprintf("Couldn't save dir blob contents: %v", err))
	}

	// Have to recalculate current blob's global epoch set. To recalculate this,
	// we have to consider all sets from children and all those changes in
	// current dir blob which were scheduled during the update.
	d.unsavedGlobalEpochSet.clear()
	d.unsavedGlobalEpochSet.addSet(&d.unsavedLocalEpochSet)
	for _, de := range d.entries {
		d.unsavedGlobalEpochSet.addSet(&de.unsavedEpochSet)
	}
	defer d.unsavedGlobalEpochChange.Broadcast()

	// Notify that there's new persisted blob
	err = d.blobUpdated(d, bid, key, &d.unsavedGlobalEpochSet)
	if err != nil {
		// TODO: Support this, maybe some retries?
		panic(fmt.Sprintf("Couldn't save dir blob contents: %v", err))
	}

	switch d.state {
	case beDirNodeStateSaving:
		// All done, nothing has been scheduled while we were saving
		d.state = beDirNodeStateIdle

	case beDirNodeStateSaveRequested:
		// New change added while we were saving, reschedule another save
		go d.save()

	default:
		panic(fmt.Sprintf("Invalid state: %v", d.state))
	}
}

func (d *beDirNode) propagateUnsavedEpoch(epoch int64) error {
	if d == nil {
		return nil
	}
	f, err := d.wlockLoad()
	defer f()
	if err != nil {
		return err
	}
	d.unsavedGlobalEpochSet.add(epoch)
	return d.parent.propagateUnsavedEpoch(epoch)
}
