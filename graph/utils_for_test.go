package graph

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/cinode/go/blenc"
	"github.com/cinode/go/datastore"
)

type helperReader struct {
	buf     io.Reader
	onRead  func() error
	onEOF   func() error
	onClose func() error
}

func bReader(b []byte, onRead func() error, onEOF func() error, onClose func() error) *helperReader {

	nop := func() error {
		return nil
	}

	if onRead == nil {
		onRead = nop
	}
	if onEOF == nil {
		onEOF = nop
	}
	if onClose == nil {
		onClose = nop
	}

	return &helperReader{
		buf:     bytes.NewReader(b),
		onRead:  onRead,
		onEOF:   onEOF,
		onClose: onClose,
	}
}

func (h *helperReader) Read(b []byte) (n int, err error) {
	err = h.onRead()
	if err != nil {
		return 0, err
	}

	n, err = h.buf.Read(b)
	if err == io.EOF {
		err = h.onEOF()
		if err != nil {
			return 0, err
		}
		return 0, io.EOF
	}

	return n, err
}

func (h *helperReader) Close() error {
	return h.onClose()
}

func saveFile(t *testing.T, ep EntryPoint, dir DirNode, name string, b []byte, meta map[string]string) FileNode {
	fn, err := ep.NewDetachedFileNode()
	errCheck(t, err, nil)
	err = fn.Save(bReader(b, nil, nil, nil))
	errCheck(t, err, nil)
	if dir == nil {
		return fn
	}
	node, err := dir.SetEntry(name, fn)
	errCheck(t, err, nil)
	if node, ok := node.(FileNode); ok {
		return node
	}
	t.Fatalf("Didn't get FileNode after attaching")
	return nil
}

func errCheck(t *testing.T, err error, expected error) {

	if err == expected {
		return
	}

	if expected == nil {
		t.Fatalf("Unexpected error: %v", err)
	} else {
		t.Fatalf("Expected error: %v, got %v", expected, err)
	}
}

func mkDir(t *testing.T, ep EntryPoint, path []string) DirNode {
	dir, err := ep.Root()
	errCheck(t, err, nil)

	pathStr := ""

	for _, p := range path {
		pathStr = pathStr + p + "/"
		node, err := dir.GetEntry(p)
		if err == ErrEntryNotFound {
			newDir, err2 := ep.NewDetachedDirNode()
			errCheck(t, err2, nil)
			node, err = dir.SetEntry(p, newDir)
		}
		errCheck(t, err, nil)
		if d, ok := node.(DirNode); !ok {
			t.Fatalf("Not a directory: %v", pathStr)
		} else {
			dir = d
		}
	}
	return dir
}

func dump(n Node, name, ind string) {
	switch n := n.(type) {
	case DirNode:
		fmt.Println(ind + name + ":")
		for i := n.ListEntries(); i.Next(); {
			node, n, _ := i.GetEntry()
			dump(node, n, ind+"| ")
		}
	case FileNode:
		fmt.Println(ind + name)
	default:
		fmt.Println(ind + "<unknown>")
	}
}

func dumpEP(ep EntryPoint) {
	root, _ := ep.Root()
	dump(root, "/", "")
}

func ensureIsDir(t *testing.T, ep EntryPoint, path []string) DirNode {
	dir, err := ep.Root()
	errCheck(t, err, nil)

	pathStr := ""
	for _, p := range path {
		pathStr = pathStr + p + "/"
		node, err := dir.GetEntry(p)
		if err == ErrEntryNotFound {
			t.Fatalf("IsDir: %s does not exist", pathStr)
		}
		errCheck(t, err, nil)
		if d, ok := node.(DirNode); !ok {
			t.Fatalf("IsDir Not a directory: %v", pathStr)
		} else {
			dir = d
		}
	}

	return dir
}

/*
func ensureMetadata(t *testing.T, de DirEntry, path []string, metaCheck map[string]string) {

	if len(metaCheck) != len(de.Metadata) {
		t.Fatalf("IsFile: %s has invalid metadata (count does not match)",
			strings.Join(path, "/"))
	}
	for k, v := range metaCheck {
		if vc, ok := de.Metadata[k]; !ok || v != vc {
			t.Fatalf("IsFile: %s has invalid metadata (value does not exist or does not match)",
				strings.Join(path, "/"))
		}
	}
}
*/

func ensureIsFile(t *testing.T, ep EntryPoint, path []string,
	contentsCheck []byte, metaCheck map[string]string) FileNode {

	dir := ensureIsDir(t, ep, path[:len(path)-1])
	node, err := dir.GetEntry(path[len(path)-1])
	if err == ErrEntryNotFound {
		dumpEP(ep)
		t.Fatalf("IsFile: %s does not exist", strings.Join(path, "/"))
	}
	/*
		if metaCheck != nil {
			ensureMetadata(t, de, path, metaCheck)
			errCheck(t, err, nil)
		}
	*/
	if f, ok := node.(FileNode); ok {
		if contentsCheck != nil {
			r, err := f.Open()
			errCheck(t, err, nil)
			defer r.Close()
			b, err := ioutil.ReadAll(r)
			errCheck(t, err, nil)

			if !bytes.Equal(b, contentsCheck) {
				t.Fatalf("Incorrect contents of file %s", strings.Join(path, "/"))
			}
		}
		return f
	}

	t.Fatalf("IsFile Not a file: %v", strings.Join(path, "/"))
	return nil
}

func listAllEntries(t *testing.T, d DirNode) (map[string]Node, error) {
	ret := make(map[string]Node)
	for it := d.ListEntries(); it.Next(); {
		node, name, err := it.GetEntry()
		if err != nil {
			return nil, nil
		}
		if _, found := ret[name]; found {
			t.Fatalf("Duplicate dir entry name found: %s", name)
		}
		ret[name] = node
	}
	return ret, nil
}

type memBEPersistance struct {
	bid string
	key string
}

func (p *memBEPersistance) Get() (bid, key string, err error) {
	return p.bid, p.key, nil
}

func (p *memBEPersistance) Set(bid, key string) error {
	p.bid = bid
	p.key = key
	return nil
}

func blencTest() *blencEP {
	ret, err := FromBE(
		blenc.FromDatastore(
			datastore.InMemory()),
		&memBEPersistance{},
	)
	if err != nil {
		panic("Can't create datastore-based EP")
	}
	return ret.(*blencEP)
}
