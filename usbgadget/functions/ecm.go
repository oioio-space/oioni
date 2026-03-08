package functions

type ecmFunc struct {
	instance string
}

// ECM creates an ECM network function (Linux/macOS USB network).
func ECM() Function { return &ecmFunc{instance: "usb1"} }

func (f *ecmFunc) TypeName() string        { return "ecm" }
func (f *ecmFunc) InstanceName() string    { return f.instance }
func (f *ecmFunc) Configure(_ string) error { return nil }
