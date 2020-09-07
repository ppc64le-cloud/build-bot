package storage

type Storage interface {
	Download(bucket, object string) ([]byte, error)
	Upload(bucket string, object string, content []byte) error
}