package ducky

// LayoutFR is the AZERTY (French) keyboard layout.
// Use this when the target host has a French AZERTY layout configured.
//
// HID keycodes are physical positions (US QWERTY baseline). The host OS
// translates them to characters based on its AZERTY layout. For example:
//   - Keycode 0x14 (physical 'q' position) → 'a' on AZERTY host
//   - Keycode 0x04 (physical 'a' position) → 'q' on AZERTY host
//   - Keycode 0x1e (physical '1' position) → '&' unshifted, '1' shifted on AZERTY
var LayoutFR = &Layout{chars: map[rune]Key{
	// Lowercase letters — AZERTY row mapping
	// Physical 'a' key (0x04) = 'q' on AZERTY
	// Physical 'q' key (0x14) = 'a' on AZERTY
	// Physical 'w' key (0x1a) = 'z' on AZERTY
	// Physical 'z' key (0x1d) = 'w' on AZERTY (bottom row)
	// Physical 'm' key (0x10) = ',' on AZERTY — letter 'm' is on ';' key (0x33)
	'a': {0x14, ModNone}, 'b': {0x05, ModNone}, 'c': {0x06, ModNone},
	'd': {0x07, ModNone}, 'e': {0x08, ModNone}, 'f': {0x09, ModNone},
	'g': {0x0a, ModNone}, 'h': {0x0b, ModNone}, 'i': {0x0c, ModNone},
	'j': {0x0d, ModNone}, 'k': {0x0e, ModNone}, 'l': {0x0f, ModNone},
	'm': {0x33, ModNone}, 'n': {0x11, ModNone}, 'o': {0x12, ModNone},
	'p': {0x13, ModNone}, 'q': {0x04, ModNone}, 'r': {0x15, ModNone},
	's': {0x16, ModNone}, 't': {0x17, ModNone}, 'u': {0x18, ModNone},
	'v': {0x19, ModNone}, 'w': {0x1d, ModNone}, 'x': {0x1b, ModNone},
	'y': {0x1c, ModNone}, 'z': {0x1a, ModNone},

	// Uppercase letters (shift + same keycode)
	'A': {0x14, ModLShift}, 'B': {0x05, ModLShift}, 'C': {0x06, ModLShift},
	'D': {0x07, ModLShift}, 'E': {0x08, ModLShift}, 'F': {0x09, ModLShift},
	'G': {0x0a, ModLShift}, 'H': {0x0b, ModLShift}, 'I': {0x0c, ModLShift},
	'J': {0x0d, ModLShift}, 'K': {0x0e, ModLShift}, 'L': {0x0f, ModLShift},
	'M': {0x33, ModLShift}, 'N': {0x11, ModLShift}, 'O': {0x12, ModLShift},
	'P': {0x13, ModLShift}, 'Q': {0x04, ModLShift}, 'R': {0x15, ModLShift},
	'S': {0x16, ModLShift}, 'T': {0x17, ModLShift}, 'U': {0x18, ModLShift},
	'V': {0x19, ModLShift}, 'W': {0x1d, ModLShift}, 'X': {0x1b, ModLShift},
	'Y': {0x1c, ModLShift}, 'Z': {0x1a, ModLShift},

	// AZERTY digit row — unshifted characters (not digits!)
	// Position:  1    2    3    4    5    6    7    8    9    0    )    =
	// AZERTY:    &    é    "    '    (    -    è    _    ç    à    )    =
	'&': {0x1e, ModNone}, // unshifted 1 key
	'é': {0x1f, ModNone}, // unshifted 2 key
	'"': {0x20, ModNone}, // unshifted 3 key
	'\'': {0x21, ModNone}, // unshifted 4 key
	'(': {0x22, ModNone}, // unshifted 5 key
	'-': {0x23, ModNone}, // unshifted 6 key
	'è': {0x24, ModNone}, // unshifted 7 key
	'_': {0x25, ModNone}, // unshifted 8 key
	'ç': {0x26, ModNone}, // unshifted 9 key
	'à': {0x27, ModNone}, // unshifted 0 key

	// AZERTY digit row — shifted characters (digits + some symbols)
	'1': {0x1e, ModLShift},
	'2': {0x1f, ModLShift},
	'3': {0x20, ModLShift},
	'4': {0x21, ModLShift},
	'5': {0x22, ModLShift},
	'6': {0x23, ModLShift},
	'7': {0x24, ModLShift},
	'8': {0x25, ModLShift},
	'9': {0x26, ModLShift},
	'0': {0x27, ModLShift},

	// Common punctuation
	' ':  {KeySpace, ModNone},
	'\t': {KeyTab, ModNone},
	'\n': {KeyEnter, ModNone},
	'\r': {KeyEnter, ModNone},

	// AZERTY punctuation row — physical ; key (0x33) = 'm' (mapped above)
	// Physical , key (0x36) = ',' on AZERTY
	',': {0x10, ModNone},  // physical 'm' key = ',' on AZERTY
	';': {0x36, ModNone},  // physical ',' key = ';' on AZERTY
	':': {0x37, ModNone},  // physical '.' key = ':' on AZERTY
	'!': {0x38, ModLShift}, // physical '/' key + shift = '!' on AZERTY

	// Brackets (AltGr combinations on AZERTY — using RAlt)
	'[': {0x25, ModRAlt},  // AltGr + 5
	']': {0x2d, ModRAlt},  // AltGr + ) key
	'{': {0x21, ModRAlt},  // AltGr + 4
	'}': {0x2e, ModRAlt},  // AltGr + = key
	'|': {0x23, ModRAlt},  // AltGr + 6
	'@': {0x1f, ModRAlt},  // AltGr + 2 = @ on AZERTY
	'#': {0x20, ModRAlt},  // AltGr + 3 = # on AZERTY
	'~': {0x1d, ModRAlt},  // AltGr + 2 on some AZERTY variants
	'`': {0x24, ModRAlt},  // AltGr + 7 on some keyboards

	// Other symbols available without AltGr on AZERTY
	')': {0x2d, ModNone},  // physical '-' key = ')' on AZERTY (unshifted)
	'=': {0x2e, ModNone},  // physical '=' key = '=' on AZERTY (unshifted)
	'+': {0x2e, ModLShift}, // shifted '='
	'*': {0x31, ModNone},  // physical '\' key = '*' on AZERTY
	'/': {0x38, ModNone},  // physical '/' key = '/' on AZERTY... varies by keyboard
	'.': {0x37, ModLShift}, // physical '.' key + shift = '.' on AZERTY (unshifted is ':')
	'<': {0x64, ModNone},  // extra key between left shift and z (ISO layout)
	'>': {0x64, ModLShift},
}}
