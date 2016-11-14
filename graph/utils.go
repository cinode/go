package graph

func (d *DirEntry) clone(withNode bool) DirEntry {
	ret := DirEntry{
		Node:     d.Node,
		Metadata: make(map[string]string),
	}
	if withNode {
		ret.Node = ret.Node.clone()
	}
	for k, v := range d.Metadata {
		ret.Metadata[k] = v
	}
	return ret
}

func (d DirEntryMap) clone(withNodes bool) DirEntryMap {
	ret := DirEntryMap{}
	for n, e := range d {
		ret[n] = e.clone(withNodes)
	}
	return ret
}
