package library

import (
    "cmp"
    "encoding/binary"
    "errors"
    "fmt"
    "io"
    "os"
    "slices"
    "strings"
    "unicode/utf16"
)

const (
    maxMovieBox     = 256 << 20
    maxChapters     = 10000
    maxChapterTitle = 64 << 10
)

type Chapter struct {
    Title string
    Start float64
}

func ReadChapters(path string) []Chapter {
    file, err := os.Open(path)
    if err != nil {
        return nil
    }
    defer file.Close()

    movie, err := readMovieBox(file)
    if err != nil {
        return nil
    }

    chapters := quickTimeChapters(file, movie)
    if len(chapters) == 0 {
        chapters = neroChapters(movie)
    }

    return tidyChapters(chapters)
}

func readMovieBox(file io.ReadSeeker) ([]byte, error) {
    header := make([]byte, 16)
    var position int64

    for first := true; ; first = false {
        if _, err := file.Seek(position, io.SeekStart); err != nil {
            return nil, err
        }
        if _, err := io.ReadFull(file, header[:8]); err != nil {
            return nil, err
        }

        size := int64(binary.BigEndian.Uint32(header))
        kind := string(header[4:8])
        headerSize := int64(8)

        if first && kind != "ftyp" {
            return nil, errors.New("not an MP4 file")
        }

        switch size {
        case 0:
            if kind != "moov" {
                return nil, errors.New("no movie box")
            }
            body, err := io.ReadAll(io.LimitReader(file, maxMovieBox))
            return body, err
        case 1:
            if _, err := io.ReadFull(file, header[8:16]); err != nil {
                return nil, err
            }
            size = int64(binary.BigEndian.Uint64(header[8:16]))
            headerSize = 16
        }

        if size < headerSize {
            return nil, errors.New("invalid box size")
        }

        if kind == "moov" {
            if size-headerSize > maxMovieBox {
                return nil, errors.New("movie box is too large")
            }

            body := make([]byte, size-headerSize)
            if _, err := io.ReadFull(file, body); err != nil {
                return nil, err
            }

            return body, nil
        }

        position += size
    }
}

type mp4Box struct {
    kind string
    body []byte
}

func mp4Boxes(data []byte) []mp4Box {
    var boxes []mp4Box

    for len(data) >= 8 {
        size := uint64(binary.BigEndian.Uint32(data))
        kind := string(data[4:8])
        headerSize := uint64(8)

        switch size {
        case 0:
            size = uint64(len(data))
        case 1:
            if len(data) < 16 {
                return boxes
            }
            size = binary.BigEndian.Uint64(data[8:16])
            headerSize = 16
        }

        if size < headerSize || size > uint64(len(data)) {
            return boxes
        }

        boxes = append(boxes, mp4Box{kind: kind, body: data[headerSize:size]})
        data = data[size:]
    }

    return boxes
}

func mp4Find(data []byte, path ...string) ([]byte, bool) {
    for _, kind := range path {
        found := false
        for _, box := range mp4Boxes(data) {
            if box.kind == kind {
                data = box.body
                found = true
                break
            }
        }

        if !found {
            return nil, false
        }
    }

    return data, true
}

func quickTimeChapters(file io.ReaderAt, movie []byte) []Chapter {
    tracks := map[uint32][]byte{}
    var chapterTracks []uint32

    for _, box := range mp4Boxes(movie) {
        if box.kind != "trak" {
            continue
        }

        if id, ok := trackID(box.body); ok {
            tracks[id] = box.body
        }

        if references, ok := mp4Find(box.body, "tref", "chap"); ok {
            for ; len(references) >= 4; references = references[4:] {
                chapterTracks = append(chapterTracks, binary.BigEndian.Uint32(references))
            }
        }
    }

    for _, id := range chapterTracks {
        if track, ok := tracks[id]; ok {
            if chapters := readChapterTrack(file, track); len(chapters) > 0 {
                return chapters
            }
        }
    }

    return nil
}

func trackID(track []byte) (uint32, bool) {
    header, ok := mp4Find(track, "tkhd")
    if !ok || len(header) < 1 {
        return 0, false
    }

    offset := 12
    if header[0] == 1 {
        offset = 20
    }
    if len(header) < offset+4 {
        return 0, false
    }

    return binary.BigEndian.Uint32(header[offset:]), true
}

func readChapterTrack(file io.ReaderAt, track []byte) []Chapter {
    handler, ok := mp4Find(track, "mdia", "hdlr")
    if !ok || len(handler) < 12 {
        return nil
    }
    if kind := string(handler[8:12]); kind != "text" && kind != "sbtl" {
        return nil
    }

    timescale := mediaTimescale(track)
    if timescale == 0 {
        return nil
    }

    table, ok := mp4Find(track, "mdia", "minf", "stbl")
    if !ok {
        return nil
    }

    starts := sampleStarts(table)
    sizes := sampleSizes(table)
    offsets := sampleOffsets(table, sizes)

    count := min(len(starts), len(sizes), len(offsets))
    chapters := make([]Chapter, 0, count)
    for i := 0; i < count; i++ {
        chapters = append(chapters, Chapter{
            Title: readChapterTitle(file, offsets[i], sizes[i]),
            Start: float64(starts[i]) / float64(timescale),
        })
    }

    return chapters
}

func mediaTimescale(track []byte) uint32 {
    header, ok := mp4Find(track, "mdia", "mdhd")
    if !ok || len(header) < 1 {
        return 0
    }

    offset := 12
    if header[0] == 1 {
        offset = 20
    }
    if len(header) < offset+4 {
        return 0
    }

    return binary.BigEndian.Uint32(header[offset:])
}

