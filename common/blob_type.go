package common

type BlobType struct {
	t byte
}

func NewBlobType(t byte) BlobType {
	return BlobType{t: t}
}
