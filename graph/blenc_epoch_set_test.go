package graph

import "testing"

func TestBlencEpochSetClear(t *testing.T) {
	set := blencEpochSet{}
	set.clear()
	if !set.isEmpty() {
		t.Fatal("beEpochSet is not empty after reset")
	}
}

func TestBlencEpochSetSingleEpoch(t *testing.T) {
	set := blencEpochSet{}
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

func TestBlencEpochSetMultipleEpochs(t *testing.T) {
	epochs := []int64{7, 11, 17}

	set := blencEpochSet{}
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

func TestBlencEpochSetAddOtherSet(t *testing.T) {
	epochs1 := []int64{7, 11, 17}
	epochs2 := []int64{13, 23, 27}
	epochs3 := []int64{3, 5, 7}

	set1 := blencEpochSetEmpty
	for _, e := range epochs1 {
		set1.add(e)
	}

	set2 := blencEpochSetEmpty
	for _, e := range epochs2 {
		set2.add(e)
	}

	set3 := blencEpochSetEmpty
	for _, e := range epochs3 {
		set3.add(e)
	}

	set1.addSet(set2)
	set1.addSet(set3)

	if set1.getMinEpoch() != epochs3[0] {
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
	for _, e := range epochs3 {
		if !set1.hasEpoch(e) {
			t.Fatal("Valid epoch not contained")
		}
	}
	if set1.hasEpoch(epochs3[0] - 1) {
		t.Fatal("Invalid epoch contained")
	}
	if set1.hasEpoch(epochs2[len(epochs2)-1] + 1) {
		t.Fatal("Invalid epoch contained")
	}
}

func TestBlencEpochSetOverlaps(t *testing.T) {
	for _, d := range []struct {
		s1, s2   []int64
		overlaps bool
		name     string
	}{
		{[]int64{}, []int64{}, false, "Empty sets don't overlap"},
		{[]int64{}, []int64{1, 2, 3}, false, "Empty set don't overlap with anything"},
		{[]int64{1, 5, 7}, []int64{11, 15, 16}, false, "Disjoint sets don't overlap"},
		{[]int64{1, 5, 7}, []int64{7, 15, 16}, true, "Single value overlap"},
		{[]int64{1, 5, 7}, []int64{7, 5, 16}, true, "Multiple values overlap"},
	} {
		set1, set2 := blencEpochSetEmpty, blencEpochSetEmpty
		for _, e := range d.s1 {
			set1.add(e)
		}
		for _, e := range d.s2 {
			set2.add(e)
		}

		for _, ovCheck := range []bool{
			set1.overlaps(set2),
			set2.overlaps(set1),
		} {
			if d.overlaps != ovCheck {
				t.Fatalf("Failed overlapping test '%s', expected %v, got %v, set1: %s, set2: %s",
					d.name, d.overlaps, ovCheck, set1, set2)
			}
		}

	}
}

func TestBlencEpochSetContains(t *testing.T) {
	for _, d := range []struct {
		s1, s2   []int64
		contains bool
		name     string
	}{
		{[]int64{}, []int64{}, true, "Empty sets is contained by  empty set"},
		{[]int64{1, 2, 3}, []int64{}, true, "Empty set is contained by any other set"},
		{[]int64{1, 5, 7}, []int64{11, 15, 16}, false, "Disjoint sets don't contain themselves"},
		{[]int64{1, 5, 7}, []int64{7}, true, "Single value containment"},
		{[]int64{1, 5, 7}, []int64{1, 5}, true, "Multiple values containment"},
	} {
		set1, set2 := blencEpochSetEmpty, blencEpochSetEmpty
		for _, e := range d.s1 {
			set1.add(e)
		}
		for _, e := range d.s2 {
			set2.add(e)
		}

		cnCheck := set1.contains(set2)
		if d.contains != cnCheck {
			t.Fatalf("Failed containment test '%s', expected %v, got %v, set1: %s, set2: %s",
				d.name, d.contains, cnCheck, set1, set2)
		}
	}
}

func TestBlencEpochSetString(t *testing.T) {
	set := blencEpochSetEmpty
	if set.String() != "{ }" {
		t.Fatalf("Invalid string representation of empty set")
	}

	set.add(1)
	if set.String() != "{1..1}" {
		t.Fatalf("Invalid string representation of single-value set")
	}

	set.add(5)
	if set.String() != "{1..5}" {
		t.Fatalf("Invalid string representation of multi-value set")
	}
}
