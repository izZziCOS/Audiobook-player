package library

import (
    "regexp"
    "strings"

    "github.com/dhowden/tag"
    "github.com/izzzicos/audiobook-player/vfs"
)

type Metadata struct {
    Title   string
    Album   string
    Artist  string
    ASIN    string
    Picture []byte
}

var asinPattern = regexp.MustCompile(`^[0-9A-Z]{10}$`)

func ReadMetadata(path string) (metadata Metadata) {
    defer func() {
        if recover() != nil {
            metadata = Metadata{}
        }
    }()

    file, err := vfs.Open(path)
    if err != nil {
        return Metadata{}
    }
    defer file.Close()

    tags, err := tag.ReadFrom(file)
    if err != nil {
        return Metadata{}
    }

    metadata = Metadata{
        Title:  CleanText(tags.Title()),
        Album:  CleanText(tags.Album()),
        Artist: CleanText(tags.Artist()),
    }
    if metadata.Artist == "" {
        metadata.Artist = CleanText(tags.AlbumArtist())
    }

    metadata.ASIN = readASIN(tags.Raw())

    if picture := tags.Picture(); picture != nil {
        metadata.Picture = picture.Data
    }

    return metadata
}

func readASIN(raw map[string]interface{}) string {
    for key, value := range raw {
        var text string
        switch value := value.(type) {
        case string:
            if strings.EqualFold(key, "asin") || strings.EqualFold(key, "audible_asin") {
                text = value
            }
        case *tag.Comm:
            if strings.EqualFold(value.Description, "asin") {
                text = value.Text
            }
        }

        if asin := strings.ToUpper(CleanText(text)); asinPattern.MatchString(asin) {
            return asin
        }
    }

    return ""
}

func (m Metadata) Override(title, author string) Metadata {
    if title != "" {
        m.Album = title
    }
    if author != "" {
        m.Artist = author
    }

    return m
}

func CleanText(value string) string {
    return strings.Join(strings.Fields(strings.ReplaceAll(value, "\x00", " ")), " ")
}
