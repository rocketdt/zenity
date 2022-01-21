// +build windows darwin dev

package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/rocketdt/zenity"
	"github.com/rocketdt/zenity/internal/zenutil"
)

const (
	unspecified = "\x00"
)

var tag = "v0.0.0"

var (
	// Application Options
	errorDlg          bool
	infoDlg           bool
	warningDlg        bool
	questionDlg       bool
	entryDlg          bool
	listDlg           bool
	passwordDlg       bool
	fileSelectionDlg  bool
	colorSelectionDlg bool
	progressDlg       bool
	notification      bool

	// General options
	title         string
	width         uint
	height        uint
	okLabel       string
	cancelLabel   string
	extraButton   string
	text          string
	icon          string
	multiple      bool
	defaultCancel bool

	// Message options
	noWrap    bool
	ellipsize bool

	// Entry options
	entryText string
	hideText  bool

	// List options
	columns    int
	allowEmpty bool

	// File selection options
	save             bool
	directory        bool
	confirmOverwrite bool
	confirmCreate    bool
	showHidden       bool
	filename         string
	fileFilters      zenity.FileFilters

	// Color selection options
	defaultColor string
	showPalette  bool

	// Progress options
	percentage float64
	pulsate    bool
	autoClose  bool
	autoKill   bool
	noCancel   bool

	// Notify options
	listen bool

	// Windows specific options
	unixeol bool
	cygpath bool
	wslpath bool

	// Command options
	version bool
)

func init() {
	prevUsage := flag.Usage
	flag.Usage = func() {
		prevUsage()
		os.Exit(-1)
	}
}

func main() {
	setupFlags()
	flag.Parse()
	validateFlags()
	opts := loadFlags()
	zenutil.Command = true
	if unixeol {
		zenutil.LineBreak = "\n"
	}
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if zenutil.Timeout > 0 {
		c, cancel := context.WithTimeout(ctx, time.Duration(zenutil.Timeout)*time.Second)
		defer cancel()
		ctx = c
	}
	opts = append(opts, zenity.Context(ctx))

	switch {
	case errorDlg:
		errResult(zenity.Error(text, opts...))
	case infoDlg:
		errResult(zenity.Info(text, opts...))
	case warningDlg:
		errResult(zenity.Warning(text, opts...))
	case questionDlg:
		errResult(zenity.Question(text, opts...))

	case entryDlg:
		strResult(zenity.Entry(text, opts...))

	case listDlg:
		if multiple {
			lstResult(zenity.ListMultiple(text, flag.Args(), opts...))
		} else {
			strResult(zenity.List(text, flag.Args(), opts...))
		}

	case passwordDlg:
		_, pw, err := zenity.Password(opts...)
		strResult(pw, err)

	case fileSelectionDlg:
		switch {
		default:
			strResult(egestPath(zenity.SelectFile(opts...)))
		case save:
			strResult(egestPath(zenity.SelectFileSave(opts...)))
		case multiple:
			lstResult(egestPaths(zenity.SelectFileMutiple(opts...)))
		}

	case colorSelectionDlg:
		colResult(zenity.SelectColor(opts...))

	case progressDlg:
		errResult(progress(opts...))

	case notification:
		errResult(notify(opts...))

	default:
		flag.Usage()
	}
}

