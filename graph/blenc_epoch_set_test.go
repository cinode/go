package graph

import "testing"

func TestBeEpochSetClear(t *testing.T) {
	set := beEpochSet{}
	set.clear()
	if !set.isEmpty() {
		t.Fatal("beEpochSet is not empty after reset")
	}
}

func TestBeEpochSetSingleEpoch(t *testing.T) {
	set := beEpochSet{}
	set.clear()

	set.add(7)
	if set.getMinEpoch() != 7 {
		t.Fatal("Invalid min epoch set")
	}
	if set.getMaxEpoch() != 7 {
		t.Fatal("Invalid max epoch set")
	}
	if !set.hasEpoch(7) {
		t.Fatal("Valid epoch not contained")
	}
	if set.hasEpoch(6) {
		t.Fatal("Invalid epoch contained")
	}
	if set.hasEpoch(8) {
		t.Fatal("Invalid epoch contained")
	}
}

func TestBeEpochSetMultipleEpochs(t *testing.T) {
	epochs := []int64{7, 11, 17}

	set := beEpochSet{}
	set.clear()

	for _, e := range epochs {
		set.add(e)
	}

	if set.getMinEpoch() != epochs[0] {
		t.Fatal("Invalid min epoch set")
	}
	if set.getMaxEpoch() != epochs[len(epochs)-1] {
		t.Fatal("Invalid max epoch set")
	}
	for _, e := range epochs {
		if !set.hasEpoch(e) {
			t.Fatal("Valid epoch not contained")
		}
	}
	if set.hasEpoch(epochs[0] - 1) {
		t.Fatal("Invalid epoch contained")
	}
	if set.hasEpoch(epochs[len(epochs)-1] + 1) {
		t.Fatal("Invalid epoch contained")
	}
}

func TestBeEpochSetAddOtherSet(t *testing.T) {
	epochs1 := []int64{7, 11, 17}
	epochs2 := []int64{13, 23, 27}

	set1 := beEpochSet{}
	set1.clear()
	for _, e := range epochs1 {
		set1.add(e)
	}

	set2 := beEpochSet{}
	set2.clear()
	for _, e := range epochs2 {
		set2.add(e)
	}

	set1.addSet(&set2)

	if set1.getMinEpoch() != epochs1[0] {
		t.Fatal("Invalid min epoch set")
	}
	if set1.getMaxEpoch() != epochs2[len(epochs2)-1] {
		t.Fatal("Invalid max epoch set")
	}
	for _, e := range epochs1 {
		if !set1.hasEpoch(e) {
			t.Fatal("Valid epoch not contained")
		}
	}
	for _, e := range epochs2 {
		if !set1.hasEpoch(e) {
			t.Fatal("Valid epoch not contained")
		}
	}
	if set1.hasEpoch(epochs1[0] - 1) {
		t.Fatal("Invalid epoch contained")
	}
	if set1.hasEpoch(epochs2[len(epochs2)-1] + 1) {
		t.Fatal("Invalid epoch contained")
	}
}
