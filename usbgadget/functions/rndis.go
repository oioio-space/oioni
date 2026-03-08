package functions

type rndisFunc struct {
	instance string
}

// RNDIS creates a RNDIS network function (Windows USB network).
// Must be the first function in the composite for Windows compatibility.
func RNDIS() Function { return &rndisFunc{instance: "usb0"} }

func (f *rndisFunc) TypeName() string        { return "rndis" }
func (f *rndisFunc) InstanceName() string    { return f.instance }
func (f *rndisFunc) Configure(_ string) error { return nil }
