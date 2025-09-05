# xbm2gdshader

`xbm2gdshader` is a Go tool that converts [X Bitmaps (XBM)](https://en.wikipedia.org/wiki/X_BitMap) into self-contained [Godot 4](https://godotengine.org/) shaders.  
The generated shaders render your bitmap tiled across the screen, aligned to screen pixels.

## Features

- Parses `.xbm` files (`static char`, `unsigned char`, or `short` arrays).
- Repackages 1-bit image data into a compact `uint[]` for use in Godot shaders.
- Generates Godot 4 `.gdshader` files for `canvas_item` and `spatial` types.
- Pixel-perfect tiling: each XBM pixel maps directly to a screen pixel.
- Foreground/background colours and invert flag are exposed as instance uniforms.

## Installation

Clone and build:

```bash
git clone https://github.com/ganehag/xbm2gdshader.git
cd xbm2gdshader
go build -o xbm2gdshader main.go
````

Or run directly:

```bash
go run main.go -in /path/to/input.xbm -out output.gdshader
```

## Usage

```bash
xbm2gdshader -in input.xbm -out pattern.gdshader \
  -fg "#000000FF" -bg "#00000000" -type canvas_item
```

### Options

| Flag    | Default        | Description                             |
| ------- | -------------- | --------------------------------------- |
| `-in`   | *(required)*   | Input `.xbm` file                       |
| `-out`  | `out.gdshader` | Output shader path                      |
| `-type` | `canvas_item`  | Shader type: `canvas_item` or `spatial` |
| `-fg`   | `#000000FF`    | Foreground colour in `#RRGGBBAA` format |
| `-bg`   | `#00000000`    | Background colour in `#RRGGBBAA` format |

## Using in Godot 4

1. Convert an XBM file to a shader:

   ```bash
   xbm2gdshader -in /usr/include/X11/bitmaps/root_weave -out root_weave.gdshader
   ```

2. In Godot:

   * Create a `ColorRect` (or any node with a ShaderMaterial).
   * Add a new ShaderMaterial and assign `root_weave.gdshader`.
   * Adjust `fg_color`, `bg_color`, and `invert` under *Instance Shader Parameters*.

The bitmap will tile across the viewport aligned to screen pixels.

## Example

Given an XBM file:

```c
#define root_weave_width 4
#define root_weave_height 4
static char root_weave_bits[] = {
   0x07, 0x0d, 0x0b, 0x0e};
```

The generated shader includes:

```glsl
shader_type canvas_item;

const uint WIDTH = 4u;
const uint HEIGHT = 4u;

instance uniform vec4 fg_color = vec4(0,0,0,1);
instance uniform vec4 bg_color = vec4(0,0,0,0);
instance uniform bool invert = false;

...
```

## License

MIT License.
