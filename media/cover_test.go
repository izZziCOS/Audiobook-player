package media

import (
    "bytes"
    "image"
    "image/color"
    "image/png"
    "testing"
)

func testCover(t *testing.T) []byte {
    t.Helper()

    picture := image.NewNRGBA(image.Rect(0, 0, 64, 64))
    for x := range 64 {
        for y := range 64 {
            picture.Set(x, y, color.NRGBA{R: uint8(x * 4), G: 90, B: uint8(y * 4), A: 255})
        }
    }

    var encoded bytes.Buffer
    if err := png.Encode(&encoded, picture); err != nil {
        t.Fatal(err)
    }

    return encoded.Bytes()
}
