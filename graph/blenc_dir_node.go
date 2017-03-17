package graph

import (
	"fmt"
	"sync"
)

//go:generate stringer -type=blencDirNodeState -output blencdirnodestate.generated.go
type blencDirNodeState int

const (
	blencDirNodeStateUnloaded blencDirNodeState = iota
	blencDirNodeStateLoading
	blencDirNodeStateIdle
	blencDirNodeStateSaveRequested
	blencDirNodeStateSaving
	blencDirNodeStateLoadError
)

const (
	blencDirMaxEntries  = 1024 // TODO: Take out, allow split directories
	blencMaxBlobNameLen = 128
	blencMaxKeyLen      = 1024
)

type blencDirNodeEntry struct {
	bid             string
	key             string
	node            Node
	unsavedEpochSet blencEpochSet
	metadata        MetadataMap
}

type blencNodeToNameMap map[Node]string
type blencEntriesMap map[string]*blencDirNodeEntry

type blencDirNode struct {
	blencNodeBase

	nodeToName blencNodeToNameMap
	entries    blencEntriesMap

	loadFinishedCondition *sync.Cond
	state                 blencDirNodeState

	// unsavedEpochSet contains all unsaved epoch set for this node and
	// all it's children, including those changes which are currently
	// saved
	unsavedEpochSet blencEpochSet

	// unsavedPendingEpochSet contains set of unsaved epochs which are
	// not currently being saved
	unsavedPendingEpochSet blencEpochSet

	// unsavedEpochSetReduced will broadcast whenever there's a chance
	// that unsaved epoch set has been reduced
	unsavedEpochSetReduced *sync.Cond
}

