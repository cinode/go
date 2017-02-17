package graph

import (
	"errors"
	"sync/atomic"

	"github.com/cinode/go/blenc"
)

var (
	// ErrMalformedDirectoryBlob is returned if the directory blob can not be
	// read
	ErrMalformedDirectoryBlob = errors.New("Malformed data in directory blob")
)

const (
	dirTypeBasicDir  = 1
	dirTypeBasicFile = 2

	beKeyInfoTypeValue = 1
)

// BERootPersistence is an interface describing how to read and save
// root directory blob and it's encryption key
type BERootPersistence interface {

	// Get returns currently saved bid and it's key, error in case there's
	// an error during the save. If ErrBERootTabulaRasa is returned as an error,
	// new, empty one will be created automatically. Any other error will cause
	// EntryPoint initialization failure.
	Get() (bid, key string, err error)

	// Set stores currently saved bid and it's key.
	Set(bid, key string) error
}

// FromBE creates graph's entrypoint from blob encryption layer implementation
func FromBE(be blenc.BE, p BERootPersistence) (EntryPoint, error) {

	bid, key, err := p.Get()
	if err != nil {
		return nil, err
	}

	// TODO: More fine-grained key generator ?
	ep := &epBE{
		be: be,
		p:  p,
		kg: blenc.ContentsHashKey(),
	}

	root := beDirNodeNew(ep)
	root.isRoot = true
	root.path = "@r"
	root.bid, root.key = bid, key

	ep.root = root
	return ep, nil
}

type epBE struct {
	be    blenc.BE
	p     BERootPersistence
	kg    blenc.KeyDataGenerator
	epoch int64
	root  *beDirNode
}

func (ep *epBE) generateEpoch() int64 {
	return atomic.AddInt64(&ep.epoch, 1)
}

func (ep *epBE) Root() (DirNode, error) {
	return ep.root, nil
}

// NewDirNode creates new empy directory node. Created instance must be used
// only inside this EntryPoint
func (ep *epBE) NewDetachedDirNode() (DirNode, error) {
	ret := beDirNodeNew(ep)
	ret.state = beDirNodeStateIdle
	ret.path = "@d"
	return ret, nil
}

// NewFileNode creates new empty file node. Created instance myst be used
// only inside this EntryPoint instance.
func (ep *epBE) NewDetachedFileNode() (FileNode, error) {
	ret := beFileNodeNew(ep)
	ret.path = "@d"
	return ret, nil
}

func (ep *epBE) sync() error {
	f, err := ep.root.rlockSync()
	f()
	return err
}

func beNewNode(t uint64, ep *epBE) Node {
	switch t {
	case dirTypeBasicDir:
		return beDirNodeNew(ep)

	case dirTypeBasicFile:
		return beFileNodeNew(ep)

	default:
		return nil
	}
}

func beNodeType(n Node) uint64 {
	switch n.(type) {
	case *beDirNode:
		return dirTypeBasicDir

	case *beFileNode:
		return dirTypeBasicFile
	}
	panic("Unknown node type")
}
