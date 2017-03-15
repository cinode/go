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

// BlencRootPersistence is an interface describing how to read and save
// root directory blob and it's encryption key
type BlencRootPersistence interface {

	// Get returns currently saved bid and it's key, error in case there's
	// an error during the save. If ErrBERootTabulaRasa is returned as an error,
	// new, empty one will be created automatically. Any other error will cause
	// EntryPoint initialization failure.
	Get() (bid, key string, err error)

	// Set stores currently saved bid and it's key.
	Set(bid, key string) error
}

// FromBE creates graph's entrypoint from blob encryption layer implementation
func FromBE(be blenc.BE, p BlencRootPersistence) (EntryPoint, error) {

	bid, key, err := p.Get()
	if err != nil {
		return nil, err
	}

	// TODO: More fine-grained key generator ?
	ep := &blencEP{
		be: be,
		p:  p,
		kg: blenc.ContentsHashKey(),
	}

	root := blencDirNodeNew(ep)
	root.isRoot = true
	root.path = "@r"
	root.bid, root.key = bid, key

	ep.root = root
	return ep, nil
}

type blencEP struct {
	be    blenc.BE
	p     BlencRootPersistence
	kg    blenc.KeyDataGenerator
	epoch int64
	root  *blencDirNode
}

func (ep *blencEP) generateEpoch() int64 {
	return atomic.AddInt64(&ep.epoch, 1)
}

func (ep *blencEP) Root() (DirNode, error) {
	return ep.root, nil
}

// NewDirNode creates new empy directory node. Created instance must be used
// only inside this EntryPoint
func (ep *blencEP) NewDetachedDirNode() (DirNode, error) {
	ret := blencDirNodeNew(ep)
	ret.path = "@d"
	// Skip loading phase by setting up empty dir
	ret.state = blencDirNodeStateIdle
	ret.entries = make(blencEntriesMap)
	ret.nodeToName = make(blencNodeToNameMap)
	return ret, nil
}

// NewFileNode creates new empty file node. Created instance myst be used
// only inside this EntryPoint instance.
func (ep *blencEP) NewDetachedFileNode() (FileNode, error) {
	ret := blencFileNodeNew(ep)
	ret.path = "@d"
	return ret, nil
}

func (ep *blencEP) sync() error {
	f, err := ep.root.rlockSync()
	f()
	return err
}

func blencNewNode(t uint64, ep *blencEP) Node {
	switch t {
	case dirTypeBasicDir:
		return blencDirNodeNew(ep)

	case dirTypeBasicFile:
		return blencFileNodeNew(ep)

	default:
		return nil
	}
}

func blencNodeType(n Node) uint64 {
	switch n.(type) {
	case *blencDirNode:
		return dirTypeBasicDir

	case *blencFileNode:
		return dirTypeBasicFile
	}
	panic("Unknown node type")
}
