package graph

// apply creates new MetadataMap with changes applied, starting with
// original metadata map. The original metadata map will not be modified
// during this function.
func (c *MetadataChange) apply(original MetadataMap) (MetadataMap, error) {

	if c == nil {
		return MetadataMap{}, nil
	}

	// Early check if we can build a valid map at all
	if len(c.Set) > MaxMetadataKeysInNode {
		return nil, ErrTooManyMetadataKeys
	}
	for k, v := range c.Set {
		if len(k) == 0 || len(k) > MaxMetadataKeyLength {
			return nil, ErrInvalidMetadataKey
		}
		if len(v) > MaxMetadataValueLength {
			return nil, ErrInvalidMetadataValue
		}
	}

	// Start with either clean map or the old one
	ret := MetadataMap{}
	if c.KeepOld && original != nil {
		ret = original.clone()
	}

	// Remove keys marked to be cleared
	for _, k := range c.Clear {
		delete(ret, k)
	}

	// Set new keys
	for k, v := range c.Set {
		ret[k] = v
	}

	// We could have crossed the limit during loop with Set.
	if len(ret) > MaxMetadataKeysInNode {
		return nil, ErrTooManyMetadataKeys
	}

	return ret, nil
}
