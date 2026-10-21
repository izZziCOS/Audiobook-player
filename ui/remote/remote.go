package remote

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/ncruces/zenity"
	"github.com/zalando/go-keyring"

	"github.com/izzzicos/audiobook-player/mediaserver"
	"github.com/izzzicos/audiobook-player/tunnel"
	"github.com/izzzicos/audiobook-player/ui/kit"
	"github.com/izzzicos/audiobook-player/vfs"
)

const (
	preferenceSettings = "mediaServer"
	secretService      = "Audiobook Player"
	passwordKey        = "media-server-password"
	wireGuardKey       = "media-server-wireguard"
	mountName          = "Media server"
	fileExtension      = ".audiobookserver"
	fileVersion        = 1
)

type Secrets interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
}

type Keychain struct{}

func (Keychain) Get(key string) (string, error) {
	value, err := keyring.Get(secretService, key)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return value, err
}

func (Keychain) Set(key, value string) error {
	return keyring.Set(secretService, key, value)
}

func (Keychain) Delete(key string) error {
	err := keyring.Delete(secretService, key)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

type connectionFile struct {
	Version   int                  `json:"version"`
	Server    mediaserver.Settings `json:"server"`
	WireGuard string               `json:"wireguard,omitempty"`
}

type Connection struct {
	OnConnected  func()
	UseAsLibrary func(path string)
	SyncThrough  func(path string)

	window  fyne.Window
	prefs   fyne.Preferences
	picker  *kit.Picker
	secrets Secrets
	root    string
	onMain  func(func())

	server    *mediaserver.Server
	settings  mediaserver.Settings
	password  string
	wireGuard string
	connected bool

	section      fyne.CanvasObject
	status       *widget.Label
	address      *widget.Entry
	share        *widget.Entry
	folder       *widget.Entry
	username     *widget.Entry
	passwordBox  *widget.Entry
	tunnelLabel  *widget.Label
	removeTunnel *widget.Button
	connect      *widget.Button
	forget       *widget.Button
	useLibrary   *widget.Button
	useSync      *widget.Button
}

func New(window fyne.Window, prefs fyne.Preferences, picker *kit.Picker, secrets Secrets, dataFolder string) *Connection {
	c := &Connection{
		window:  window,
		prefs:   prefs,
		picker:  picker,
		secrets: secrets,
		root:    filepath.Join(dataFolder, mountName),
		onMain:  fyne.Do,
	}
	c.build()

	return c
}

func (c *Connection) Root() string {
	return c.root
}

func (c *Connection) Section() fyne.CanvasObject {
	return c.section
}

func (c *Connection) Configured() bool {
	return c.settings.Address != "" && c.settings.Share != ""
}

func (c *Connection) build() {
	c.status = widget.NewLabel("")
	c.status.Wrapping = fyne.TextWrapWord

	c.address = widget.NewEntry()
	c.address.PlaceHolder = "For example 192.168.1.10"
	c.share = widget.NewEntry()
	c.share.PlaceHolder = "For example audiobooks"
	c.folder = widget.NewEntry()
	c.folder.PlaceHolder = "Optional, for example Books"
	c.username = widget.NewEntry()
	c.passwordBox = widget.NewPasswordEntry()

	c.tunnelLabel = widget.NewLabel("")
	c.tunnelLabel.Wrapping = fyne.TextWrapWord
	importTunnel := widget.NewButtonWithIcon("Import WireGuard file…", theme.FileIcon(), c.chooseWireGuard)
	c.removeTunnel = widget.NewButtonWithIcon("Remove", theme.ContentClearIcon(), func() {
		c.wireGuard = ""
		c.showTunnel()
	})
	c.removeTunnel.Importance = widget.LowImportance

	form := widget.NewForm(
		widget.NewFormItem("Server address", c.address),
		widget.NewFormItem("Shared folder", c.share),
		widget.NewFormItem("Folder inside it", c.folder),
		widget.NewFormItem("Username", c.username),
		widget.NewFormItem("Password", c.passwordBox),
	)

	c.connect = widget.NewButtonWithIcon("Connect", theme.LoginIcon(), c.save)
	c.connect.Importance = widget.HighImportance
	c.forget = widget.NewButtonWithIcon("Disconnect and forget", theme.LogoutIcon(), c.confirmForget)
	c.forget.Importance = widget.LowImportance
	c.useLibrary = widget.NewButtonWithIcon("Use as library folder", theme.FolderOpenIcon(), func() {
		if c.UseAsLibrary != nil {
			c.UseAsLibrary(c.root)
		}
	})
	c.useSync = widget.NewButtonWithIcon("Sync through it", theme.ViewRefreshIcon(), func() {
		if c.SyncThrough != nil {
			c.SyncThrough(c.root)
		}
	})

	importFile := widget.NewButtonWithIcon("Import connection file…", theme.DownloadIcon(), c.chooseConnectionFile)
	importFile.Importance = widget.LowImportance
	exportFile := widget.NewButtonWithIcon("Export connection file…", theme.UploadIcon(), c.export)
	exportFile.Importance = widget.LowImportance

	c.showTunnel()
	c.showStatus(mediaserver.Status{})

	c.section = kit.Section(
		"Media server",
		"Connect straight to the shared folder on your media server, at home or away through WireGuard, "+
			"without setting anything up in Finder, File Explorer or the WireGuard app. "+
			"The password and WireGuard key are kept in this computer's keychain.",
		c.status,
		form,
		container.NewBorder(nil, nil, widget.NewLabel("WireGuard"), container.NewHBox(importTunnel, c.removeTunnel), c.tunnelLabel),
		container.NewHBox(c.connect, c.forget),
		container.NewHBox(c.useLibrary, c.useSync),
		container.NewHBox(importFile, exportFile),
	)
}

func (c *Connection) showTunnel() {
	if c.wireGuard == "" {
		c.tunnelLabel.SetText("Not set up. Only needed to reach the server away from home.")
		c.removeTunnel.Hide()
		return
	}

	config, err := tunnel.Parse(c.wireGuard)
	if err != nil {
		c.tunnelLabel.SetText("The WireGuard settings can't be used: " + err.Error())
	} else {
		c.tunnelLabel.SetText(fmt.Sprintf("Set up as %s. Used when the server can't be reached directly.", config.Addresses[0]))
	}
	c.removeTunnel.Show()
}

func (c *Connection) showStatus(status mediaserver.Status) {
	wasConnected := c.connected
	c.connected = status.State == mediaserver.Connected

	switch {
	case !c.Configured():
		c.status.SetText("Not set up.")
	case status.State == mediaserver.Connected:
		c.status.SetText("Connected " + status.Route + ".")
	case status.Problem != "":
		c.status.SetText(status.Problem + " Trying again…")
	default:
		c.status.SetText("Connecting…")
	}

	if c.Configured() {
		c.forget.Show()
	} else {
		c.forget.Hide()
	}
	if c.connected {
		c.useLibrary.Enable()
		c.useSync.Enable()
	} else {
		c.useLibrary.Disable()
		c.useSync.Disable()
	}

	if c.connected && !wasConnected && c.OnConnected != nil {
		c.OnConnected()
	}
}

func (c *Connection) Load() {
	var settings mediaserver.Settings
	if raw := c.prefs.String(preferenceSettings); raw != "" {
		json.Unmarshal([]byte(raw), &settings)
	}
	c.settings = settings
	c.password, _ = c.secrets.Get(passwordKey)
	c.wireGuard, _ = c.secrets.Get(wireGuardKey)
	c.fill()
	c.start()
}

func (c *Connection) fill() {
	c.address.SetText(c.settings.Address)
	c.share.SetText(c.settings.Share)
	c.folder.SetText(c.settings.Folder)
	c.username.SetText(c.settings.Username)
	c.passwordBox.SetText(c.password)
	c.showTunnel()
}

func (c *Connection) start() {
	c.stop()
	if !c.Configured() {
		vfs.Unmount(c.root)
		c.showStatus(mediaserver.Status{})
		return
	}

	var config *tunnel.Config
	if c.wireGuard != "" {
		if parsed, err := tunnel.Parse(c.wireGuard); err == nil {
			config = &parsed
		}
	}

	server := mediaserver.New(c.settings, c.password, config)
	server.OnChange = func() {
		status := server.Status()
		c.onMain(func() {
			if c.server == server {
				c.showStatus(status)
			}
		})
	}
	c.server = server
	vfs.Mount(c.root, server)
	server.Start()
	c.showStatus(server.Status())
}

func (c *Connection) stop() {
	if c.server == nil {
		return
	}

	server := c.server
	c.server = nil
	go server.Close()
}

func (c *Connection) Close() {
	if c.server != nil {
		c.server.Close()
		c.server = nil
	}
}

func (c *Connection) save() {
	settings := mediaserver.Settings{
		Address:  strings.TrimSpace(c.address.Text),
		Share:    strings.Trim(strings.TrimSpace(c.share.Text), `/\`),
		Folder:   strings.Trim(strings.TrimSpace(c.folder.Text), `/\`),
		Username: strings.TrimSpace(c.username.Text),
	}
	if settings.Address == "" || settings.Share == "" {
		dialog.ShowError(errors.New("enter the server's address and the name of its shared folder"), c.window)
		return
	}
	if c.wireGuard != "" {
		if _, err := tunnel.Parse(c.wireGuard); err != nil {
			dialog.ShowError(err, c.window)
			return
		}
	}

	c.settings = settings
	c.password = c.passwordBox.Text
	encoded, _ := json.Marshal(settings)
	c.prefs.SetString(preferenceSettings, string(encoded))

	var problems []error
	if err := c.storeSecret(passwordKey, c.password); err != nil {
		problems = append(problems, err)
	}
	if err := c.storeSecret(wireGuardKey, c.wireGuard); err != nil {
		problems = append(problems, err)
	}
	if len(problems) > 0 {
		dialog.ShowError(fmt.Errorf("the password or WireGuard key couldn't be kept in this computer's keychain, so you'll need to enter them again next time: %w", errors.Join(problems...)), c.window)
	}

	c.start()
}

func (c *Connection) storeSecret(key, value string) error {
	if value == "" {
		return c.secrets.Delete(key)
	}

	return c.secrets.Set(key, value)
}

func (c *Connection) confirmForget() {
	dialog.ShowConfirm(
		"Disconnect from the media server?",
		"The server's details, password and WireGuard key are removed from this computer. Books from the server show Not connected until you connect again; downloaded books keep playing.",
		func(confirmed bool) {
			if confirmed {
				c.Forget()
			}
		},
		c.window,
	)
}

func (c *Connection) Forget() {
	c.settings = mediaserver.Settings{}
	c.password, c.wireGuard = "", ""
	c.prefs.RemoveValue(preferenceSettings)
	c.secrets.Delete(passwordKey)
	c.secrets.Delete(wireGuardKey)
	c.fill()
	c.start()
}

func (c *Connection) chooseWireGuard() {
	c.chooseFile("Choose your WireGuard file", "WireGuard settings", []string{"*.conf"}, func(data []byte) {
		if _, err := tunnel.Parse(string(data)); err != nil {
			dialog.ShowError(err, c.window)
			return
		}
		c.wireGuard = string(data)
		c.showTunnel()
	})
}

func (c *Connection) chooseConnectionFile() {
	c.chooseFile("Choose a connection file", "Audiobook Player connection", []string{"*" + fileExtension}, func(data []byte) {
		if err := c.Import(data); err != nil {
			dialog.ShowError(err, c.window)
		}
	})
}

func (c *Connection) Import(data []byte) error {
	var file connectionFile
	if err := json.Unmarshal(data, &file); err != nil || file.Version != fileVersion || file.Server.Address == "" {
		return errors.New("this isn't an Audiobook Player connection file")
	}
	if file.WireGuard != "" {
		if _, err := tunnel.Parse(file.WireGuard); err != nil {
			return err
		}
	}

	c.address.SetText(file.Server.Address)
	c.share.SetText(file.Server.Share)
	c.folder.SetText(file.Server.Folder)
	c.username.SetText(file.Server.Username)
	c.passwordBox.SetText("")
	c.wireGuard = file.WireGuard
	c.showTunnel()
	c.window.Canvas().Focus(c.passwordBox)

	return nil
}

func (c *Connection) Export() ([]byte, error) {
	if !c.Configured() {
		return nil, errors.New("connect to the media server first")
	}

	return json.MarshalIndent(connectionFile{Version: fileVersion, Server: c.settings, WireGuard: c.wireGuard}, "", "  ")
}

func (c *Connection) export() {
	data, err := c.Export()
	if err != nil {
		dialog.ShowError(err, c.window)
		return
	}

	save := func(path string) {
		if filepath.Ext(path) != fileExtension {
			path += fileExtension
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			dialog.ShowError(err, c.window)
			return
		}
		dialog.ShowInformation("Connection file saved",
			"Open it in Audiobook Player on another computer and enter the password. It contains your WireGuard key, so keep it private.",
			c.window)
	}

	kit.Browse(c.picker, c.window,
		func() (string, error) {
			return zenity.SelectFileSave(
				zenity.Title("Save the connection file"),
				zenity.Filename("Media server"+fileExtension),
				zenity.ConfirmOverwrite(),
			)
		},
		save,
		func() {
			saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
				if err != nil || writer == nil {
					return
				}
				writer.Close()
				save(writer.URI().Path())
			}, c.window)
			saveDialog.SetFileName("Media server" + fileExtension)
			kit.ShowFallbackDialog(c.window, saveDialog, kit.HomeFolder())
		},
	)
}

func (c *Connection) chooseFile(title, kind string, patterns []string, chosen func([]byte)) {
	read := func(path string) {
		data, err := os.ReadFile(path)
		if err != nil {
			dialog.ShowError(err, c.window)
			return
		}
		chosen(data)
	}

	kit.Browse(c.picker, c.window,
		func() (string, error) {
			return zenity.SelectFile(
				zenity.Title(title),
				zenity.Filename(kit.WithTrailingSeparator(kit.HomeFolder())),
				zenity.FileFilters{{Name: kind, Patterns: patterns, CaseFold: true}},
			)
		},
		read,
		func() {
			openDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
				if err != nil || reader == nil {
					return
				}
				reader.Close()
				read(reader.URI().Path())
			}, c.window)
			var extensions []string
			for _, pattern := range patterns {
				extensions = append(extensions, strings.TrimPrefix(pattern, "*"))
			}
			openDialog.SetFilter(storage.NewExtensionFileFilter(extensions))
			kit.ShowFallbackDialog(c.window, openDialog, kit.HomeFolder())
		},
	)
}

type Memory map[string]string

func (m Memory) Get(key string) (string, error) {
	return m[key], nil
}

func (m Memory) Set(key, value string) error {
	m[key] = value
	return nil
}

func (m Memory) Delete(key string) error {
	delete(m, key)
	return nil
}

func (c *Connection) Fill(address, share, folder, username, password string) {
	c.address.SetText(address)
	c.share.SetText(share)
	c.folder.SetText(folder)
	c.username.SetText(username)
	c.passwordBox.SetText(password)
}

func (c *Connection) Save() {
	c.save()
}
