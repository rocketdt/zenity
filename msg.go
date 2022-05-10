package zenity

// Question displays the question dialog.
//
// Valid options: Title, Width, Height, OKLabel, CancelLabel, ExtraButton,
// Icon, NoWrap, Ellipsize, DefaultCancel.
func Question(text string, timeout int, options ...Option) error {
	return message(questionKind, text, timeout, applyOptions(options))
}

// Info displays the info dialog.
//
// Valid options: Title, Width, Height, OKLabel, ExtraButton, Icon,
// NoWrap, Ellipsize.
func Info(text string, timeout int, options ...Option) error {
	return message(infoKind, text, timeout, applyOptions(options))
}

// Warning displays the warning dialog.
//
// Valid options: Title, Width, Height, OKLabel, ExtraButton, Icon,
// NoWrap, Ellipsize.
func Warning(text string, timeout int, options ...Option) error {
	return message(warningKind, text, timeout, applyOptions(options))
}

// Error displays the error dialog.
//
// Valid options: Title, Width, Height, OKLabel, ExtraButton, Icon,
// NoWrap, Ellipsize.
func Error(text string, timeout int, options ...Option) error {
	return message(errorKind, text, timeout, applyOptions(options))
}

type messageKind int

const (
	questionKind messageKind = iota
	infoKind
	warningKind
	errorKind
)

// NoWrap returns an Option to disable enable text wrapping (Unix only).
func NoWrap() Option {
	return funcOption(func(o *options) { o.noWrap = true })
}

// Ellipsize returns an Option to enable ellipsizing in the dialog text (Unix only).
func Ellipsize() Option {
	return funcOption(func(o *options) { o.ellipsize = true })
}

// DefaultCancel returns an Option to give the Cancel button focus by default.
func DefaultCancel() Option {
	return funcOption(func(o *options) { o.defaultCancel = true })
}
