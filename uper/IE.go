package uper

type IE interface {
	Encode(*UperWriter) error
	Decode(*UperReader) error
}

