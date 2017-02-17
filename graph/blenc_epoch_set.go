package graph

import (
	"fmt"
	"math"
)

var beEpochSetEmpty = beEpochSet{min: math.MaxInt64, max: math.MinInt64}

// beEpochSet holds a set of unsaved epochs, in contains only the min and max
// vale which is enough to hold unsaved epoch set
// Some rules:
//  * if beEpochSet is empty, min == MaxInt64 and max = MinInt64, this
//    eliminates a lot of special cases
//  * in all other cases: min <= max and min != MinInt64 and max != MaxInt64,
//    this implies that neither MinInt64 nor MaxInt64 can be a valid epoch
// TODO: This could be converted to a fine-grained set, investigate if there's
//       enough advantage of using it
type beEpochSet struct {
	min int64
	max int64
}

// String returns human-readable set representation considering special cases
// into account
func (s beEpochSet) String() string {
	if s.min > s.max {
		return "{ }"
	}
	return fmt.Sprintf("{%d..%d}", s.min, s.max)
}

// clear resets the set to empty one
func (s *beEpochSet) clear() {
	*s = beEpochSetEmpty
}

// add ensures given epoch is a part of the set
func (s *beEpochSet) add(epoch int64) {
	if epoch > s.max {
		s.max = epoch
	}
	if epoch < s.min {
		s.min = epoch
	}
}

// addSet includes other set in the current one
func (s *beEpochSet) addSet(other *beEpochSet) {
	if other.max > s.max {
		s.max = other.max
	}
	if other.min < s.min {
		s.min = other.min
	}
}

func (s *beEpochSet) isEmpty() bool {
	return s.max < s.min
}

func (s *beEpochSet) getMinEpoch() int64 {
	return s.min
}

func (s *beEpochSet) getMaxEpoch() int64 {
	return s.max
}

func (s *beEpochSet) hasEpoch(epoch int64) bool {
	return epoch >= s.min && epoch <= s.max
}

func (s *beEpochSet) overlaps(o *beEpochSet) bool {
	min := s.min
	if o.min > min {
		min = o.min
	}

	max := s.max
	if o.max < max {
		max = o.max
	}

	return min <= max
}
