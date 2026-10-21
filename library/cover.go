package library

import (
    "bytes"
    "errors"
    "fmt"
    "image"
    "image/color"
    _ "image/gif"
    "image/jpeg"
    _ "image/png"
    "os"
    "path/filepath"
    "strings"

    _ "golang.org/x/image/bmp"
    "golang.org/x/image/draw"
    _ "golang.org/x/image/webp"
)

const (
    customCoverSide = 1200
    embeddedSuffix  = "#cover"
)

var ImageExtensions = []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp"}

type Cover struct {
    Name string
    Data []byte
}

func FindCover(audioPath string, embedded []byte) (*Cover, error) {
    directory := filepath.Dir(audioPath)
    audioName := strings.TrimSuffix(
        filepath.Base(audioPath),
        filepath.Ext(audioPath),
    )

    entries, err := os.ReadDir(directory)
    if err != nil {
        return nil, fmt.Errorf("reading audio directory: %w", err)
    }

    // Prefer artwork that matches the audio filename.
    matching := []string{
        audioName + ".jpg",
        audioName + ".jpeg",
        audioName + ".png",
    }

    for _, name := range matching {
        if cover := readCover(filepath.Join(directory, name)); cover != nil {
            return cover, nil
        }
    }

    if isDecodableImage(embedded) {
        return &Cover{Name: audioPath + embeddedSuffix, Data: embedded}, nil
    }

    preferred := []string{
        "cover.jpg",
        "cover.png",
        "folder.jpg",
        "folder.png",
        "front.jpg",
        "front.png",
    }

    for _, name := range preferred {
        if cover := readCover(filepath.Join(directory, name)); cover != nil {
            return cover, nil
        }
    }

    // Otherwise, use the first supported image.
    for _, entry := range entries {
        if entry.IsDir() || !isImageFile(entry.Name()) {
            continue
        }

        if cover := readCover(filepath.Join(directory, entry.Name())); cover != nil {
            return cover, nil
        }
    }

    return nil, nil
}

func (c *Cover) File() string {
    if strings.HasSuffix(c.Name, embeddedSuffix) {
        return ""
    }

    return c.Name
}

func Thumbnail(data []byte, side int) (image.Image, error) {
    source, _, err := image.Decode(bytes.NewReader(data))
    if err != nil {
        return nil, err
    }

    bounds := source.Bounds()
    scale := float64(side) / float64(max(bounds.Dx(), bounds.Dy()))
    if scale >= 1 {
        return source, nil
    }

    thumbnail := image.NewRGBA(image.Rect(
        0,
        0,
        max(int(float64(bounds.Dx())*scale), 1),
        max(int(float64(bounds.Dy())*scale), 1),
    ))
    draw.CatmullRom.Scale(thumbnail, thumbnail.Bounds(), source, bounds, draw.Src, nil)

    return thumbnail, nil
}

func PrepareCover(data []byte) ([]byte, error) {
    if !isDecodableImage(data) {
        return nil, errors.New("this file isn't an image the app can read; choose a JPEG, PNG, GIF, BMP or WebP image")
    }

    scaled, err := Thumbnail(data, customCoverSide)
    if err != nil {
        return nil, fmt.Errorf("reading the image: %w", err)
    }

    flattened := image.NewRGBA(scaled.Bounds())
    draw.Draw(flattened, flattened.Bounds(), image.White, image.Point{}, draw.Src)
    draw.Draw(flattened, flattened.Bounds(), scaled, scaled.Bounds().Min, draw.Over)

    var encoded bytes.Buffer
    if err := jpeg.Encode(&encoded, flattened, &jpeg.Options{Quality: 90}); err != nil {
        return nil, fmt.Errorf("saving the image: %w", err)
    }

    return encoded.Bytes(), nil
}

func AverageColor(data []byte) (color.NRGBA, bool) {
    thumbnail, err := Thumbnail(data, 32)
    if err != nil {
        return color.NRGBA{}, false
    }

    bounds := thumbnail.Bounds()
    var red, green, blue, count float64
    for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
        for x := bounds.Min.X; x < bounds.Max.X; x++ {
            r, g, b, a := thumbnail.At(x, y).RGBA()
            if a == 0 {
                continue
            }

            red += float64(r)
            green += float64(g)
            blue += float64(b)
            count++
        }
    }

    if count == 0 {
        return color.NRGBA{}, false
    }

    red, green, blue = red/count/257, green/count/257, blue/count/257
    gray := (red + green + blue) / 3
    vivid := func(channel float64) uint8 {
        return uint8(min(max(gray+(channel-gray)*1.4, 0), 255))
    }

    return color.NRGBA{R: vivid(red), G: vivid(green), B: vivid(blue), A: 255}, true
}

func readCover(path string) *Cover {
    data, err := os.ReadFile(path)
    if err != nil || !isDecodableImage(data) {
        return nil
    }

    return &Cover{Name: path, Data: data}
}

func isDecodableImage(data []byte) bool {
    if len(data) == 0 {
        return false
    }

    _, _, err := image.DecodeConfig(bytes.NewReader(data))
    return err == nil
}

func isImageFile(filename string) bool {
    extensions := []string{
        ".jpg",
        ".jpeg",
        ".png",
        ".gif",
        ".bmp",
        ".tiff",
        ".webp",
    }

    extension := strings.ToLower(filepath.Ext(filename))

    for _, ext := range extensions {
        if extension == ext {
            return true
        }
    }

    return false
}
