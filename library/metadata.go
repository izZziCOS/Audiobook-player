package library

import (
    "os"
    "strings"

    "github.com/dhowden/tag"
)

type Metadata struct {
    Title   string
    Album   string
    Artist  string
    Picture []byte
}

func ReadMetadata(path string) (metadata Metadata) {
    defer func() {
        if recover() != nil {
            metadata = Metadata{}
        }
    }()

    file, err := os.Open(path)
    if err != nil {
        return Metadata{}
    }
    defer file.Close()

    tags, err := tag.ReadFrom(file)
    if err != nil {
        return Metadata{}
    }

    metadata = Metadata{
        Title:  cleanTag(tags.Title()),
        Album:  cleanTag(tags.Album()),
        Artist: cleanTag(tags.Artist()),
    }
    if metadata.Artist == "" {
        metadata.Artist = cleanTag(tags.AlbumArtist())
    }

    if picture := tags.Picture(); picture != nil {
        metadata.Picture = picture.Data
    }

    return metadata
}

func cleanTag(value string) string {
    return strings.Join(strings.Fields(strings.ReplaceAll(value, "\x00", " ")), " ")
}
