package bcn

import (
	"encoding/binary"
	"io"
)

// ReadDDSHeader reads DDS magic + header.
func ReadDDSHeader(r io.Reader) (*DDSHeader, error) {
	var magic uint32
	if err := binary.Read(r, binary.LittleEndian, &magic); err != nil {
		return nil, err
	}

	if magic != DDSMagic {
		return nil, ErrInvalidDDSMagic
	}

	var h DDSHeader
	if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
		return nil, err
	}

	if h.Size != DDSHeaderSize {
		return nil, ErrInvalidDDSHeaderSize
	}

	if h.PixelFormat.Size != DDSPixelFormatSize {
		return nil, ErrInvalidDDSPixelFormatSize
	}

	return &h, nil
}

// ReadDDSHeaderDX10 reads the optional DX10 header.
func ReadDDSHeaderDX10(r io.Reader, h *DDSHeader) (*DDSHeaderDX10, error) {
	if h == nil {
		return nil, nil
	}

	if (h.PixelFormat.Flags&DDSPFFourCC == 0) || h.PixelFormat.FourCC != DDSFourCCDX10 {
		return nil, nil
	}

	var dx10 DDSHeaderDX10
	if err := binary.Read(r, binary.LittleEndian, &dx10); err != nil {
		return nil, err
	}

	return &dx10, nil
}

// WriteDDSMagic writes DDS magic.
func WriteDDSMagic(w io.Writer) error {
	return binary.Write(w, binary.LittleEndian, uint32(DDSMagic))
}

// WriteDDSHeader writes DDS header (without magic).
func WriteDDSHeader(w io.Writer, h *DDSHeader) error {
	if h == nil {
		return ErrNilDDSHeader
	}

	return binary.Write(w, binary.LittleEndian, h)
}