func sampleStarts(table []byte) []uint64 {
    times, ok := mp4Find(table, "stts")
    if !ok || len(times) < 8 {
        return nil
    }

    var starts []uint64
    var time uint64
    entries := int(binary.BigEndian.Uint32(times[4:]))

    for i := 0; i < entries && 8+i*8+8 <= len(times); i++ {
        entry := times[8+i*8:]
        count := binary.BigEndian.Uint32(entry)
        delta := uint64(binary.BigEndian.Uint32(entry[4:]))

        for j := uint32(0); j < count; j++ {
            if len(starts) == maxChapters {
                return starts
            }

            starts = append(starts, time)
            time += delta
        }
    }

    return starts
}

func sampleSizes(table []byte) []uint32 {
    sizes, ok := mp4Find(table, "stsz")
    if !ok || len(sizes) < 12 {
        return nil
    }

    uniform := binary.BigEndian.Uint32(sizes[4:])
    count := min(int(binary.BigEndian.Uint32(sizes[8:])), maxChapters)

    result := make([]uint32, 0, count)
    for i := 0; i < count; i++ {
        if uniform != 0 {
            result = append(result, uniform)
            continue
        }

        if 12+i*4+4 > len(sizes) {
            break
        }
        result = append(result, binary.BigEndian.Uint32(sizes[12+i*4:]))
    }

    return result
}

func sampleOffsets(table []byte, sizes []uint32) []uint64 {
    chunks := chunkOffsets(table)

    layout, ok := mp4Find(table, "stsc")
    if !ok || len(layout) < 8 {
        return nil
    }

    type run struct {
        firstChunk      int
        samplesPerChunk int
    }

    var runs []run
    entries := int(binary.BigEndian.Uint32(layout[4:]))
    for i := 0; i < entries && 8+i*12+12 <= len(layout); i++ {
        entry := layout[8+i*12:]
        runs = append(runs, run{
            firstChunk:      int(binary.BigEndian.Uint32(entry)),
            samplesPerChunk: int(binary.BigEndian.Uint32(entry[4:])),
        })
    }

    var offsets []uint64
    current := 0
    for chunk, offset := range chunks {
        for current+1 < len(runs) && runs[current+1].firstChunk <= chunk+1 {
            current++
        }
        if len(runs) == 0 || runs[current].firstChunk > chunk+1 {
            continue
        }

        for i := 0; i < runs[current].samplesPerChunk && len(offsets) < len(sizes); i++ {
            offsets = append(offsets, offset)
            offset += uint64(sizes[len(offsets)-1])
        }

        if len(offsets) == len(sizes) {
            break
        }
    }

    return offsets
}

func chunkOffsets(table []byte) []uint64 {
    if offsets, ok := mp4Find(table, "stco"); ok && len(offsets) >= 8 {
        count := min(int(binary.BigEndian.Uint32(offsets[4:])), (len(offsets)-8)/4, maxChapters)
        result := make([]uint64, count)
        for i := range result {
            result[i] = uint64(binary.BigEndian.Uint32(offsets[8+i*4:]))
        }
        return result
    }

    if offsets, ok := mp4Find(table, "co64"); ok && len(offsets) >= 8 {
        count := min(int(binary.BigEndian.Uint32(offsets[4:])), (len(offsets)-8)/8, maxChapters)
        result := make([]uint64, count)
        for i := range result {
            result[i] = binary.BigEndian.Uint64(offsets[8+i*8:])
        }
        return result
    }

    return nil
}

func readChapterTitle(file io.ReaderAt, offset uint64, size uint32) string {
    if size < 2 || size > maxChapterTitle || offset > 1<<62 {
        return ""
    }

    sample := make([]byte, size)
    if _, err := file.ReadAt(sample, int64(offset)); err != nil {
        return ""
    }

    length := min(int(binary.BigEndian.Uint16(sample)), len(sample)-2)
    return decodeChapterText(sample[2 : 2+length])
}

func decodeChapterText(text []byte) string {
    if len(text) >= 2 && (text[0] == 0xfe && text[1] == 0xff || text[0] == 0xff && text[1] == 0xfe) {
        order := binary.ByteOrder(binary.BigEndian)
        if text[0] == 0xff {
            order = binary.LittleEndian
        }

        units := make([]uint16, 0, len(text)/2)
        for i := 2; i+1 < len(text); i += 2 {
            units = append(units, order.Uint16(text[i:]))
        }
        return string(utf16.Decode(units))
    }

    return strings.ToValidUTF8(string(text), "")
}

func neroChapters(movie []byte) []Chapter {
    body, ok := mp4Find(movie, "udta", "chpl")
    if !ok || len(body) < 5 {
        return nil
    }

    data := body[4:]
    if body[0] > 0 {
        if len(data) < 5 {
            return nil
        }
        data = data[4:]
    }

    count := int(data[0])
    data = data[1:]

    var chapters []Chapter
    for i := 0; i < count && len(data) >= 9; i++ {
        start := binary.BigEndian.Uint64(data)
        length := int(data[8])
        data = data[9:]
        if len(data) < length {
            break
        }

        chapters = append(chapters, Chapter{
            Title: decodeChapterText(data[:length]),
            Start: float64(start) / 1e7,
        })
        data = data[length:]
    }

    return chapters
}

func tidyChapters(chapters []Chapter) []Chapter {
    slices.SortStableFunc(chapters, func(a, b Chapter) int {
        return cmp.Compare(a.Start, b.Start)
    })

    for i := range chapters {
        chapters[i].Title = CleanText(chapters[i].Title)
        if chapters[i].Title == "" {
            chapters[i].Title = fmt.Sprintf("Chapter %d", i+1)
        }
        chapters[i].Start = max(chapters[i].Start, 0)
    }

    return chapters
}