func setupFlags() {
	// Application Options
	flag.BoolVar(&errorDlg, "error", false, "Display error dialog")
	flag.BoolVar(&infoDlg, "info", false, "Display info dialog")
	flag.BoolVar(&warningDlg, "warning", false, "Display warning dialog")
	flag.BoolVar(&questionDlg, "question", false, "Display question dialog")
	flag.BoolVar(&entryDlg, "entry", false, "Display text entry dialog")
	flag.BoolVar(&listDlg, "list", false, "Display list dialog")
	flag.BoolVar(&passwordDlg, "password", false, "Display password dialog")
	flag.BoolVar(&fileSelectionDlg, "file-selection", false, "Display file selection dialog")
	flag.BoolVar(&colorSelectionDlg, "color-selection", false, "Display color selection dialog")
	flag.BoolVar(&progressDlg, "progress", false, "Display progress indication dialog")
	flag.BoolVar(&notification, "notification", false, "Display notification")

	// General options
	flag.StringVar(&title, "title", "", "Set the dialog `title`")
	flag.UintVar(&width, "width", 0, "Set the `width`")
	flag.UintVar(&height, "height", 0, "Set the `height`")
	flag.StringVar(&okLabel, "ok-label", "", "Set the label of the OK button")
	flag.StringVar(&cancelLabel, "cancel-label", "", "Set the label of the Cancel button")
	flag.StringVar(&extraButton, "extra-button", "", "Add an extra button")
	flag.StringVar(&text, "text", "", "Set the dialog `text`")
	flag.StringVar(&icon, "window-icon", "", "Set the window `icon` (error, info, question, warning)")
	flag.BoolVar(&multiple, "multiple", false, "Allow multiple items to be selected")
	flag.BoolVar(&defaultCancel, "default-cancel", false, "Give Cancel button focus by default")

	// Message options
	flag.StringVar(&icon, "icon-name", "", "Set the dialog `icon` (dialog-error, dialog-information, dialog-question, dialog-warning)")
	flag.BoolVar(&noWrap, "no-wrap", false, "Do not enable text wrapping")
	flag.BoolVar(&ellipsize, "ellipsize", false, "Enable ellipsizing in the dialog text")
	flag.Bool("no-markup", true, "Do not enable Pango markup")

	// Entry options
	flag.StringVar(&entryText, "entry-text", "", "Set the entry `text`")
	flag.BoolVar(&hideText, "hide-text", false, "Hide the entry text")

	// List options
	flag.Func("column", "Set the column header", addColumn)
	flag.Bool("hide-header", true, "Hide the column headers")
	flag.BoolVar(&allowEmpty, "allow-empty", true, "Allow empty selection (macOS only)")

	// File selection options
	flag.BoolVar(&save, "save", false, "Activate save mode")
	flag.BoolVar(&directory, "directory", false, "Activate directory-only selection")
	flag.BoolVar(&confirmOverwrite, "confirm-overwrite", false, "Confirm file selection if filename already exists")
	flag.BoolVar(&confirmCreate, "confirm-create", false, "Confirm file selection if filename does not yet exist (Windows only)")
	flag.BoolVar(&showHidden, "show-hidden", false, "Show hidden files (Windows and macOS only)")
	flag.StringVar(&filename, "filename", "", "Set the `filename`")
	flag.Func("file-filter", "Set a filename filter (NAME | PATTERN1 PATTERN2 ...)", addFileFilter)

	// Color selection options
	flag.StringVar(&defaultColor, "color", "", "Set the `color`")
	flag.BoolVar(&showPalette, "show-palette", false, "Show the palette")

	// Progress options
	flag.Float64Var(&percentage, "percentage", 0, "Set initial `percentage`")
	flag.BoolVar(&pulsate, "pulsate", false, "Pulsate progress bar")
	flag.BoolVar(&noCancel, "no-cancel", false, "Hide Cancel button (Windows and Unix only)")
	flag.BoolVar(&autoClose, "auto-close", false, "Dismiss the dialog when 100% has been reached")
	flag.BoolVar(&autoKill, "auto-kill", false, "Kill parent process if Cancel button is pressed (macOS and Unix only)")

	// Notify options
	flag.BoolVar(&listen, "listen", false, "Listen for commands on stdin")

	// Windows specific options
	if runtime.GOOS == "windows" {
		flag.BoolVar(&unixeol, "unixeol", false, "Use Unix line endings (Windows only)")
		flag.BoolVar(&cygpath, "cygpath", false, "Use cygpath for path translation (Windows only)")
		flag.BoolVar(&wslpath, "wslpath", false, "Use wslpath for path translation (Windows only)")
	}

	// Command options
	flag.BoolVar(&version, "version", false, "Show version of program")
	flag.IntVar(&zenutil.Timeout, "timeout", 0, "Set dialog `timeout` in seconds")
	flag.StringVar(&zenutil.Separator, "separator", "|", "Set output `separator` character")

	// Detect unspecified values
	title = unspecified
	okLabel = unspecified
	cancelLabel = unspecified
	extraButton = unspecified
	text = unspecified
	icon = unspecified
}

