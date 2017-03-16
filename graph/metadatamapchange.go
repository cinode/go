package graph

func metadataChangesApplied(
	original MetadataMap,
	metadataChange *MetadataChange,
) MetadataMap {

	if metadataChange == nil {
		return MetadataMap{}
	}

	if !metadataChange.DontClear {
		return metadataChange.Set.clone()
	}

	ret := original.clone()
	for _, k := range metadataChange.Clear {
		delete(ret, k)
	}
	for k, v := range metadataChange.Set {
		ret[k] = v
	}

	return ret
}
