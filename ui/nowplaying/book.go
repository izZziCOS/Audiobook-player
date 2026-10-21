package nowplaying

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/izzzicos/audiobook-player/library"
	"github.com/izzzicos/audiobook-player/ui/about"
	"github.com/izzzicos/audiobook-player/ui/details"
	"github.com/izzzicos/audiobook-player/ui/kit"
	"github.com/izzzicos/audiobook-player/ui/userdata"
)

const detailsThumbnailSide = 320

func (p *Player) OpenFile(path string) {
	if p.FolderBookOf != nil {
		if folder := p.FolderBookOf(path); folder != "" && p.openPart(folder, path) {
			return
		}
	}

	p.OpenBook(library.SingleFile(path))
}

func (p *Player) openStored(path string) (*library.Book, error) {
	if p.Offline != nil {
		return p.Offline.Open(path)
	}

	return library.OpenBook(path)
}

func (p *Player) openPart(folder, path string) bool {
	book, err := p.openStored(folder)
	if err != nil {
		return false
	}

	index, found := book.Find(path)
	if !found {
		return false
	}

	p.SavePosition()
	book.Index = index
	position := 0.0
	if saved, found := p.data.Positions[book.Key()]; found && saved.Part == book.Current() {
		position = saved.Position
	}
	p.showBook(book, position)

	return true
}

func (p *Player) OpenFolder(folder string) {
	book, err := p.openStored(folder)
	if err != nil {
		dialog.ShowError(err, p.window)
		return
	}

	p.OpenBook(book)
}

func (p *Player) Resume(folder, path string, fallbackPosition float64) {
	book := p.findBook(folder, path)

	if _, saved := p.data.Positions[book.Key()]; saved {
		p.OpenBook(book)
		return
	}

	p.showBook(book, fallbackPosition)
}

func (p *Player) findBook(folder, path string) *library.Book {
	if folder != "" {
		if book, err := p.openStored(folder); err == nil {
			if index, found := book.Find(path); found {
				book.Index = index
				return book
			}
		}
	}

	if book, err := p.openStored(path); err == nil && book.Folder == "" {
		return book
	}

	return library.SingleFile(path)
}

func (p *Player) OpenBook(book *library.Book) {
	p.SavePosition()

	saved, found := p.data.Positions[book.Key()]
	if !found {
		p.showBook(book, 0)
		return
	}

	index, found := book.Find(saved.Part)
	if !found {
		p.showBook(book, 0)
		return
	}

	book.Index = index
	p.showBook(book, saved.Position)
}

func (p *Player) OpenBookAndPlay(book *library.Book) {
	p.playAfterOpen = true
	p.OpenBook(book)
}

func (p *Player) showBook(book *library.Book, position float64) {
	p.Offline.Prepare(book)
	p.forgetPrefetched()

	p.load(book.Key(), book.Playable(), func(ok bool) {
		if !ok {
			return
		}

		p.pausedAt = time.Time{}
		if saved, found := p.data.Positions[book.Key()]; found && saved.Listened > 0 && position > 0 {
			p.pausedAt = time.Unix(saved.Listened, 0)
		}

		p.book = book
		p.selectSpeed(p.bookSpeed(book.Key()))
		p.updateBookInfo()

		if position > 0 {
			_ = p.audio.SeekTo(position)
			p.refreshProgress()
		}

		p.SavePosition()
		p.loadDurations(book)
		if p.OnBookOpened != nil {
			p.OnBookOpened(book.Key())
		}
		p.playIfAsked()
	})
}

func (p *Player) playIfAsked() {
	if p.playAfterOpen {
		p.playAfterOpen = false
		p.Play()
	}
}

func (p *Player) goToRelativePart(offset int) {
	if p.book == nil {
		return
	}

	p.goToPart(p.book.Index+offset, p.playing, nil)
}

func (p *Player) goToPart(index int, play bool, then func()) {
	if p.book == nil || !p.book.HasPart(index) {
		return
	}

	book := p.book
	p.playAfterOpen = play
	p.load(book.Key(), book.PlayablePart(index), func(ok bool) {
		if !ok || p.book != book {
			return
		}

		book.Index = index
		p.pausedAt = time.Time{}
		p.updateBookInfo()
		if then != nil {
			then()
		}

		if p.playAfterOpen {
			p.playAfterOpen = false
			p.audio.Play()
			p.setPlaying(true)
		}

		p.refreshProgress()
		p.saveMove()
	})
}

func (p *Player) handlePartEnd() {
	if err := p.audio.Err(); err != nil {
		p.setPlaying(false)
		p.SavePosition()
		dialog.ShowError(
			fmt.Errorf("playback stopped because the file could not be read: %w", err),
			p.window,
		)
		return
	}

	stop := p.sleepsAtPartEnd()

	if p.book != nil && p.book.HasPart(p.book.Index+1) {
		p.goToPart(p.book.Index+1, !stop, func() {
			if stop {
				p.fallAsleep()
			}
		})
		return
	}

	p.setPlaying(false)
	p.rewindBook()
	p.markFinished()
	if stop {
		p.fallAsleep()
	}
}

func (p *Player) rewindBook() {
	if p.book != nil && p.book.Index > 0 {
		p.goToPart(0, false, nil)
		return
	}

	_ = p.audio.SeekTo(0)
	p.refreshProgress()
	p.saveMove()
}

