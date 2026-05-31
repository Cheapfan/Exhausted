package audio

type AudioSource interface {
	ReadSample() ([]int16, error)
	Close() error
}
