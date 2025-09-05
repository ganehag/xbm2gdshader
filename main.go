// xbm2gdshader - convert XBM files into a self-contained Godot 4 canvas_item shader
// Usage: go run main.go -in test.xbm -out test.gdshader [-type canvas_item|spatial] [-fg "#000000FF"] [-bg "#00000000"]
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var version = "0.1.0"

var (
	// Width/height #defines (any symbol prefix)
	reW = regexp.MustCompile(`(?m)#define\s+\w+_width\s+(\d+)`)
	reH = regexp.MustCompile(`(?m)#define\s+\w+_height\s+(\d+)`)

	// Permissive: match "<name>_bits[] = { ... };", ignore qualifiers/types
	reArr = regexp.MustCompile(`(?s)[A-Za-z_]\w*_bits\[\]\s*=\s*\{(.*?)\};`)

	// Accept hex (0x..), decimal; treat bare numbers as decimal
	reNum = regexp.MustCompile(`0[xX][0-9A-Fa-f]+|\d+`)
)

func main() {
	in := flag.String("in", "", "input .xbm file")
	out := flag.String("out", "out.gdshader", "output .gdshader path")
	shType := flag.String("type", "canvas_item", "shader type: canvas_item or spatial")
	fg := flag.String("fg", "#000000FF", "foreground RGBA (hex #RRGGBBAA)")
	bg := flag.String("bg", "#00000000", "background RGBA (hex #RRGGBBAA)")
	flag.Parse()

	if *in == "" {
		fail("missing -in")
	}

	src, err := os.ReadFile(*in)
	check(err)

	w, h, raw, err := parseXBM(string(src))
	check(err)

	data32 := repackBitsToU32(raw, w, h)

	fgVec, err := hexToVec4(*fg)
	check(err)
	bgVec, err := hexToVec4(*bg)
	check(err)

	sh := buildShader(*shType, w, h, data32, fgVec, bgVec)

	check(os.WriteFile(*out, []byte(sh), 0o644))
	fmt.Printf("Wrote %s (%dx%d, %d uints)\n", *out, w, h, len(data32))
}

func check(err error) {
	if err != nil {
		fail(err.Error())
	}
}
func fail(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}

func parseXBM(s string) (int, int, []byte, error) {
	wm := reW.FindStringSubmatch(s)
	hm := reH.FindStringSubmatch(s)
	am := reArr.FindStringSubmatch(s)
	if wm == nil || hm == nil || am == nil {
		return 0, 0, nil, errors.New("failed to parse #defines or bits array")
	}
	w, _ := strconv.Atoi(wm[1])
	h, _ := strconv.Atoi(hm[1])

	nums := reNum.FindAllString(am[1], -1)
	if len(nums) == 0 {
		return 0, 0, nil, errors.New("no numbers found in bits array")
	}

	// Build raw byte stream; if value > 0xFF, assume 16-bit little-endian (common for short-based XBM).
	out := make([]byte, 0, len(nums))
	for _, t := range nums {
		var v int64
		var err error
		if strings.HasPrefix(t, "0x") || strings.HasPrefix(t, "0X") {
			v, err = strconv.ParseInt(t[2:], 16, 64)
		} else {
			v, err = strconv.ParseInt(t, 10, 64)
		}
		if err != nil {
			return 0, 0, nil, fmt.Errorf("bad number %q: %w", t, err)
		}
		if v < 0 {
			v = 0
		}
		if v <= 0xFF {
			out = append(out, byte(v))
		} else {
			out = append(out, byte(v&0xFF), byte((v>>8)&0xFF))
		}
	}
	return w, h, out, nil
}

// XBM rows are byte-padded, LSB-first within each byte.
// Repack to tight bitstream (width*height bits) → 32-bit words for the shader.
func repackBitsToU32(xbm []byte, w, h int) []uint32 {
	rowBytes := (w + 7) / 8
	totalBits := w * h
	words := (totalBits + 31) / 32
	dst := make([]uint32, words)

	setBit := func(i int) {
		dst[i>>5] |= 1 << uint(i&31)
	}

	outIdx := 0
	for y := 0; y < h; y++ {
		base := y * rowBytes
		for x := 0; x < w; x++ {
			bi := base + (x >> 3)
			if bi >= len(xbm) {
				break
			}
			bit := (xbm[bi] >> uint(x&7)) & 1 // LSB is leftmost pixel
			if bit == 1 {
				setBit(outIdx)
			}
			outIdx++
		}
	}
	return dst
}

