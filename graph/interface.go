package graph

import (
	"errors"
	"io"
)

var (
	// ErrEntryNotFound informs that given entry does not exist
	ErrEntryNotFound = errors.New("Entry not found")

	// ErrInvalidEntryName is used when entry name is empty or longer than
	// MaxEntryNameLength bytes
	ErrInvalidEntryName = errors.New("Invalid entry name")

	// ErrIncompatibleNode is returned if node of incompatible node type or
	// node from different entrypoint instance is being added to the current
	// graph
	ErrIncompatibleNode = errors.New("Given node is not compatible with this EntryPoint")

	// ErrMetadataKeyNotFound is used when queried metadata key does not exist
	ErrMetadataKeyNotFound = errors.New("Metadata key does not exist")

	// ErrInvalidMetadataKey is used to indicate that given metadata key is
	// empty or longer than MaxMetadataKeyLength bytes (in utf-8)
	ErrInvalidMetadataKey = errors.New("Invalid metadata key")

	// ErrInvalidMetadataValue is used when given metadata value is longer
	// than MaxMetadataValueLength bytes (in utf-8)
	ErrInvalidMetadataValue = errors.New("Invalid metadata value")

	// ErrTooManyMetadataKeys is used when operation has been cancelled because
	// it would increase the number of metadata keys in one node to a value
	// greater than MaxMetadataKeysInNode
	ErrTooManyMetadataKeys = errors.New("Too many metadata keys in a node")

	// ErrIterationCancelled is used to indicate that iteration of directory
	// entries has been cancelled
	ErrIterationCancelled = errors.New("Entries iteration has been cancelled")
)

const (
	// MaxEntryNameLength is the maximum length in bytes (utf-8) of a single
	// entry name
	MaxEntryNameLength = 1024
	// MaxMetadataKeyLength is the maximum length in bytes (utf-8) of metadata
	// key
	MaxMetadataKeyLength = 128
	// MaxMetadataValueLength is the maximum length in bytes (utf-8) of metadata
	// value
	MaxMetadataValueLength = 1024
	// MaxMetadataKeysInNode is the maximum number of metadata keys for one node
	MaxMetadataKeysInNode = 128
)

// Node is an abstract common interface representing all node types in blob
// graph. A Node may be detached (not attached to any attachment points) or
// attached to exactly one parent node. If a node is reattached, the result will
// be a clone of the node attached to this other attachment point. Such clone
// operation must be very cheap. A cloned node may be totally independent from
// the original one if it's a static one (that includes node's children)
type Node interface {
	clone() Node

	// TODO: Following functions would be interesting to have here:
	// IsReadOnly() bool
	// IsDynamic() bool

	// Parent returns parent node this one is attached to. If this node is
	// detached, nil is returned
	//GetParent() Node
}

// EntriesIterator does return an iterator that will list dir entries
// Iterating over directory list entries should be done as follows:
//
//   for i := dir.List(...); i.Next(); {
//	   node, name, err := i.GetEntry()
//	   if err != nil {
//       // Handle iteration error
//     }
//     ...
//   }
//
type EntriesIterator interface {

	// Advance to next element, this must be called for first element too
	Next() bool

	// Return current entry
	GetEntry() (Node, string, error)

	// Cancels iteration, if other thread/goroutine is currently waiting
	// for the Next() call it must end immediately with true, next call to
	// GetEntry() must return an error ErrIterationCancelled
	Cancel()
}

// DirNode represents a directory node which does gather other entries
type DirNode interface {
	Node

	// TODO: Add metatada operations

	// GetEntry looks for one child entry of given name in this directory
	// If given entry does not exist, ErrNotFound is returned
	GetEntry(name string) (Node, error)

	// HasEntry returns true if given entry exists, false otherwise
	HasEntry(name string) (bool, error)

	// SetEntry creates new or updates existing entry, the node given will be
	// cloned (according to node's clone strategy), the clone is returned back
	// from this function
	SetEntry(name string, node Node) (Node, error)

	// DeleteEntry removes given entry if found, ErrEntryNotFound is returned if
	// entry does not exist
	DeleteEntry(name string) error

	// Get entries iterator
	ListEntries() EntriesIterator
}

// FileNode represents just a blob of data
type FileNode interface {
	Node

	// Open opens the contents of this file node for reading. If there's no
	// error, the caller must close returned ReadCloser instance, Close must be
	// called exactly once on the returned ReadCloser instance, even in case of
	// an error during read.
	Open() (io.ReadCloser, error)

	// Save tries to save data on given file node. Data will be read
	// from given reader until either EOF ending successfull save or any other
	// error which will cancel the save - in such case this error will be
	// returned from this function. In case of a successfull save, parent
	// directory structure (if this node is attached to one) will be updated
	// to reflect changes made to this node. This change does not affect any
	// clones previously created from this node.
	Save(io.ReadCloser) error
}

// EntryPoint represents a 'gate' through which user does see the graph of nodes.
// It does identify the root node and knows underlying storage. New nodes must
// be created through this interface and must not be mixed with other instances
// of EntryPoint.
type EntryPoint interface {

	// Root returns the root node
	Root() (DirNode, error)

	// NewDirNode creates new empy directory node. Created instance must be used
	// only inside this EntryPoint instance
	NewDetachedDirNode() (DirNode, error)

	// NewFileNode creates new empty file node. Created instance myst be used
	// only inside this EntryPoint instance.
	NewDetachedFileNode() (FileNode, error)
}