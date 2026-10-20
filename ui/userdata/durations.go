package userdata

import (
    "os"

    "github.com/izzzicos/audiobook-player/audio"
)

type PartDuration struct {
    Size     int64   `json:"size"`
    Modified int64   `json:"modified"`
    Seconds  float64 `json:"seconds"`
}

func (d *Data) CachedDuration(path string) (float64, bool) {
    return LookupDuration(d.Durations, path)
}

func LookupDuration(cache map[string]PartDuration, path string) (float64, bool) {
    cached, found := cache[path]
    if !found {
        return 0, false
    }

    info, err := os.Stat(path)
    if err != nil || info.Size() != cached.Size || info.ModTime().UnixNano() != cached.Modified {
        return 0, false
    }

    return cached.Seconds, true
}

func MeasureDuration(path string) (PartDuration, error) {
    info, err := os.Stat(path)
    if err != nil {
        return PartDuration{}, err
    }

    seconds, err := audio.Duration(path)
    if err != nil {
        return PartDuration{}, err
    }

    return PartDuration{
        Size:     info.Size(),
        Modified: info.ModTime().UnixNano(),
        Seconds:  seconds,
    }, nil
}

func (d *Data) CacheDuration(path string, seconds float64) {
    info, err := os.Stat(path)
    if err != nil {
        return
    }

    d.Durations[path] = PartDuration{
        Size:     info.Size(),
        Modified: info.ModTime().UnixNano(),
        Seconds:  seconds,
    }
}
