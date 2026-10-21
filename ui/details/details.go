package details

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/ncruces/zenity"

	"github.com/izzzicos/audiobook-player/audio"
	"github.com/izzzicos/audiobook-player/library"
	"github.com/izzzicos/audiobook-player/ui/kit"
	"github.com/izzzicos/audiobook-player/ui/userdata"
)

const (
	previewSide   = 160
	thumbnailSide = 320
	maxImageSize  = 50 << 20
	note          = "Clear a field to go back to what the book's files say. Your files aren't changed."
)

type Original struct {
	Title  string
	Author string
	Cover  image.Image
}

type Editor struct {
	OnSaved func(key string)

	window fyne.Window
	picker *kit.Picker
	data   *userdata.Data
	async  func(func())
}

func New(window fyne.Window, picker *kit.Picker, data *userdata.Data, async func(func())) *Editor {
	return &Editor{window: window, picker: picker, data: data, async: async}
}

type editing struct {
	editor   *Editor
	key      string
	original Original
	edit     userdata.BookEdit

	title       *widget.Entry
	author      *widget.Entry
	preview     *canvas.Image
	placeholder fyne.CanvasObject
	useOriginal *widget.Button
	save        *widget.Button
	popUp       dialog.Dialog

	chosen   []byte
	reverted bool
}

func (e *Editor) Show(key string, original Original) {
	e.open(key, original).popUp.Show()
}

func (e *Editor) open(key string, original Original) *editing {
	form := &editing{
		editor:   e,
		key:      key,
		original: original,
		edit:     e.data.Edit(key),
	}
	form.build()

	return form
}

func (f *editing) build() {
	f.title = widget.NewEntry()
	f.title.SetText(firstOf(f.edit.Title, f.original.Title))
	f.title.PlaceHolder = firstOf(f.original.Title, "Title")
	f.title.OnSubmitted = func(string) {
		f.submit()
	}

	f.author = widget.NewEntry()
	f.author.SetText(firstOf(f.edit.Author, f.original.Author))
	f.author.PlaceHolder = firstOf(f.original.Author, "Unknown author")
	f.author.OnSubmitted = func(string) {
		f.submit()
	}

	f.preview = canvas.NewImageFromImage(nil)
	f.preview.FillMode = canvas.ImageFillContain
	f.preview.CornerRadius = 12

	placeholderBackground := canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
	placeholderBackground.CornerRadius = 12
	placeholderIcon := canvas.NewImageFromResource(
		theme.NewColoredResource(theme.MediaMusicIcon(), theme.ColorNamePlaceHolder),
	)
	placeholderIcon.SetMinSize(fyne.NewSquareSize(previewSide * 0.4))
	f.placeholder = container.NewStack(placeholderBackground, container.NewCenter(placeholderIcon))

	coverBox := container.NewGridWrap(
		fyne.NewSquareSize(previewSide),
		container.NewStack(f.placeholder, f.preview),
	)

	choose := widget.NewButtonWithIcon("Choose image…", theme.FileImageIcon(), f.chooseImage)
	f.useOriginal = widget.NewButtonWithIcon("Use original", theme.ContentUndoIcon(), func() {
		f.chosen = nil
		f.reverted = true
		f.showCover()
	})

	form := widget.NewForm(
		widget.NewFormItem("Title", f.title),
		widget.NewFormItem("Author", f.author),
	)

	message := widget.NewLabel(note)
	message.Wrapping = fyne.TextWrapWord
	message.SizeName = theme.SizeNameCaptionText
	message.Importance = widget.LowImportance

	f.save = widget.NewButtonWithIcon("Save", theme.ConfirmIcon(), f.submit)
	f.save.Importance = widget.HighImportance
	cancel := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		f.popUp.Hide()
	})

	f.showCover()

	window := f.editor.window
	f.popUp = dialog.NewCustomWithoutButtons("Edit details", container.NewVBox(
		container.NewCenter(coverBox),
		container.NewCenter(container.NewHBox(choose, f.useOriginal)),
		form,
		message,
		container.NewGridWithColumns(2, cancel, f.save),
	), window)
	f.popUp.Resize(fyne.NewSize(min(window.Canvas().Size().Width-40, 460), f.popUp.MinSize().Height))
}