func (p *Player) updateBookInfo() {
	setRichText(p.title, p.book.Title(p.metadata))
	p.chapterIndex = p.currentChapter()
	p.setSubtitle(p.subtitleText())

	if p.hasChapters() {
		p.partsButton.SetText("Chapters")
	} else {
		p.partsButton.SetText("Parts")
	}

	p.editButton.Show()
	p.aboutButton.Show()
	for _, object := range []fyne.CanvasObject{p.previousButton, p.nextButton, p.partsButton} {
		if p.book.IsMultiPart() || p.hasChapters() {
			object.Show()
		} else {
			object.Hide()
		}
	}

	p.resetProgress()
	kit.RefreshContent(p.window)
	p.publishTrack()
}

func (p *Player) setSubtitle(text string) {
	setRichText(p.subtitle, text)

	if text == "" {
		p.subtitle.Hide()
	} else {
		p.subtitle.Show()
	}

	kit.RefreshContent(p.window)
}

func (p *Player) applyEdit(key string) {
	edit := p.data.Edit(key)
	p.metadata = p.tags.Override(edit.Title, edit.Author)

	cover := p.foundCover
	if file := p.data.CoverFile(key); file != "" {
		if custom := userdata.ReadCustomCover(file); custom != nil {
			cover = &library.Cover{Name: file, Data: custom}
		}
	}
	p.showCover(cover)
}

func (p *Player) RefreshDetails(key string) {
	if p.book == nil || p.book.Key() != key {
		return
	}

	p.applyEdit(key)
	p.selectSpeed(p.bookSpeed(key))
	setRichText(p.title, p.book.Title(p.metadata))
	p.setSubtitle(p.subtitleText())
	p.publishTrack()
}

func (p *Player) editDetails() {
	if p.book == nil || p.EditDetails == nil {
		return
	}

	original := details.Original{
		Title:  p.book.Title(p.tags),
		Author: p.tags.Artist,
	}
	if p.foundCover != nil {
		original.Cover, _ = library.Thumbnail(p.foundCover.Data, detailsThumbnailSide)
	}
	p.EditDetails(p.book.Key(), original)
}

func (p *Player) showAbout() {
	if p.book == nil || p.ShowAbout == nil {
		return
	}

	book := about.Book{Title: p.book.Title(p.metadata), Author: p.metadata.Artist}
	if p.cover.Resource != nil {
		book.Cover, _ = library.Thumbnail(p.cover.Resource.Content(), detailsThumbnailSide)
	}
	p.ShowAbout(p.book.Key(), book)
}

func (p *Player) showCover(cover *library.Cover) {
	p.cover.Resource = nil
	p.coverName = ""
	p.setBackdropTint(nil)
	if cover != nil {
		p.cover.Resource = fyne.NewStaticResource(cover.Name, cover.Data)
		p.coverName = cover.Name
		p.tintBackdropFrom(cover)
	}
	p.cover.Refresh()

	if cover == nil {
		p.cover.Hide()
		p.coverPlaceholder.Show()
	} else {
		p.coverPlaceholder.Hide()
		p.cover.Show()
	}
}

type contentsRow struct {
	title  string
	detail string
}

func (p *Player) tintBackdropFrom(cover *library.Cover) {
	p.async(func() {
		tint, ok := library.AverageColor(cover.Data)
		if !ok {
			return
		}

		fyne.Do(func() {
			if p.coverName == cover.Name {
				p.setBackdropTint(&tint)
			}
		})
	})
}

func (p *Player) showParts() {
	if p.book == nil {
		return
	}

	if p.hasChapters() {
		rows := make([]contentsRow, len(p.chapters))
		for i, chapter := range p.chapters {
			rows[i] = contentsRow{
				title:  fmt.Sprintf("%d. %s", i+1, chapter.Title),
				detail: kit.FormatTime(chapter.Start),
			}
		}

		p.showContents("Chapters", rows, p.currentChapter(), func(id int) {
			p.seekInPart(p.chapters[id].Start)
		})
		return
	}

	rows := make([]contentsRow, len(p.book.Parts))
	for i, part := range p.book.Parts {
		rows[i].title = fmt.Sprintf("%d. %s", i+1, library.DisplayTitle(part))
		if p.book.HasDurations() {
			rows[i].detail = kit.FormatTime(p.book.Durations[i])
		}
	}

	p.showContents("Parts", rows, p.book.Index, func(id int) {
		p.goToPart(id, p.playing, nil)
	})
}

func (p *Player) showContents(title string, rows []contentsRow, current int, onSelect func(int)) {
	var contentsDialog dialog.Dialog

	list := widget.NewList(
		func() int {
			return len(rows)
		},
		func() fyne.CanvasObject {
			name := widget.NewLabel("")
			name.Truncation = fyne.TextTruncateEllipsis
			detail := widget.NewLabel("00:00:00")
			detail.Importance = widget.LowImportance
			detail.TextStyle = fyne.TextStyle{Monospace: true}
			return container.NewBorder(nil, nil, nil, detail, name)
		},
		func(id widget.ListItemID, object fyne.CanvasObject) {
			row := object.(*fyne.Container)
			name := row.Objects[0].(*widget.Label)
			detail := row.Objects[1].(*widget.Label)

			name.TextStyle.Bold = id == current
			name.SetText(rows[id].title)
			detail.SetText(rows[id].detail)
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		contentsDialog.Hide()
		onSelect(id)
	}

	contentsDialog = dialog.NewCustom(title, "Close", list, p.window)

	size := p.window.Canvas().Size()
	contentsDialog.Resize(fyne.NewSize(min(size.Width-40, 460), size.Height*0.8))
	contentsDialog.Show()
	list.ScrollTo(current)
}

func (p *Player) Title() string {
	return p.title.String()
}

func (p *Player) ShowsCover(name string) bool {
	return name != "" && p.coverName == name
}
