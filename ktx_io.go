package bcn

import (
	"encoding/binary"
	"io"
)

// ReadKTXHeader reads a KTX v1 header (no payload).
func ReadKTXHeader(r io.Reader) (*KTXHeader, error) {
	var h KTXHeader
	if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
		return nil, err
	}

	if h.Identifier != KTXIdentifier {
		return nil, ErrInvalidKTXIdentifier
	}

	if h.Endianness != KTXEndianness {
		return nil, ErrUnsupportedKTXEndianness
	}

	return &h, nil
}

// WriteKTXHeader writes a KTX v1 header (no payload).
func WriteKTXHeader(w io.Writer, h *KTXHeader) error {
	if h == nil {
		return ErrNilKTXHeader
	}

	return binary.Write(w, binary.LittleEndian, h)
}
