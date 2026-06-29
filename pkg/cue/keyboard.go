package cue

// KeyMap maps human-readable key names to terminal escape sequences.
var KeyMap = map[string][]byte{
    "Enter":     {0x0D},
    "Return":    {0x0D},
    "Tab":       {0x09},
    "Backspace": {0x7F},
    "Escape":    {0x1B},
    "Space":     {0x20},
    "Up":        {0x1B, 0x5B, 0x41},
    "Down":      {0x1B, 0x5B, 0x42},
    "Right":     {0x1B, 0x5B, 0x43},
    "Left":      {0x1B, 0x5B, 0x44},
    "Home":      {0x1B, 0x5B, 0x48},
    "End":       {0x1B, 0x5B, 0x46},
    "PageUp":    {0x1B, 0x5B, 0x35, 0x7E},
    "PageDown":  {0x1B, 0x5B, 0x36, 0x7E},
    "Delete":    {0x1B, 0x5B, 0x33, 0x7E},
    "Insert":    {0x1B, 0x5B, 0x32, 0x7E},
    "F1":        {0x1B, 0x4F, 0x50},
    "F2":        {0x1B, 0x4F, 0x51},
    "F3":        {0x1B, 0x4F, 0x52},
    "F4":        {0x1B, 0x4F, 0x53},
    "F5":        {0x1B, 0x5B, 0x31, 0x35, 0x7E},
    "F6":        {0x1B, 0x5B, 0x31, 0x37, 0x7E},
    "F7":        {0x1B, 0x5B, 0x31, 0x38, 0x7E},
    "F8":        {0x1B, 0x5B, 0x31, 0x39, 0x7E},
    "F9":        {0x1B, 0x5B, 0x32, 0x30, 0x7E},
    "F10":       {0x1B, 0x5B, 0x32, 0x31, 0x7E},
    "F11":       {0x1B, 0x5B, 0x32, 0x33, 0x7E},
    "F12":       {0x1B, 0x5B, 0x32, 0x34, 0x7E},
}

// ResolveKey converts a key name to terminal bytes.
// Supports: "Enter", "Ctrl+A", "F1", "Up", etc.
func ResolveKey(key string) []byte {
    // Direct lookup
    if seq, ok := KeyMap[key]; ok {
        return seq
    }

    // Ctrl+ sequences
    if len(key) > 5 && key[:5] == "Ctrl+" {
        r := []rune(key)[5]
        if r >= 'A' && r <= 'Z' {
            return []byte{byte(r - 'A' + 1)}
        }
        if r >= 'a' && r <= 'z' {
            return []byte{byte(r - 'a' + 1)}
        }
    }

    // Single character
    if len([]rune(key)) == 1 {
        return []byte(key)
    }

    return nil
}

// MouseSGR encodes a mouse event using the SGR extended protocol.
// button: 0=left, 1=middle, 2=right, 64=scrollUp, 65=scrollDown.
// x, y are 0-based cell coordinates.
// release=true sends button-up.
func MouseSGR(button, x, y int, release bool) []byte {
    b := byte(button)
    if release {
        b = 0 // release button is always 0 in SGR 1006
    }
    suffix := byte('M')
    if release {
        suffix = 'm'
    }
    // ESC[ < button ; x+1 ; y+1 M/m
    // Using strconv for speed over fmt.Sprintf
    return []byte("\x1b[<" + itoa(int(b)) + ";" + itoa(x+1) + ";" + itoa(y+1) + string(suffix))
}

// MouseSGRMove encodes a mouse motion event using the SGR 1006 protocol.
// Motion uses button value 35 and reports current coordinates without press/release.
func MouseSGRMove(x, y int) []byte {
	return []byte("\x1b[<35;" + itoa(x+1) + ";" + itoa(y+1) + "M")
}

// fast itoa for small integers (mouse coords are small)
func itoa(n int) string {
    if n < 0 {
        return "-" + itoa(-n)
    }
    if n < 10 {
        return string(byte('0' + n))
    }
    if n < 100 {
        return string(byte('0'+n/10)) + string(byte('0'+n%10))
    }
    // SGR button values are small, this is enough
    return string(byte('0'+n/100)) + string(byte('0'+(n/10)%10)) + string(byte('0'+n%10))
}
