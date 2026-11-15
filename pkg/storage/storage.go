package storage

type ObjectInfo struct {
	ContentLength int64
	ContentType   string
}

type Storage interface {
	Download(bucket, object string) ([]byte, error)
	Upload(bucket string, object string, content []byte) error
	Head(bucket, object string) (*ObjectInfo, error)
}
