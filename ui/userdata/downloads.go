package userdata

type DownloadPart struct {
	Source   string `json:"source"`
	File     string `json:"file"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"`
	Done     bool   `json:"done,omitempty"`
}

type Download struct {
	Folder    string         `json:"folder"`
	Single    bool           `json:"single,omitempty"`
	Parts     []DownloadPart `json:"parts"`
	Cover     string         `json:"cover,omitempty"`
	Complete  bool           `json:"complete,omitempty"`
	Automatic bool           `json:"automatic,omitempty"`
}

func (d Download) Bytes() int64 {
	var total int64
	for _, part := range d.Parts {
		if part.Done {
			total += part.Size
		}
	}

	return total
}

func (d Download) TotalBytes() int64 {
	var total int64
	for _, part := range d.Parts {
		total += part.Size
	}

	return total
}

func (d *Data) Download(book string) (Download, bool) {
	download, found := d.downloads[book]
	return download, found
}

func (d *Data) Downloads() map[string]Download {
	return d.downloads
}

func (d *Data) SetDownload(book string, download Download) {
	d.downloads[book] = download
	d.MarkDirty()
}

func (d *Data) ForgetDownload(book string) {
	if _, found := d.downloads[book]; found {
		delete(d.downloads, book)
		d.MarkDirty()
	}
}

func (d *Data) moveDownload(old, newPath string) {
	download, found := d.downloads[old]
	if !found {
		return
	}

	for i := range download.Parts {
		download.Parts[i].Source = MovedPath(download.Parts[i].Source, old, newPath)
	}
	delete(d.downloads, old)
	if _, taken := d.downloads[newPath]; !taken {
		d.downloads[newPath] = download
	}
}
