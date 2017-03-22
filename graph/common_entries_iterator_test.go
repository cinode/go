package graph

import (
	"errors"
	"testing"
)

func TestArrayEntriesIteratorEmpty(t *testing.T) {
	ei := newArrayEntriesIterator(nil, nil, nil)
	mustPanic(t, func() {
		// Panic before first Next()
		ei.GetEntry()
	})
	if ei.Next() {
		// No entries at all
		t.Fatal("Next succeedd for empty array iteration")
	}
	mustPanic(t, func() {
		// Panic after Next() returned false
		ei.GetEntry()
	})
	mustPanic(t, func() {
		// Must not call Next() once it returned false
		ei.Next()
	})
}

func TestArrayEntriesIteratorFewElements(t *testing.T) {
	ei := newArrayEntriesIterator(
		[]Node{&dummyNode{}, &dummyNode{}, &dummyNode{}},
		[]string{"a", "b", "c"},
		[]MetadataMap{MetadataMap{}, MetadataMap{}, MetadataMap{}},
	)
	mustPanic(t, func() {
		ei.GetEntry()
	})
	for i := 0; i < 3; i++ {
		if !ei.Next() {
			t.Fatal("Next did not succeed")
		}
		// Must not panic nor error
		_, _, _, err := ei.GetEntry()
		errCheck(t, err, nil)
	}
	if ei.Next() {
		// No entries at all
		t.Fatal("Next did not return false after all entries read")
	}
	mustPanic(t, func() {
		ei.GetEntry()
	})
	mustPanic(t, func() {
		ei.Next()
	})
}

func TestArrayEntriesIteratorCancelBeforeIeration(t *testing.T) {
	nodes := []Node{&dummyNode{}, &dummyNode{}, &dummyNode{}}
	names := []string{"a", "b", "c"}
	metas := []MetadataMap{MetadataMap{}, MetadataMap{}, MetadataMap{}}

	for _, ei := range []EntriesIterator{
		newArrayEntriesIterator(nil, nil, nil),
		newArrayEntriesIterator(nodes, names, metas),
	} {
		// Cancel before iteration starts
		ei.Cancel()

		if !ei.Next() {
			// This is required to be able to get the cancellation error
			t.Fatalf("Next() must always succeed when iteration is cancelled")
		}

		_, _, _, err := ei.GetEntry()
		errCheck(t, err, ErrIterationCancelled)
	}
}

func TestArrayEntriesIteratorCancelDuringIteration(t *testing.T) {
	nodes := []Node{&dummyNode{}, &dummyNode{}, &dummyNode{}}
	names := []string{"a", "b", "c"}
	metas := []MetadataMap{MetadataMap{}, MetadataMap{}, MetadataMap{}}

	ei := newArrayEntriesIterator(nodes, names, metas)

	if !ei.Next() {
		t.Fatal("Couldn't iterate")
	}

	_, _, _, err := ei.GetEntry()
	errCheck(t, err, nil)

	ei.Cancel()
	if !ei.Next() {
		// This is required to be able to get the cancellation error
		t.Fatalf("Next() must always succeed when iteration is cancelled")
	}

	_, _, _, err = ei.GetEntry()
	errCheck(t, err, ErrIterationCancelled)
}

func TestArrayEntriesIteratorCancelAfterIteration(t *testing.T) {
	nodes := []Node{&dummyNode{}, &dummyNode{}, &dummyNode{}}
	names := []string{"a", "b", "c"}
	metas := []MetadataMap{MetadataMap{}, MetadataMap{}, MetadataMap{}}

	ei := newArrayEntriesIterator(nodes, names, metas)

	for i := 0; i < 3; i++ {
		if !ei.Next() {
			t.Fatal("Couldn't iterate")
		}

		_, _, _, err := ei.GetEntry()
		errCheck(t, err, nil)
	}

	ei.Cancel()
	if !ei.Next() {
		// This is required to be able to get the cancellation error
		t.Fatalf("Next() must always succeed when iteration is cancelled")
	}

	_, _, _, err := ei.GetEntry()
	errCheck(t, err, ErrIterationCancelled)
}

func TestErrorEntriesIterator(t *testing.T) {
	errReturned := errors.New("Test Error")
	ei := newErrorEntriesIterator(errReturned)

	if !ei.Next() {
		t.Fatal("Can't get error if Next() returns false")
	}
	_, _, _, err := ei.GetEntry()
	errCheck(t, err, errReturned)

	// Cancel does not change anything, we'll still return the error
	ei.Cancel()

	_, _, _, err = ei.GetEntry()
	errCheck(t, err, errReturned)
}
