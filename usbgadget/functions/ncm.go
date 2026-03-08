package functions

type ncmFunc struct {
	instance string
}

// NCM creates an NCM network function (high-speed USB network).
func NCM() Function { return &ncmFunc{instance: "usb2"} }

func (f *ncmFunc) TypeName() string        { return "ncm" }
func (f *ncmFunc) InstanceName() string    { return f.instance }
func (f *ncmFunc) Configure(_ string) error { return nil }
