package video

type VideoSource interface {
	ReadFrame() ([]byte, error)
	Close() error
}
