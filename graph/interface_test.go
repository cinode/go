package graph

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"testing"
)

/*
type memBEPersistance struct {
	bid string
	key string
}

func (p *memBEPersistance) Get() (bid, key string, err error) {
	if p.bid == "" {
		return "", "", ErrBERootTabulaRasa
	}
	return p.bid, p.key, nil
}

func (p *memBEPersistance) Set(bid, key string) error {
	p.bid = bid
	p.key = key
	return nil
}
*/

func allGrP(f func(newEp func() EntryPoint)) {

	f(func() EntryPoint {
		return InMemory()
	})
	/*
		f(func() EntryPoint {
			ret, err := FromBE(
				blenc.FromDatastore(
					datastore.InMemory()),
				&memBEPersistance{},
			)
			if err != nil {
				panic("Can't create datastore-based EP")
			}
			return ret
		})
	*/
}

func allGr(f func(ep EntryPoint)) {
	allGrP(func(newEp func() EntryPoint) {
		f(newEp())
	})
}

func TestInterfaceNewDirNode(t *testing.T) {
	allGr(func(ep EntryPoint) {
		dir, err := ep.NewDetachedDirNode()
		errCheck(t, err, nil)
		if dir == nil {
			t.Fatalf("NewDirNode must return dir object if no error")
		}
	})
}

func TestInterfaceNewFileNode(t *testing.T) {
	allGr(func(ep EntryPoint) {
		f, err := ep.NewDetachedFileNode()
		errCheck(t, err, nil)
		if f == nil {
			t.Fatalf("NewFileNode must return file object if no error")
		}
	})
}

func TestInterfaceRoot(t *testing.T) {
	allGr(func(ep EntryPoint) {
		r, err := ep.Root()
		errCheck(t, err, nil)
		if r == nil {
			t.Fatalf("Root must return root dir node object if no error")
		}
	})
}

var blobs = []struct {
	name string
	data []byte
}{
	{"empty", []byte{}},
	{"onebyte", []byte{0xFF}},
}

func TestCreateFileOnRoot(t *testing.T) {
	allGr(func(ep EntryPoint) {

		r, err := ep.Root()
		errCheck(t, err, nil)

		// Saving files
		for _, d := range blobs {
			_, err := r.GetEntry(d.name)
			errCheck(t, err, ErrEntryNotFound)
			saveFile(t, ep, r, d.name, d.data, nil)
		}

		// Reading back contents
		for _, d := range blobs {
			node, err := r.GetEntry(d.name)
			errCheck(t, err, nil)
			f, ok := node.(FileNode)
			if !ok {
				t.Fatalf("Node is not a file")
			}
			r, err := f.Open()
			errCheck(t, err, nil)
			b, err := ioutil.ReadAll(r)
			r.Close()
			errCheck(t, err, nil)

			if !bytes.Equal(b, d.data) {
				t.Fatal("Incorrect data read back")
			}
		}

	})
}

type dummyNode struct {
}

func (d *dummyNode) clone() Node {
	panic("Should not be here")
}

func TestIncompatibleNode(t *testing.T) {
	allGrP(func(newEp func() EntryPoint) {
		ep1 := newEp()
		ep2 := newEp()

		d1, err := ep1.Root()
		errCheck(t, err, nil)

		f2, err := ep2.NewDetachedFileNode()
		errCheck(t, err, nil)

		_, err = d1.SetEntry("test", f2)
		errCheck(t, err, ErrIncompatibleNode)

		_, err = d1.SetEntry("test", &dummyNode{})
		errCheck(t, err, ErrIncompatibleNode)

		_, err = d1.SetEntry("test", nil)
		errCheck(t, err, ErrIncompatibleNode)
	})
}

func TestDeleteNode(t *testing.T) {
	allGr(func(ep EntryPoint) {
		r, err := ep.Root()
		errCheck(t, err, nil)

		for _, d := range blobs {
			saveFile(t, ep, r, d.name, d.data, nil)
		}

		for _, d := range blobs {
			_, err := r.GetEntry(d.name)
			errCheck(t, err, nil)
			err = r.DeleteEntry(d.name)
			errCheck(t, err, nil)
			_, err = r.GetEntry(d.name)
			errCheck(t, err, ErrEntryNotFound)
			err = r.DeleteEntry(d.name)
			errCheck(t, err, ErrEntryNotFound)
		}

	})
}

func TestSubDir(t *testing.T) {
	allGr(func(ep EntryPoint) {
		r, err := ep.Root()
		errCheck(t, err, nil)

		d, err := ep.NewDetachedDirNode()
		errCheck(t, err, nil)

		_, err = r.SetEntry("d", d)
		errCheck(t, err, nil)

		node, err := r.GetEntry("d")
		errCheck(t, err, nil)

		if _, ok := node.(DirNode); !ok {
			t.Fatalf("Did not get dir node")
		}
	})
}

