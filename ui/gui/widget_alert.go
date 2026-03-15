package gui

// AlertButton is a button shown in a ShowAlert dialog.
type AlertButton struct {
	Label   string
	OnPress func()
}
