package ducky

// LayoutEN is the QWERTY (US English) keyboard layout.
// Use this when the target host has a US/UK QWERTY layout configured.
//
// HID keycodes represent physical key positions (US QWERTY baseline).
// Pressing keycode 0x04 on a QWERTY host produces 'a'.
var LayoutEN = &Layout{chars: map[rune]Key{
	// Lowercase letters
	'a': {0x04, ModNone}, 'b': {0x05, ModNone}, 'c': {0x06, ModNone},
	'd': {0x07, ModNone}, 'e': {0x08, ModNone}, 'f': {0x09, ModNone},
	'g': {0x0a, ModNone}, 'h': {0x0b, ModNone}, 'i': {0x0c, ModNone},
	'j': {0x0d, ModNone}, 'k': {0x0e, ModNone}, 'l': {0x0f, ModNone},
	'm': {0x10, ModNone}, 'n': {0x11, ModNone}, 'o': {0x12, ModNone},
	'p': {0x13, ModNone}, 'q': {0x14, ModNone}, 'r': {0x15, ModNone},
	's': {0x16, ModNone}, 't': {0x17, ModNone}, 'u': {0x18, ModNone},
	'v': {0x19, ModNone}, 'w': {0x1a, ModNone}, 'x': {0x1b, ModNone},
	'y': {0x1c, ModNone}, 'z': {0x1d, ModNone},

	// Uppercase letters (shift + same keycode)
	'A': {0x04, ModLShift}, 'B': {0x05, ModLShift}, 'C': {0x06, ModLShift},
	'D': {0x07, ModLShift}, 'E': {0x08, ModLShift}, 'F': {0x09, ModLShift},
	'G': {0x0a, ModLShift}, 'H': {0x0b, ModLShift}, 'I': {0x0c, ModLShift},
	'J': {0x0d, ModLShift}, 'K': {0x0e, ModLShift}, 'L': {0x0f, ModLShift},
	'M': {0x10, ModLShift}, 'N': {0x11, ModLShift}, 'O': {0x12, ModLShift},
	'P': {0x13, ModLShift}, 'Q': {0x14, ModLShift}, 'R': {0x15, ModLShift},
	'S': {0x16, ModLShift}, 'T': {0x17, ModLShift}, 'U': {0x18, ModLShift},
	'V': {0x19, ModLShift}, 'W': {0x1a, ModLShift}, 'X': {0x1b, ModLShift},
	'Y': {0x1c, ModLShift}, 'Z': {0x1d, ModLShift},

	// Digits (unshifted)
	'1': {0x1e, ModNone}, '2': {0x1f, ModNone}, '3': {0x20, ModNone},
	'4': {0x21, ModNone}, '5': {0x22, ModNone}, '6': {0x23, ModNone},
	'7': {0x24, ModNone}, '8': {0x25, ModNone}, '9': {0x26, ModNone},
	'0': {0x27, ModNone},

	// Symbols on digit row (shifted)
	'!': {0x1e, ModLShift}, '@': {0x1f, ModLShift}, '#': {0x20, ModLShift},
	'$': {0x21, ModLShift}, '%': {0x22, ModLShift}, '^': {0x23, ModLShift},
	'&': {0x24, ModLShift}, '*': {0x25, ModLShift}, '(': {0x26, ModLShift},
	')': {0x27, ModLShift},

	// Punctuation and symbols
	' ':  {KeySpace, ModNone},
	'\t': {KeyTab, ModNone},
	'\n': {KeyEnter, ModNone},
	'\r': {KeyEnter, ModNone},

	'-': {0x2d, ModNone}, '_': {0x2d, ModLShift},
	'=': {0x2e, ModNone}, '+': {0x2e, ModLShift},
	'[': {0x2f, ModNone}, '{': {0x2f, ModLShift},
	']': {0x30, ModNone}, '}': {0x30, ModLShift},
	'\\': {0x31, ModNone}, '|': {0x31, ModLShift},
	';': {0x33, ModNone}, ':': {0x33, ModLShift},
	'\'': {0x34, ModNone}, '"': {0x34, ModLShift},
	'`': {0x35, ModNone}, '~': {0x35, ModLShift},
	',': {0x36, ModNone}, '<': {0x36, ModLShift},
	'.': {0x37, ModNone}, '>': {0x37, ModLShift},
	'/': {0x38, ModNone}, '?': {0x38, ModLShift},
}}