/*
func TestModifyEntriesMap(t *testing.T) {

	meta := map[string]string{
		"meta1key": "meta1value",
		"meta2key": "meta2value",
		"meta3key": "meta3value",
	}

	allGr(func(ep EntryPoint) {
		d, err := ep.Root()
		errCheck(t, err, nil)
		saveFile(t, ep, d, "file", []byte("file"), meta)

		// Metadata entries in Child()-returned value must not propagate
		entry, err := d.GetEntry("file")
		errCheck(t, err, nil)
		entry.Metadata["meta4key"] = "meta4value"
		ensureIsFile(t, ep, []string{"file"}, nil, meta)

		// Metadata entries in List()-returned value must not propagate
		ls, err := d.List()
		errCheck(t, err, nil)
		ls["file"].Metadata["meta5key"] = "meta5value"
		ensureIsFile(t, ep, []string{"file"}, nil, meta)

		// Changing map returned from List() must not propagate
		ls, err = d.List()
		errCheck(t, err, nil)
		ls["file2"] = ls["file"]
		_, err = d.GetEntry("file2")
		errCheck(t, err, ErrNotFound)
	})
}
*/

func TestAttachSubtree(t *testing.T) {
	allGr(func(ep EntryPoint) {

		dir1 := mkDir(t, ep, []string{"a", "b", "c"})
		dir2 := mkDir(t, ep, []string{"d", "e", "f"})

		ensureIsDir(t, ep, []string{"a", "b", "c"})
		ensureIsDir(t, ep, []string{"d", "e", "f"})

		contents1 := []byte("file1data")
		contents2 := []byte("changed file contents")

		attrs1 := map[string]string{"meta1": "value1", "meta2": "value2"}
		attrs2 := map[string]string{"meta3": "value3", "meta4": "value4"}

		// Create a/b/c/file1
		saveFile(t, ep, dir1, "file1", contents1, attrs1)
		ensureIsFile(t, ep, []string{"a", "b", "c", "file1"}, contents1, attrs1)

		// Clone dir1 contents (a/b/c) into dir2 (d/e/f) using name g
		// this should create d/e/f/g and d/e/f/g/file1
		dir2.SetEntry("g", dir1)
		ensureIsFile(t, ep, []string{"d", "e", "f", "g", "file1"}, contents1, attrs1)

		// Change original file, ensure the cloned one did not change
		saveFile(t, ep, dir1, "file1", contents2, attrs2)
		fl := ensureIsFile(t, ep, []string{"d", "e", "f", "g", "file1"}, contents1, attrs1)
		ensureIsFile(t, ep, []string{"a", "b", "c", "file1"}, contents2, attrs2)

		// Clone file only, this must not propagate attributes
		dir2.SetEntry("file1", fl)
		ensureIsFile(t, ep, []string{"d", "e", "f", "file1"}, contents1, map[string]string{})

		// Delete original file, ensure the clone is still there
		errCheck(t, dir1.DeleteEntry("file1"), nil)
		ensureIsFile(t, ep, []string{"d", "e", "f", "g", "file1"}, contents1, attrs1)
	})
}

func TestListChildren(t *testing.T) {

	allGr(func(ep EntryPoint) {
		testList := func(path []string, entries []string) {
			dir := ensureIsDir(t, ep, path)
			list, err := listAllEntries(t, dir)
			errCheck(t, err, nil)
			if len(entries) != len(list) {
				t.Fatalf("Incorrect number of entries in '%s', expeced %d, got %d",
					strings.Join(path, "/"), len(entries), len(list))
			}
			for _, name := range entries {
				if _, ok := list[name]; !ok {
					t.Fatalf("Missing entry: %s", name)
				}
			}
		}

		mkDir(t, ep, []string{"a", "b", "c"})
		mkDir(t, ep, []string{"a", "b", "d"})
		mkDir(t, ep, []string{"a", "e", "f"})

		testList([]string{"a"}, []string{"b", "e"})
		testList([]string{"a", "b"}, []string{"c", "d"})
		testList([]string{"a", "e"}, []string{"f"})
	})

}

func TestSaveError(t *testing.T) {

	contents1 := []byte("test1")
	contents2 := []byte("test2")

	allGr(func(ep EntryPoint) {
		root, err := ep.Root()
		errCheck(t, err, nil)
		fl := saveFile(t, ep, root, "a", contents1, nil)
		errToRet := errors.New("err")
		err = fl.Save(bReader(contents2, func() error {
			return errToRet
		}, nil, nil))
		if err == nil {
			t.Fatal("Expected error did not happen")
		}
		ensureIsFile(t, ep, []string{"a"}, contents1, nil)
		err = fl.Save(bReader(contents2, nil, func() error {
			return errToRet
		}, nil))
		if err == nil {
			t.Fatal("Expected error did not happen")
		}
		ensureIsFile(t, ep, []string{"a"}, contents1, nil)
		err = fl.Save(bReader(contents2, nil, nil, func() error {
			return errToRet
		}))
		if err == nil {
			t.Fatal("Expected error did not happen")
		}
		ensureIsFile(t, ep, []string{"a"}, contents1, nil)
	})
}

func TestSaveConcurrent(t *testing.T) {
	threadCnt := 10
	saveCnt := 200

	allGr(func(ep EntryPoint) {
		wg := sync.WaitGroup{}
		wg.Add(threadCnt)

		for i := 0; i < threadCnt; i++ {
			go func(i int) {
				defer wg.Done()
				for n := 0; n < saveCnt; n++ {
					saveFile(t, ep, mkDir(t, ep, []string{}), "file", []byte(
						fmt.Sprintf("contents-%d-%d", i, n)), nil)
				}
			}(i)
		}

		wg.Wait()
	})
}
