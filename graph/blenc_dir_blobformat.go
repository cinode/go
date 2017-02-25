package graph

import (
	"bytes"
	"io"
	"io/ioutil"
)

func blencDirBlobFormatSerialize(entries blencEntriesMap) (io.ReadCloser, error) {
	b := bytes.Buffer{}
	s := newBlencWriter(&b)

	if entries == nil {
		s.UInt(uint64(0))
	} else {
		s.UInt(uint64(len(entries)))

		for n, de := range entries {
			s.String(n)
			s.UInt(blencNodeType(de.node))
			s.String(de.bid)
			s.UInt(beKeyInfoTypeValue)
			s.String(de.key)

			// TODO: Metadata
			s.UInt(0)
			/*
			   s.UInt(uint64(len(de.Metadata)))
			   for k, v := range de.Metadata {
			       s.String(k)
			       s.String(v)
			   }
			*/
		}
	}

	if s.err != nil {
		return nil, s.err
	}

	return ioutil.NopCloser(bytes.NewReader(b.Bytes())), nil
}

func blencDirBlobFormatDeserialize(rawReader io.Reader, ep *blencEP) (blencEntriesMap, error) {
	r := newBlencReader(rawReader)

	// Get number of entries and validate them
	entriesCnt := r.UInt()
	if entriesCnt > blencDirMaxEntries {
		return nil, ErrMalformedDirectoryBlob
	}
	entries := make(blencEntriesMap)

	for ; entriesCnt > 0; entriesCnt-- {

		// Get the name and validate it
		name := r.String(MaxEntryNameLength)
		if _, found := entries[name]; found || name == "" {
			return nil, ErrMalformedDirectoryBlob
		}

		// Create subnode instance
		nodeType := r.UInt()
		bid := r.String(blencMaxBlobNameLen)

		// KeyInfo is limited to a directly embedded key now
		keyInfoType := r.UInt()
		if keyInfoType != beKeyInfoTypeValue {
			return nil, ErrMalformedDirectoryBlob
		}
		key := r.String(blencMaxKeyLen)

		node := blencNewNode(nodeType, ep)
		if node == nil {
			return nil, ErrMalformedDirectoryBlob
		}

		nodeBase := toBlencNodeBase(node)
		nodeBase.bid = bid
		nodeBase.key = key

		entry := blencDirNodeEntry{
			bid:             bid,
			key:             key,
			node:            node,
			unsavedEpochSet: blencEpochSetEmpty,
		}
		metaCount := r.UInt()
		if metaCount > 0 {
			return nil, ErrMalformedDirectoryBlob
		}
		/*
		   if metaCount > maxMetaEntries {
		       return ErrMalformedDirectoryBlob
		   }
		   for ; metaCount > 0; metaCount-- {
		       key := r.String(maxMetaKeyLen)
		       if _, exists := entry.Metadata[key]; exists || key == "" {
		           return ErrMalformedDirectoryBlob
		       }
		       entry.Metadata[key] = r.String(maxMetaValueLen)
		   }
		*/

		entries[name] = &entry
	}

	if r.err != nil {
		return nil, r.err
	}

	return entries, nil
}
