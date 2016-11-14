package graph

import (
	"errors"
	"io"
)

const (
	metadataKeyName    = "n"
	metadataKeyKeyInfo = "k"
)

var (
	// ErrNotFound informs that given entry does not exist
	ErrNotFound = errors.New("Entry not found")

	// ErrIncompatibleNode is returned if node of incompatible type or from
	// different entrypoint instance is being added to the current one
	ErrIncompatibleNode = errors.New("Given node is not compatible with this EntryPoint")
)

// Node is an abstract common interface representing all node types in blob
// graph.
type Node interface {
	// ReadOnly() bool

	clone() Node
}

// DirEntry represents one entry in a directory structure
type DirEntry struct {

	// Node is a destination node this entry points to
	Node Node

	// Metadata contains all user-defined metadata entries.
	Metadata map[string]string
}

// DirEntryMap represents map of entries inside directory
type DirEntryMap map[string]DirEntry

// DirNode represents a directory node which does gather other entries
// TODO: Pagination of entries based on cursors
type DirNode interface {
	Node

	// Child looks for one child of given name in this directory
	// If given entry does not exist, ErrNotFound is returned
	Child(name string) (DirEntry, error)

	// List returns a list of all entries in the directory.
	List() (entries DirEntryMap, err error)

	// AttachChild does attach already existing node to given entry,
	// if entry already exists, it will be overwritten by the new one,
	// if entry does not exist yet, it will be created.
	// Entry being attached must be from the same graph structure.
	// This attachement does attach current node state but does not
	// automatically propagate changes made on the original node.
	// Also if you'd like to alter attached object, the returned DirEntry
	// should be used instead of the one passed as argument to this function.
	//
	// TODO: Add note about link nodes once imlemented to create automatic
	//       contents update.
	AttachChild(name string, entry DirEntry) (DirEntry, error)

	// DetachChild removes given child from this directory.
	// Note that this removal does not mean physical removal of node that was
	// attached earlier. It just breaks the link between this directory and
	// child entry.
	DetachChild(name string) error
}

// FileNode represents just a blob of data
type FileNode interface {
	Node

	// Open opens the contents of this file node for reading. If there's no error,
	// the caller must close returned ReadCloser instance, Close must be called
	// exactly once.
	Open() (io.ReadCloser, error)

	// Save tries to save data on given file node. Data will be read
	// from given reader until either EOF ending successfull save or any other
	// error which will cancel the save - in such case this error will be
	// returned from this function
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
