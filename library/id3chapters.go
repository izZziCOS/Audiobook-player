package library

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"unicode/utf16"
)

const maxID3Tag = 64 << 20

func syncsafe(b []byte) int {
	return int(b[0]&0x7F)<<21 | int(b[1]&0x7F)<<14 | int(b[2]&0x7F)<<7 | int(b[3]&0x7F)
}

func removeUnsynchronisation(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte{0xFF, 0x00}, []byte{0xFF})
}

func id3Chapters(file io.ReaderAt) []Chapter {
	header := make([]byte, 10)
	if n, _ := file.ReadAt(header, 0); n < 10 || string(header[:3]) != "ID3" {
		return nil
	}

	version, flags := header[3], header[5]
	if version != 3 && version != 4 {
		return nil
	}

	size := syncsafe(header[6:10])
	if size <= 0 || size > maxID3Tag {
		return nil
	}
	body := make([]byte, size)
	if n, _ := file.ReadAt(body, 10); n < size {
		return nil
	}
	if version == 3 && flags&0x80 != 0 {
		body = removeUnsynchronisation(body)
	}

	if flags&0x40 != 0 && len(body) >= 4 {
		skip := int(binary.BigEndian.Uint32(body)) + 4
		if version == 4 {
			skip = syncsafe(body[:4])
		}
		if skip < 0 || skip > len(body) {
			return nil
		}
		body = body[skip:]
	}

	var chapters []Chapter
	for _, frame := range tagFrames(body, version) {
		if frame.id != "CHAP" || len(chapters) >= maxChapters {
			continue
		}
		if chapter, ok := parseChapterFrame(frame.data, version); ok {
			chapters = append(chapters, chapter)
		}
	}

	return chapters
}

type tagFrame struct {
	id   string
	data []byte
}

func tagFrames(data []byte, version byte) []tagFrame {
	var frames []tagFrame
	for len(data) >= 10 && data[0] != 0 {
		id := string(data[:4])
		size := int(binary.BigEndian.Uint32(data[4:8]))
		if version == 4 {
			size = syncsafe(data[4:8])
		}
		formatFlags := data[9]
		if size < 0 || 10+size > len(data) {
			break
		}

		content := data[10 : 10+size]
		data = data[10+size:]

		if version == 4 {
			if formatFlags&0x0C != 0 {
				continue
			}
			if formatFlags&0x01 != 0 {
				if len(content) < 4 {
					continue
				}
				content = content[4:]
			}
			if formatFlags&0x02 != 0 {
				content = removeUnsynchronisation(content)
			}
		} else if formatFlags&0xC0 != 0 {
			continue
		}

		frames = append(frames, tagFrame{id: id, data: content})
	}

	return frames
}

func parseChapterFrame(data []byte, version byte) (Chapter, bool) {
	end := bytes.IndexByte(data, 0)
	if end < 0 || end+17 > len(data) {
		return Chapter{}, false
	}

	times := data[end+1:]
	chapter := Chapter{Start: float64(binary.BigEndian.Uint32(times)) / 1000}
	for _, sub := range tagFrames(times[16:], version) {
		if sub.id == "TIT2" {
			chapter.Title = id3Text(sub.data)
			break
		}
	}

	return chapter, true
}

func id3Text(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	encoding, text := data[0], data[1:]
	switch encoding {
	case 1, 2:
		bigEndian := encoding == 2
		if len(text) >= 2 && text[0] == 0xFE && text[1] == 0xFF {
			bigEndian, text = true, text[2:]
		} else if len(text) >= 2 && text[0] == 0xFF && text[1] == 0xFE {
			bigEndian, text = false, text[2:]
		}

		units := make([]uint16, 0, len(text)/2)
		for i := 0; i+1 < len(text); i += 2 {
			unit := binary.LittleEndian.Uint16(text[i:])
			if bigEndian {
				unit = binary.BigEndian.Uint16(text[i:])
			}
			if unit == 0 {
				break
			}
			units = append(units, unit)
		}
		return string(utf16.Decode(units))
	case 3:
		return strings.TrimRight(string(text), "\x00")
	default:
		if end := bytes.IndexByte(text, 0); end >= 0 {
			text = text[:end]
		}
		runes := make([]rune, len(text))
		for i, b := range text {
			runes[i] = rune(b)
		}
		return string(runes)
	}
}
