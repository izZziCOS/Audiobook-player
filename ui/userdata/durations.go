package userdata

import (
	"github.com/izzzicos/audiobook-player/audio"
	"github.com/izzzicos/audiobook-player/vfs"
)

type PartDuration struct {
	Size     int64   `json:"size"`
	Modified int64   `json:"modified"`
	Seconds  float64 `json:"seconds"`
}

func (d *Data) CachedDuration(path string) (float64, bool) {
	return LookupDurationOf(d.Durations, path, path)
}

func (d *Data) CachedDurationOf(part, file string) (float64, bool) {
	return LookupDurationOf(d.Durations, part, file)
}

func LookupDuration(cache map[string]PartDuration, path string) (float64, bool) {
	return LookupDurationOf(cache, path, path)
}

func LookupDurationOf(cache map[string]PartDuration, part, file string) (float64, bool) {
	cached, found := cache[part]
	if !found {
		return 0, false
	}

	info, err := vfs.Stat(file)
	if err != nil || info.Size() != cached.Size || info.ModTime().UnixNano() != cached.Modified {
		return 0, false
	}

	return cached.Seconds, true
}

func MeasureDuration(path string) (PartDuration, error) {
	info, err := vfs.Stat(path)
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
	d.CacheDurationOf(path, path, seconds)
}

func (d *Data) CacheDurationOf(part, file string, seconds float64) {
	info, err := vfs.Stat(file)
	if err != nil {
		return
	}

	d.Durations[part] = PartDuration{
		Size:     info.Size(),
		Modified: info.ModTime().UnixNano(),
		Seconds:  seconds,
	}
}