func hexToVec4(hex string) (string, error) {
	s := strings.TrimPrefix(hex, "#")
	if len(s) != 8 {
		return "", fmt.Errorf("want #RRGGBBAA, got %q", hex)
	}
	r, _ := strconv.ParseUint(s[0:2], 16, 8)
	g, _ := strconv.ParseUint(s[2:4], 16, 8)
	b, _ := strconv.ParseUint(s[4:6], 16, 8)
	a, _ := strconv.ParseUint(s[6:8], 16, 8)
	return fmt.Sprintf("vec4(%g,%g,%g,%g)",
		float32(r)/255, float32(g)/255, float32(b)/255, float32(a)/255), nil
}

func buildShader(shaderType string, w, h int, data []uint32, fg, bg string) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "shader_type %s;\n\n", shaderType)

	// Constants
	fmt.Fprintf(&buf, "const uint WIDTH = %du;\n", w)
	fmt.Fprintf(&buf, "const uint HEIGHT = %du;\n", h)
	fmt.Fprintf(&buf, "const uint WORDS = %du;\n\n", len(data))

	// Uniforms
	buf.WriteString("// Foreground = bit 1 (XBM 'black'); Background = bit 0\n")
	fmt.Fprintf(&buf, "instance uniform vec4 fg_color = %s;\n", fg)
	fmt.Fprintf(&buf, "instance uniform vec4 bg_color = %s;\n", bg)
	buf.WriteString("instance uniform bool invert = false;\n")
	// buf.WriteString("uniform ivec2 tile_repeat = ivec2(8, 6); // (unused in pixel-perfect mode)\n")
	buf.WriteString("\n")

	// Data array
	buf.WriteString("const uint DATA[WORDS] = uint[](\n")
	for i, v := range data {
		sep := ","
		if i == len(data)-1 {
			sep = ""
		}
		fmt.Fprintf(&buf, "    0x%08Xu%s\n", v, sep)
	}
	buf.WriteString(");\n\n")

	// Bit lookup
	buf.WriteString(`bool xbm_bit(ivec2 p) {
    if (p.x < 0 || p.y < 0 || p.x >= int(WIDTH) || p.y >= int(HEIGHT)) return false;
    int idx = p.y * int(WIDTH) + p.x;
    uint w = DATA[idx >> 5];
    return ((w >> uint(idx & 31)) & 1u) == 1u;
}
` + "\n")

	// Pixel-perfect tiling fragment (screen-locked)
	if shaderType == "canvas_item" {
		buf.WriteString(`void fragment() {
    // Convert normalized screen UV (0..1) into integer screen pixel coords
    vec2 screen_px = floor(SCREEN_UV / SCREEN_PIXEL_SIZE);

    // Tile every WIDTH × HEIGHT screen pixels
    int px = int(mod(screen_px.x, float(WIDTH)));
    int py = int(mod(screen_px.y, float(HEIGHT)));
    ivec2 p = ivec2(px, py);

    bool on = xbm_bit(p);
    float v = on ? 1.0 : 0.0;
    if (invert) v = 1.0 - v;
    COLOR = mix(bg_color, fg_color, v);
}
`)
	} else {
		// Spatial variant: ALBEDO/ALPHA
		buf.WriteString(`void fragment() {
    vec2 screen_px = floor(SCREEN_UV / SCREEN_PIXEL_SIZE);
    int px = int(mod(screen_px.x, float(WIDTH)));
    int py = int(mod(screen_px.y, float(HEIGHT)));
    ivec2 p = ivec2(px, py);

    bool on = xbm_bit(p);
    float v = on ? 1.0 : 0.0;
    if (invert) v = 1.0 - v;
    ALBEDO = mix(bg_color.rgb, fg_color.rgb, v);
    ALPHA  = mix(bg_color.a,   fg_color.a,   v);
}
`)
	}
	return buf.String()
}