// rlockLoad prepares node's data for read. Once returned, we can be sure the
// data is either correctly loaded or an error happened. Returned functor is
// an unlock routine that must always be called to unlock the node (even if
// error is returned)
func (d *blencDirNode) rlockLoad() (func(), error) {
	for {
		unlock := d.rlock()
		switch d.state {
		case
			blencDirNodeStateIdle,
			blencDirNodeStateSaveRequested,
			blencDirNodeStateSaving:
			// We're already loaded (could be dirty, but that's not that
			// important now), all done, can return
			return unlock, nil

		case blencDirNodeStateLoadError:
			// There was an error while loading blob, return the error to
			// prevent further corruption of data
			return unlock, ErrMalformedDirectoryBlob

		case
			blencDirNodeStateUnloaded,
			blencDirNodeStateLoading:

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
func (d *blencDirNode) wlockLoad() (func(), error) {
	unlock := d.wlock()
	for {
		switch d.state {
		case
			blencDirNodeStateIdle,
			blencDirNodeStateSaveRequested,
			blencDirNodeStateSaving:
			// We're already loaded (could be dirty, but that's not that
			// important now), all done, can return
			return unlock, nil

		case blencDirNodeStateLoadError:
			// There was an error while loading blob, return the error to
			// prevent further corruption of data
			return unlock, ErrMalformedDirectoryBlob

		case blencDirNodeStateUnloaded:
			// The data has not been loaded yet, we're the first thread to
			// notice that and hold write lock, let's start loading here.
			// We'll analyze loading result in next loop iteration.
			d.load()

		case blencDirNodeStateLoading:
			// The data is being loaded in another thread now, let's wait for it
			// to finish. We'll analyze loading result in next loop iteration.
			d.loadFinishedCondition.Wait()

		default:
			panic(fmt.Sprintf("Invalid state: %v", d.state))
		}
	}
}

// rlockSync does sync the state of current dir node - this means all changes
// that were not saved at the time of entering the sync function must be
// persisted in the existing bid/key
func (d *blencDirNode) rlockSync() (func(), error) {
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
	epochsToClear := d.unsavedEpochSet

	for d.unsavedEpochSet.overlaps(epochsToClear) {
		d.unsavedEpochSetReduced.Wait()
	}

	return unlock, err
}

// load tries to read the data from associated blob
// Requires: wlock
func (d *blencDirNode) load() {
	d.state = blencDirNodeStateLoading

	entries := make(blencEntriesMap)
	nodeToName := make(blencNodeToNameMap)

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

		entries, err = blencDirBlobFormatDeserialize(rc, d.ep)
		if err != nil {
			return err
		}

		for name, entry := range entries {
			// Fill in missing data
			base := toBlencNodeBase(entry.node)
			base.parent = d
			base.ep = d.ep
			base.path = d.path + "/" + name

			// Build reverse map
			nodeToName[entry.node] = name
		}

		return nil
	}()

	if err != nil {
		d.state = blencDirNodeStateLoadError
	} else {
		d.entries = entries
		d.nodeToName = nodeToName
		d.state = blencDirNodeStateIdle
	}
	d.loadFinishedCondition.Broadcast()

}

func (d *blencDirNode) GetEntry(name string) (Node, error) {
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

func (d *blencDirNode) GetEntryMetadataValue(name string, metaName string) (string, error) {
	f, err := d.rlockLoad()
	defer f()
	if err != nil {
		return "", err
	}

	entry, ok := d.entries[name]
	if !ok {
		return "", ErrEntryNotFound
	}

	value, ok := entry.metadata[metaName]
	if !ok {
		return "", ErrMetadataKeyNotFound
	}

	return value, nil
}

func (d *blencDirNode) GetEntryMetadataMap(name string) (MetadataMap, error) {
	f, err := d.rlockLoad()
	defer f()
	if err != nil {
		return nil, err
	}

	entry, ok := d.entries[name]
	if !ok {
		return nil, ErrEntryNotFound
	}

	return entry.metadata.clone(), nil
}

func (d *blencDirNode) HasEntry(name string) (bool, error) {
	f, err := d.rlockLoad()
	defer f()
	if err != nil {
		return false, err
	}

	_, ok := d.entries[name]
	return ok, nil
}

func (d *blencDirNode) SetEntry(name string, node Node, metaChange *MetadataChange) (Node, error) {

	clone, old, err := func() (Node, *blencNodeBase, error) {

		base := toBlencNodeBase(node)
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

		// Test first if metadata updates are ok, don't do any change
		// if there will be an error
		oldMeta := (MetadataMap)(nil)
		if d.entries[name] != nil {
			oldMeta = d.entries[name].metadata
		}
		newMetadataMap, err := metaChange.apply(oldMeta)
		if err != nil {
			return nil, nil, err
		}

		epoch := d.ep.generateEpoch()
		d.unsavedEpochSet.add(epoch)
		d.unsavedPendingEpochSet.add(epoch)

		if err = d.parent.propagateUnsavedEpoch(epoch, d, d.unsavedEpochSet); err != nil {
			return nil, nil, err
		}

		// Note: Don't have to lock clone to change it, we're the only owner now
		cloneBase := toBlencNodeBase(clone)
		cloneBase.parent = d
		cloneBase.path = d.path + "/" + name

		var oldChild *blencNodeBase
		entry := d.entries[name]
		if entry != nil {
			oldChild = toBlencNodeBase(entry.node)
		} else {
			entry = &blencDirNodeEntry{}
			d.entries[name] = entry
		}

		entry.node = clone
		entry.bid = cloneBase.bid
		entry.key = cloneBase.key
		entry.unsavedEpochSet = blencEpochSetEmpty
		entry.metadata = newMetadataMap

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

func (d *blencDirNode) DeleteEntry(name string) error {
	node, err := func() (*blencNodeBase, error) {
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
		d.unsavedEpochSet.add(epoch)
		d.unsavedPendingEpochSet.add(epoch)

		if err = d.parent.propagateUnsavedEpoch(epoch, d, d.unsavedEpochSet); err != nil {
			return nil, err
		}

		delete(d.nodeToName, entry.node)
		delete(d.entries, name)

		d.scheduleUpdate()
		return toBlencNodeBase(entry.node), nil
	}()

	if node != nil {
		// Updating the child node must be done outside of parent's lock, we're
		// always locking in child -> parent order to prevent deadlocks
		defer node.wlock()()
		node.parent = nil
	}
	return err
}

func (d *blencDirNode) ListEntries() EntriesIterator {
	f, err := d.rlockLoad()
	defer f()
	if err != nil {
		return newErrorEntriesIterator(err)
	}

	nodes := make([]Node, len(d.entries))
	names := make([]string, len(d.entries))
	metadata := make([]MetadataMap, len(d.entries))
	i := 0
	for name, node := range d.entries {
		nodes[i] = node.node
		names[i] = name
		metadata[i] = node.metadata.clone()
		i++
	}

	return newArrayEntriesIterator(nodes, names, metadata)
}

func (d *blencDirNode) clone() (Node, error) {
	// sync and acquire read lock, sync is needed since we need to know the
	// correct up-to-date (relative to the time sync was requested) bid and key
	f, err := d.rlockSync()
	defer f()
	if err != nil {
		return nil, err
	}

	ret := blencDirNodeNew(d.ep)
	ret.bid, ret.key = d.bid, d.key
	// The clone ends up in state beDirNodeStateUnloaded so it'll have to be
	// loaded on the first data access. Maybe we could optimize this
	// and clone entries if those are loaded now?
	// Also if there was load error, we could end up in same error state to
	// prevent reparse of broken data.
	return ret, nil
}

func blencDirNodeNew(ep *blencEP) *blencDirNode {
	ret := &blencDirNode{
		blencNodeBase:          blencNodeBase{ep: ep},
		state:                  blencDirNodeStateUnloaded,
		unsavedEpochSet:        blencEpochSetEmpty,
		unsavedPendingEpochSet: blencEpochSetEmpty,
	}
	ret.loadFinishedCondition = sync.NewCond(&ret.mutex)
	ret.unsavedEpochSetReduced = sync.NewCond(ret.mutex.RLocker())
	return ret
}

func (d *blencDirNode) persistChildChange(n Node, b *blencNodeBase, unsavedEpochSet blencEpochSet) error {
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
	entry.unsavedEpochSet = unsavedEpochSet

	// this function may only be called as a result of child
	// bid/key update. This means that child's unsavedEpochSet
	// could only reduce or remain the same, it can not extend.
	// This means we should already know about this changes
	if !d.unsavedEpochSet.contains(unsavedEpochSet) ||
		!d.unsavedPendingEpochSet.contains(unsavedEpochSet) {
		panic("Consistency error, epoch set assumptions were wrong")
	}

	d.scheduleUpdate()
	return nil
}

// scheduleUpdate ensures current directory contents will be persisted.
// This function must consider scenarios when the update is currently
// in progress, can also delay update process to gather more changes at once.
// Note: requires wlock to be held
func (d *blencDirNode) scheduleUpdate() {

	switch d.state {
	case blencDirNodeStateIdle:
		// Nothing happening with the blob now, start saving immediately
		// TODO: Shouldn't we add some small delay here to increase the
		//       probability of gathering more changes at the same time?
		d.state = blencDirNodeStateSaveRequested
		go d.save()

	case blencDirNodeStateSaving:
		// Save is in progress now, change the state so that if it ends,
		// another update will be executed. Pending changes will be saved
		// once the current save proces ends
		d.state = blencDirNodeStateSaveRequested

	case blencDirNodeStateSaveRequested:
		// Ok, already waiting for the update

	default:
		panic(fmt.Sprintf("Invalid state: %v", d.state))
	}
}

func (d *blencDirNode) save() {

	defer d.wlock()()

	if d.state != blencDirNodeStateSaveRequested {
		panic(fmt.Sprintf("Invalid state: %v", d.state))
	}

	// Inform that we're saving and there are no other pending changes to save
	d.state = blencDirNodeStateSaving

	// Recalculate pending epoch set - we're starting new save and we'll gather
	// all changes we know so far. The only stuff currently left is what's
	// still unsaved in children
	d.unsavedPendingEpochSet = blencEpochSetEmpty
	for _, e := range d.entries {
		d.unsavedPendingEpochSet.addSet(e.unsavedEpochSet)
	}
	// Check invariant
	if !d.unsavedEpochSet.contains(d.unsavedPendingEpochSet) {
		panic("beDirNode::unsavedEpochSet invariant failure")
	}

	// Prepare blob data writer
	rdr, err := blencDirBlobFormatSerialize(d.entries)
	if err != nil {
		// TODO: Support this, maybe some retries?
		panic(fmt.Sprintf("Couldn't gerenate dir blob contents: %v", err))
	}

	// Save the blob with mutex unlocked, we already did the reservation by
	// changing the state to beDirNodeStateSaving so nobody else can start
	// saving goroutine to save this node.
	d.mutex.Unlock()
	bid, key, err := d.ep.be.Save(rdr, d.ep.kg)
	d.mutex.Lock()
	if err != nil {
		// TODO: Support this, maybe some retries?
		panic(fmt.Sprintf("Couldn't save dir blob contents: %v", err))
	}

	// Update current unsavedEpochSet - it will be the value of unsavedPendingEpochSet
	// when we started the save + all changes queued for save while we were saving blob
	d.unsavedEpochSet = d.unsavedPendingEpochSet
	defer d.unsavedEpochSetReduced.Broadcast()

	// Notify that there's new persisted blob
	err = d.blobUpdated(d, bid, key, d.unsavedEpochSet)
	if err != nil {
		// TODO: Support this, maybe some retries?
		panic(fmt.Sprintf("Couldn't save dir blob contents: %v", err))
	}

	switch d.state {
	case blencDirNodeStateSaving:
		// All done, nothing has been scheduled while we were saving
		d.state = blencDirNodeStateIdle

	case blencDirNodeStateSaveRequested:
		// New change added while we were saving, reschedule another save
		go d.save()

	default:
		panic(fmt.Sprintf("Invalid state: %v", d.state))
	}
}

func (d *blencDirNode) propagateUnsavedEpoch(epoch int64, n Node, unsavedEpochSet blencEpochSet) error {
	if d == nil {
		return nil
	}
	f, err := d.wlockLoad()
	defer f()
	if err != nil {
		return err
	}

	name, ok := d.nodeToName[n]
	if !ok {
		// Node during deletion, treat as detached
		return nil
	}

	d.unsavedEpochSet.add(epoch)
	d.unsavedPendingEpochSet.add(epoch)

	d.entries[name].unsavedEpochSet = unsavedEpochSet

	if !d.unsavedPendingEpochSet.contains(unsavedEpochSet) ||
		!d.unsavedEpochSet.contains(unsavedEpochSet) ||
		!d.unsavedEpochSet.contains(d.unsavedPendingEpochSet) {
		panic("Epoch propagation invariant failed")
	}

	return d.parent.propagateUnsavedEpoch(epoch, d, d.unsavedEpochSet)
}
