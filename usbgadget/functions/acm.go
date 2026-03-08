package functions

type acmFunc struct {
	instance string
}

// ACMSerial creates a USB ACM serial function.
func ACMSerial() Function { return &acmFunc{instance: "usb0"} }

func (f *acmFunc) TypeName() string        { return "acm" }
func (f *acmFunc) InstanceName() string    { return f.instance }
func (f *acmFunc) Configure(_ string) error { return nil }
