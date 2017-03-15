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

func allGrP(f func(newEp func() EntryPoint)) {

	f(func() EntryPoint {
		return InMemory()
	})

	f(func() EntryPoint {
		return blencTest()
	})

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
		// dumpEP(ep)
		// dump(dir1, "dir1", "")
		// dump(dir2, "dir2", "")
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

func TestHasNode(t *testing.T) {
	allGr(func(ep EntryPoint) {
		mkDir(t, ep, []string{"a", "b"})
		mkDir(t, ep, []string{"a", "c"})

		d := mkDir(t, ep, []string{"a"})

		for _, e := range []struct {
			name   string
			exists bool
		}{
			{"a", false},
			{"b", true},
			{"c", true},
			{"d", false},
		} {

			ok, err := d.HasEntry(e.name)
			errCheck(t, err, nil)
			if e.exists {
				if ok == false {
					t.Fatal("HasEntry returned false for existing entry")
				}
			} else {
				if ok == true {

					t.Fatal("HasEntry returned true for non-existing entry")
				}
			}
		}
	})
}

func TestListChildrenCancel(t *testing.T) {
	allGr(func(ep EntryPoint) {
		dir := ensureIsDir(t, ep, []string{})

		i := dir.ListEntries()
		i.Cancel()
		if !i.Next() {
			t.Fatal("After iteration has been cancelled, Next must succeed")
		}
		node, name, err := i.GetEntry()
		errCheck(t, err, ErrIterationCancelled)
		if node != nil || name != "" {
			t.Fatal("Node or name returned in case of iteration error")
		}

		saveFile(t, ep, dir, "entry", []byte{}, nil)
		i = dir.ListEntries()
		if !i.Next() {
			t.Fatal("Interation error")
		}
		node, name, err = i.GetEntry()
		if name != "entry" {
			t.Fatal("Invalid entry from the iteration")
		}
		if node == nil {
			t.Fatal("Node must not be null")
		}
		errCheck(t, err, nil)

		i.Cancel()
		node, name, err = i.GetEntry()
		errCheck(t, err, ErrIterationCancelled)
		if node != nil || name != "" {
			t.Fatal("Node or name returned in case of iteration error")
		}

		if !i.Next() {
			t.Fatal("After iteration has been cancelled, Next must succeed")
		}
		node, name, err = i.GetEntry()
		errCheck(t, err, ErrIterationCancelled)
		if node != nil || name != "" {
			t.Fatal("Node or name returned in case of iteration error")
		}

		// A small test for multithreaded interface
		for j := 0; j < 100; j++ {
			i = dir.ListEntries()
			if !i.Next() {
				t.Fatal("Interation error")
			}

			done := make(chan bool)
			sync := make(chan bool)
			go func() {
				for {
					select {
					case <-done:
						// Last chance test, GetEntry must return error
						_, _, err = i.GetEntry()
						errCheck(t, err, ErrIterationCancelled)
						sync <- true
						return

					default:
						_, _, err := i.GetEntry()
						if err == ErrIterationCancelled {
							<-done
							sync <- true
							return
						}
					}
				}
			}()

			go func() {
				i.Cancel()
				done <- true
			}()

			<-sync
			close(done)
			close(sync)
		}

	})
}

func TestSaveOverwrite(t *testing.T) {

	contents1 := []byte("test1")
	contents2 := []byte("test2")

	allGr(func(ep EntryPoint) {
		dir := mkDir(t, ep, []string{"a", "b", "c"})
		fl := saveFile(t, ep, dir, "d", contents1, nil)
		ensureIsFile(t, ep, []string{"a", "b", "c", "d"}, contents1, nil)
		errCheck(t, fl.Save(bReader(contents2, nil, nil, nil)), nil)
		ensureIsFile(t, ep, []string{"a", "b", "c", "d"}, contents2, nil)
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

func TestEmptyFileNode(t *testing.T) {
	allGr(func(ep EntryPoint) {
		f, err := ep.NewDetachedFileNode()
		errCheck(t, err, nil)
		r, err := f.Open()
		errCheck(t, err, nil)
		b, err := ioutil.ReadAll(r)
		errCheck(t, err, nil)
		errCheck(t, r.Close(), nil)

		if len(b) != 0 {
			t.Fatal("New file node contains non-empty data")
		}
	})
}

func TestEmptyDirNode(t *testing.T) {
	allGr(func(ep EntryPoint) {
		d, err := ep.NewDetachedDirNode()
		errCheck(t, err, nil)
		list, err := listAllEntries(t, d)
		errCheck(t, err, nil)
		if len(list) != 0 {
			t.Fatal("New dir node is not empty")
		}
	})
}

func TestDetachedComplexStructure(t *testing.T) {
	allGr(func(ep EntryPoint) {
		d, err := ep.NewDetachedDirNode()
		errCheck(t, err, nil)

		dirs := []struct {
			path []string
		}{
			{[]string{"b", "c", "d"}},
			{[]string{"b", "e", "f"}},
			{[]string{"b", "e", "g"}},
		}

		for _, e := range dirs {
			mkSubDir(t, ep, d, e.path)
		}

		d2 := mkSubDir(t, ep, d, dirs[0].path)
		for _, b := range blobs {
			saveFile(t, ep, d2, b.name, b.data, nil)
		}

		// ---------------------
		for _, e := range dirs {
			ensureIsSubDir(t, ep, d, e.path)
		}
		for _, b := range blobs {
			ensureIsSubFile(t, ep, d, []string{"b", "c", "d", b.name}, b.data, nil)
		}

		// ---------------------

		// Attach new structure to root
		d3 := mkDir(t, ep, []string{"x"})
		_, err = d3.SetEntry("y", d)
		errCheck(t, err, nil)

		for _, e := range dirs {
			ensureIsDir(t, ep, append([]string{"x", "y"}, e.path...))
		}
		for _, b := range blobs {
			ensureIsFile(t, ep, []string{"x", "y", "b", "c", "d", b.name}, b.data, nil)
		}
	})
}
