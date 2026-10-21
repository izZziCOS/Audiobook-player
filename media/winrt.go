package media

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var parameterizedNamespace = []byte{0x11, 0xf4, 0x7a, 0xd5, 0x7b, 0x73, 0x42, 0xc0, 0xab, 0xae, 0x87, 0x8b, 0x1e, 0x16, 0xad, 0xee}

func parameterizedIID(signature string) guid {
	sum := sha1.Sum(append(append([]byte{}, parameterizedNamespace...), signature...))
	sum[6] = sum[6]&0x0f | 0x50
	sum[8] = sum[8]&0x3f | 0x80

	id := guid{
		Data1: binary.BigEndian.Uint32(sum[0:4]),
		Data2: binary.BigEndian.Uint16(sum[4:6]),
		Data3: binary.BigEndian.Uint16(sum[6:8]),
	}
	copy(id.Data4[:], sum[8:16])

	return id
}

func (id guid) String() string {
	return fmt.Sprintf(
		"{%08x-%04x-%04x-%02x%02x-%x}",
		id.Data1, id.Data2, id.Data3, id.Data4[0], id.Data4[1], id.Data4[2:],
	)
}

func classSignature(name string, defaultInterface guid) string {
	return "rc(" + name + ";" + defaultInterface.String() + ")"
}

func typedEventHandlerSignature(sender, args string) string {
	return "pinterface(" + typedEventHandler.String() + ";" + sender + ";" + args + ")"
}

var (
	typedEventHandler = guid{0x9de1c534, 0x6ae1, 0x11e0, [8]byte{0x84, 0xe1, 0x18, 0xa9, 0x05, 0xbc, 0xc5, 0x3f}}

	iidUnknown                      = guid{0x00000000, 0x0000, 0x0000, [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidAgileObject                  = guid{0x94ea2b94, 0xe9cc, 0x49e0, [8]byte{0xc0, 0xff, 0xee, 0x64, 0xca, 0x8f, 0x5b, 0x90}}
	iidTransportControlsInterop     = guid{0xddb0472d, 0xc911, 0x4a1f, [8]byte{0x86, 0xd9, 0xdc, 0x3d, 0x71, 0xa9, 0x5f, 0x5a}}
	iidTransportControls            = guid{0x99fa3ff4, 0x1742, 0x42a6, [8]byte{0x90, 0x2e, 0x08, 0x7d, 0x41, 0xf9, 0x65, 0xec}}
	iidButtonPressedEventArgs       = guid{0xb7f47116, 0xa56f, 0x4dc8, [8]byte{0x9e, 0x11, 0x92, 0x03, 0x1f, 0x4a, 0x87, 0xc2}}
	iidRandomAccessStream           = guid{0x905a0fe1, 0xbc53, 0x11df, [8]byte{0x8c, 0x49, 0x00, 0x1e, 0x4f, 0xc6, 0x86, 0xda}}
	iidRandomAccessStreamReferences = guid{0x857309dc, 0x3fbf, 0x4e7d, [8]byte{0x98, 0x6f, 0xef, 0x3b, 0x1a, 0x07, 0xa9, 0x64}}

	iidButtonPressedHandler = parameterizedIID(typedEventHandlerSignature(
		classSignature("Windows.Media.SystemMediaTransportControls", iidTransportControls),
		classSignature("Windows.Media.SystemMediaTransportControlsButtonPressedEventArgs", iidButtonPressedEventArgs),
	))
)

type transportButton int32

const (
	buttonPlay        transportButton = 0
	buttonPause       transportButton = 1
	buttonStop        transportButton = 2
	buttonFastForward transportButton = 4
	buttonRewind      transportButton = 5
	buttonNext        transportButton = 6
	buttonPrevious    transportButton = 7
)

func commandForButton(button transportButton) (Command, bool) {
	switch button {
	case buttonPlay:
		return Play, true
	case buttonPause, buttonStop:
		return Pause, true
	case buttonNext:
		return Next, true
	case buttonPrevious:
		return Previous, true
	case buttonFastForward:
		return SkipForward, true
	case buttonRewind:
		return SkipBackward, true
	default:
		return 0, false
	}
}