func (f *editing) customCover() bool {
	return f.chosen != nil || f.edit.Cover != "" && !f.reverted
}

func (f *editing) showCover() {
	cover := f.original.Cover
	if f.customCover() {
		data := f.chosen
		if data == nil {
			data = userdata.ReadCustomCover(f.editor.data.CoverFile(f.key))
		}
		if thumbnail, err := library.Thumbnail(data, thumbnailSide); err == nil {
			cover = thumbnail
		}
	}

	f.preview.Image = cover
	f.preview.Refresh()
	if cover == nil {
		f.preview.Hide()
		f.placeholder.Show()
	} else {
		f.placeholder.Hide()
		f.preview.Show()
	}

	if f.customCover() {
		f.useOriginal.Enable()
	} else {
		f.useOriginal.Disable()
	}
}

func (f *editing) chooseImage() {
	folder := f.key
	if audio.IsSupported(folder) {
		folder = filepath.Dir(folder)
	}

	kit.Browse(f.editor.picker, f.editor.window,
		func() (string, error) {
			return zenity.SelectFile(
				zenity.Title("Choose a cover image"),
				zenity.Filename(kit.WithTrailingSeparator(folder)),
				zenity.FileFilters{
					{Name: "Images (JPEG, PNG, GIF, BMP, WebP)", Patterns: imagePatterns(), CaseFold: true},
				},
			)
		},
		f.useImage,
		func() {
			f.showFileDialog(folder)
		},
	)
}

func (f *editing) showFileDialog(folder string) {
	fileDialog := dialog.NewFileOpen(
		func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, f.editor.window)
				return
			}

			if reader == nil {
				return
			}
			defer reader.Close()

			f.useImage(reader.URI().Path())
		},
		f.editor.window,
	)

	fileDialog.SetFilter(storage.NewExtensionFileFilter(imageExtensionsAnyCase()))
	kit.ShowFallbackDialog(f.editor.window, fileDialog, folder)
}

func (f *editing) useImage(path string) {
	f.editor.async(func() {
		prepared, err := readImage(path)

		fyne.Do(func() {
			if err != nil {
				dialog.ShowError(err, f.editor.window)
				return
			}

			f.chosen = prepared
			f.reverted = false
			f.showCover()
		})
	})
}

func readImage(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxImageSize {
		return nil, fmt.Errorf("%q is too large; choose an image smaller than %d MB", filepath.Base(path), maxImageSize>>20)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return library.PrepareCover(data)
}

func (f *editing) submit() {
	data := f.editor.data
	edit := f.edit
	edit.Title = correction(f.title.Text, f.original.Title)
	edit.Author = correction(f.author.Text, f.original.Author)

	switch {
	case f.chosen != nil:
		name, err := data.SaveCover(f.chosen)
		if err != nil {
			dialog.ShowError(fmt.Errorf("the cover couldn't be saved: %w", err), f.editor.window)
			return
		}
		edit.Cover = name
	case f.reverted:
		edit.Cover = ""
	}

	data.SetEdit(f.key, edit)
	f.popUp.Hide()

	if f.editor.OnSaved != nil {
		f.editor.OnSaved(f.key)
	}
}

func correction(text, original string) string {
	text = library.CleanText(text)
	if text == library.CleanText(original) {
		return ""
	}

	return text
}

func firstOf(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

func imagePatterns() []string {
	patterns := make([]string, len(library.ImageExtensions))
	for i, extension := range library.ImageExtensions {
		patterns[i] = "*" + extension
	}

	return patterns
}

func imageExtensionsAnyCase() []string {
	var extensions []string
	for _, extension := range library.ImageExtensions {
		extensions = append(extensions, extension, strings.ToUpper(extension))
	}

	return extensions
}