func validateFlags() {
	var n int
	if version {
		fmt.Printf("zenity %s %s/%s\n", tag, runtime.GOOS, runtime.GOARCH)
		fmt.Println("https://github.com/ncruces/zenity")
		os.Exit(0)
	}
	if errorDlg {
		n++
	}
	if infoDlg {
		n++
	}
	if warningDlg {
		n++
	}
	if questionDlg {
		n++
	}
	if entryDlg {
		n++
	}
	if listDlg {
		n++
	}
	if passwordDlg {
		n++
	}
	if fileSelectionDlg {
		n++
	}
	if colorSelectionDlg {
		n++
	}
	if progressDlg {
		n++
	}
	if notification {
		n++
	}
	if n != 1 {
		flag.Usage()
	}
}

func loadFlags() []zenity.Option {
	var opts []zenity.Option

	// Defaults

	setDefault := func(s *string, val string) {
		if *s == unspecified {
			*s = val
		}
	}
	switch {
	case errorDlg:
		setDefault(&title, "Error")
		setDefault(&icon, "dialog-error")
		setDefault(&text, "An error has occurred.")
		setDefault(&okLabel, "OK")
	case infoDlg:
		setDefault(&title, "Information")
		setDefault(&icon, "dialog-information")
		setDefault(&text, "All updates are complete.")
		setDefault(&okLabel, "OK")
	case warningDlg:
		setDefault(&title, "Warning")
		setDefault(&icon, "dialog-warning")
		setDefault(&text, "Are you sure you want to proceed?")
		setDefault(&okLabel, "OK")
	case questionDlg:
		setDefault(&title, "Question")
		setDefault(&icon, "dialog-question")
		setDefault(&text, "Are you sure you want to proceed?")
		setDefault(&okLabel, "Yes")
		setDefault(&cancelLabel, "No")
	case entryDlg:
		setDefault(&title, "Add a new entry")
		setDefault(&text, "Enter new text:")
		setDefault(&okLabel, "OK")
		setDefault(&cancelLabel, "Cancel")
	case listDlg:
		setDefault(&title, "Select items from the list")
		setDefault(&text, "Select items from the list below:")
		setDefault(&okLabel, "OK")
		setDefault(&cancelLabel, "Cancel")
	case passwordDlg:
		setDefault(&title, "Type your password")
		setDefault(&icon, "dialog-password")
		setDefault(&okLabel, "OK")
		setDefault(&cancelLabel, "Cancel")
	case progressDlg:
		setDefault(&title, "Progress")
		setDefault(&text, "Running...")
		setDefault(&okLabel, "OK")
		setDefault(&cancelLabel, "Cancel")
	case notification:
		setDefault(&icon, "dialog-information")
	default:
		setDefault(&text, "")
	}

	// General options

	if title != unspecified {
		opts = append(opts, zenity.Title(title))
	}
	opts = append(opts, zenity.Width(width))
	opts = append(opts, zenity.Height(height))
	if okLabel != unspecified {
		opts = append(opts, zenity.OKLabel(okLabel))
	}
	if cancelLabel != unspecified {
		opts = append(opts, zenity.CancelLabel(cancelLabel))
	}
	if extraButton != unspecified {
		opts = append(opts, zenity.ExtraButton(extraButton))
	}

	var ico zenity.DialogIcon
	switch icon {
	case "error", "dialog-error":
		ico = zenity.ErrorIcon
	case "info", "dialog-information":
		ico = zenity.InfoIcon
	case "question", "dialog-question":
		ico = zenity.QuestionIcon
	case "important", "warning", "dialog-warning":
		ico = zenity.WarningIcon
	case "dialog-password":
		ico = zenity.PasswordIcon
	case "":
		ico = zenity.NoIcon
	}
	opts = append(opts, ico)

	// Message options

	if noWrap {
		opts = append(opts, zenity.NoWrap())
	}
	if ellipsize {
		opts = append(opts, zenity.Ellipsize())
	}
	if defaultCancel {
		opts = append(opts, zenity.DefaultCancel())
	}

	// Entry options

	opts = append(opts, zenity.EntryText(entryText))
	if hideText {
		opts = append(opts, zenity.HideText())
	}

	// List options

	if !allowEmpty {
		opts = append(opts, zenity.DisallowEmpty())
	}

	// File selection options

	if directory {
		opts = append(opts, zenity.Directory())
	}
	if confirmOverwrite {
		opts = append(opts, zenity.ConfirmOverwrite())
	}
	if confirmCreate {
		opts = append(opts, zenity.ConfirmCreate())
	}
	if showHidden {
		opts = append(opts, zenity.ShowHidden())
	}
	if filename != "" {
		opts = append(opts, zenity.Filename(ingestPath(filename)))
	}
	opts = append(opts, fileFilters)

	// Color selection options

	if defaultColor != "" {
		opts = append(opts, zenity.Color(zenutil.ParseColor(defaultColor)))
	}
	if showPalette {
		opts = append(opts, zenity.ShowPalette())
	}

	// Progress options

	if pulsate {
		opts = append(opts, zenity.Pulsate())
	}
	if noCancel {
		opts = append(opts, zenity.NoCancel())
	}

	return opts
}

