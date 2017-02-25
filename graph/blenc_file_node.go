package graph

import (
	"bytes"
	"io"
	"io/ioutil"
)

type beFileNode struct {
	beNodeBase
}

func (b *beFileNode) Open() (io.ReadCloser, error) {
	defer b.rlock()()

	if b.isEmpty() {
		// Special case to get rid of
		return ioutil.NopCloser(bytes.NewReader([]byte{})), nil
	}

	return b.rawReader()
}

func (b *beFileNode) Save(r io.ReadCloser) error {

	// Save and get new bid/key for the new contents
	bid, key, err := b.ep.be.Save(r, b.ep.kg)
	if err != nil {
		return err
	}

	defer b.wlock()()
	if b.parent != nil {
		// If there's a parent, generate new epoch and propagate through
		// parent chain. For this node itself, it's not important, file's
		// changes are immediate without a state when there's some unsaved
		// contents
		epoch := b.ep.generateEpoch()
		if err = b.parent.propagateUnsavedEpoch(epoch, b, beEpochSetEmpty); err != nil {
			return err
		}
	}

	// Immediately notify that out contents changed, the remaining set of
	// unsaved epochs is always empty for file nodes
	return b.blobUpdated(b, bid, key, beEpochSetEmpty)
}

func (b *beFileNode) clone() (Node, error) {
	defer b.rlock()()
	ret := beFileNodeNew(b.ep)
	ret.bid, ret.key = b.bid, b.key
	return ret, nil
}

func beFileNodeNew(ep *epBE) *beFileNode {
	return &beFileNode{
		beNodeBase{
			ep: ep,
		},
	}
}
