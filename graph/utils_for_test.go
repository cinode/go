package graph

import (
	"bytes"
	"io"
	"io/ioutil"
	"strings"
	"testing"
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
	d, err := dir.AttachChild(name, DirEntry{
		Node:     fn,
		Metadata: meta,
	})
	errCheck(t, err, nil)
	if node, ok := d.Node.(FileNode); ok {
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
		de, err := dir.Child(p)
		if err == ErrNotFound {
			newDir, err2 := ep.NewDetachedDirNode()
			errCheck(t, err2, nil)
			de, err = dir.AttachChild(p, DirEntry{Node: newDir})
		}
		errCheck(t, err, nil)
		if d, ok := de.Node.(DirNode); !ok {
			t.Fatalf("Not a directory: %v", pathStr)
		} else {
			dir = d
		}
	}
	return dir
}

func dump(n Node, name, ind string) string {
	switch n := n.(type) {
	case DirNode:
		ret := ind + name + ":\n"
		list, _ := n.List()
		for name, entry := range list {
			ret += dump(entry.Node, name, ind+"  ")
		}
		return ret
	case FileNode:
		return ind + name + "\n"
	default:
		return ind + "<unknown>\n"
	}
}

func ensureIsDir(t *testing.T, ep EntryPoint, path []string) DirNode {
	dir, err := ep.Root()
	errCheck(t, err, nil)

	pathStr := ""
	for _, p := range path {
		pathStr = pathStr + p + "/"
		de, err := dir.Child(p)
		if err == ErrNotFound {
			t.Fatalf("IsDir: %s does not exist", pathStr)
		}
		errCheck(t, err, nil)
		if d, ok := de.Node.(DirNode); !ok {
			t.Fatalf("IsDir Not a directory: %v", pathStr)
		} else {
			dir = d
		}
	}

	return dir
}

func ensureIsFile(t *testing.T, ep EntryPoint, path []string,
	contentsCheck []byte, metaCheck map[string]string) FileNode {

	dir := ensureIsDir(t, ep, path[:len(path)-1])
	de, err := dir.Child(path[len(path)-1])
	if err == ErrNotFound {
		t.Fatalf("IsFile: %s does not exist", strings.Join(path, "/"))
	}
	if metaCheck != nil {
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
	errCheck(t, err, nil)
	if f, ok := de.Node.(FileNode); ok {
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