func errResult(err error) {
	if os.IsTimeout(err) {
		os.Exit(5)
	}
	if err == zenity.ErrCanceled || err == context.Canceled {
		os.Exit(1)
	}
	if err == zenity.ErrExtraButton {
		os.Stdout.WriteString(extraButton)
		os.Stdout.WriteString(zenutil.LineBreak)
		os.Exit(1)
	}
	if err != nil {
		os.Stderr.WriteString(err.Error())
		os.Stderr.WriteString(zenutil.LineBreak)
		os.Exit(-1)
	}
}

func strResult(s string, err error) {
	errResult(err)
	os.Stdout.WriteString(s)
	os.Stdout.WriteString(zenutil.LineBreak)
}

func lstResult(l []string, err error) {
	errResult(err)
	os.Stdout.WriteString(strings.Join(l, zenutil.Separator))
	os.Stdout.WriteString(zenutil.LineBreak)
}

func colResult(c color.Color, err error) {
	errResult(err)
	os.Stdout.WriteString(zenutil.UnparseColor(c))
	os.Stdout.WriteString(zenutil.LineBreak)
}

func ingestPath(path string) string {
	if runtime.GOOS == "windows" && path != "" {
		var args []string
		switch {
		case wslpath:
			args = []string{"wsl", "wslpath", "-m"}
		case cygpath:
			args = []string{"cygpath", "-C", "UTF8", "-m"}
		}
		if args != nil {
			args = append(args, path)
			out, err := exec.Command(args[0], args[1:]...).Output()
			if err == nil {
				path = string(bytes.TrimSuffix(out, []byte{'\n'}))
			}
		}
	}
	return path
}

func egestPath(path string, err error) (string, error) {
	if runtime.GOOS == "windows" && path != "" && err == nil {
		var args []string
		switch {
		case wslpath:
			args = []string{"wsl", "wslpath", "-u"}
		case cygpath:
			args = []string{"cygpath", "-C", "UTF8", "-u"}
		}
		if args != nil {
			var out []byte
			args = append(args, filepath.ToSlash(path))
			out, err = exec.Command(args[0], args[1:]...).Output()
			if err == nil {
				path = string(bytes.TrimSuffix(out, []byte{'\n'}))
			}
		}
	}
	return path, err
}

func egestPaths(paths []string, err error) ([]string, error) {
	if runtime.GOOS == "windows" && err == nil && (wslpath || cygpath) {
		paths = append(paths[:0:0], paths...)
		for i, p := range paths {
			paths[i], err = egestPath(p, nil)
			if err != nil {
				break
			}
		}
	}
	return paths, err
}

func addColumn(s string) error {
	columns++
	if columns <= 1 {
		return nil
	}
	return errors.New("multiple columns not supported")
}

func addFileFilter(s string) error {
	var filter zenity.FileFilter

	if split := strings.SplitN(s, "|", 2); len(split) > 1 {
		filter.Name = strings.TrimSpace(split[0])
		s = split[1]
	}

	filter.Patterns = strings.Split(strings.TrimSpace(s), " ")
	fileFilters = append(fileFilters, filter)

	return nil
}
