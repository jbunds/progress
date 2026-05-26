Provision a hard-capped, fixed-size, reusable rendering buffer for a terminal width of 256 columns. The `Progress` struct runtime singleton will detect the TTY versus non-TTY case during construction and provision the rendering buffer accordingly. The calculations for the two cases are as follows:

1. TTY case:

    ```
    ( 256 columns *
       32 bytes   *                    // per each column to account for the 24-bit RGB ANSI color blocks
        4 bytes )                      // per each column to account for each rendered char (4 bytes == worst-case UTF-8 rune width)
    +   9 bytes == len(clearSeq      ) // once per line (clearSeq       == "\r\033[?2026h"           )
    +  12 bytes == len(prefix        ) // once per line (prefix         == "processing ("            )
    +   4 bytes == len(suffix        ) // once per line (suffix         == "%): "                    )
    +  15 bytes == len(lineTerminator) // once per line (lineTerminator == "\033[K\033[0m\033[?2026l")

    columns:               256
    per-column bytes:       32 *   4 == 128
    total buffer capacity: 256 * 128 == 32768 bytes
    ```

2. non-TTY case:
    ```
    ( 256 columns *
        4 bytes )                      // per each column to account for each rendered char (4 bytes == worst-case UTF-8 rune width)
    +   0 bytes == len(clearSeq      ) // once per line (clearSeq       == ""            )
    +   9 bytes == len(prefix        ) // once per line (prefix         == "processing (")
    +   4 bytes == len(suffix        ) // once per line (suffix         == "%): "        )
    +   1 byte  == len(lineTerminator) // once per line (lineTerminator == "\n"          )
    columns:               256
    per-column bytes:        4
    total buffer capacity: 256 * 4 == 1024 bytes
    ```

To mitigate GC pressure, a `sync.Pool` will manage the concurrent use and recycling of the provisioned rendering buffers at runtime, via a new `bufPool` field (type `*bufPool[[]byte]`) of the `Progress` struct:

```go
type bufPool[T any] struct { pool sync.Pool }

func newBufPool[T any](factory func() T) *bufPool[T] {
  return &bufPool[T]{
    pool: sync.Pool{
      New: func() any { return factory() },
    },
  }
}

func (p *bufPool[T]) get() T  { return p.pool.Get().(T) }
func (p *bufPool[T]) put(x T) {        p.pool.Put(x)    }
```

// ansi escape sequences for setting background and foreground colors can be background-first, or foreground-first
// background-first combined format:
//   \033[ 48;2;R;G;B ; 38;2;R;G;B m
combinedColor := "\033[48;2;255;255;255" + ";" + "38;2;255;255;255m"

// generalized form (background-first order):
  "\033[48;2;255;255;255" + // background color block
  ";"                     + // ansi sequence delimiter
  "38;2;255;255;255m"     + // foreground color block
  "string to be rendered"
  therefore total overhead == 32 bytes:
  - \033[            ==  2 bytes (CSI)
  - 48;2;255;255;255 == 14 bytes
  - ;                ==  1 byte
  - 38;2;255;255;255 == 14 bytes
  - m                ==  1 byte  (SGR terminator)

therefore, a pre-allocated buffer which accounts for only the ANSI 24-bit RGB color sequences and the worst-case UTF-8 text would be:

  make([]byte, 0, 32 + len(4 * textStr))

and a precisely-allocated, UTF-8-aware buffer would be:

  make([]byte, 0, 32 + utf8.RuneCountInString(textStr))
  make([]byte, 0, 32 + len(textBytes)) // NOT utf8.RuneCount(textBytes) !

general (short) form:
  \033[38;2;R;G;B;48;2;R;G;Bm
  '033[38;2; == always 24-bit RGB foreground

\033[30;48;2;255;255;255m == longest RGB fg start block == 19 bytes
\033[38;2;                == longest RGB fg start block ==  7 bytes

// Go strings are 2-word, 16-byte headers, roughly:
//
// type string struct {
//   array uintptr // 8 bytes (points to the backing data array)
//   len   int     // 8 bytes (current length)
// }
//
// Go slices are 3-word, 24-byte headers, roughly:
//
// type slice struct {
//   array uintptr // 8 bytes (points to the backing data array)
//   len   int     // 8 bytes (current length)
//   cap   int     // 8 bytes (total capacity)
// }
//
// so methods and functions which pass and return naked `[]byte` slices _by value_ (as opposed to `*[]byte`)
// is a long-established idiomatic pattern, e.g., https://pkg.go.dev/builtin#append

// initBufPool initializes a pool of reusable naked buffers with adequate capacity to accommodate
// rendering 256 columns of 24-bit color ANSI escape sequence-wrapped characters to the terminal.
func (p *Progress) initBufPool() {
	termWidth := int(p.state.Load() >> 16)
	if p.isTerminal(p.output) && termWidth < 256 { termWidth = 256 }
	capacity := p.layout.bufCap(termWidth)
	p.bufPool = newBufPool(func() []byte { return make([]byte, 0, capacity) })
}

---

a string in Go is a 2-word header: \[pointer to data (uintptr) | length (int)\]
a []byte in Go is a 3-word header: \[pointer to data (uintptr) | length (int) | capacity (int)\]

atomic.Uintptr cannot atomically swap **one single pointer (1 word)**, a []byte slice header cannot be atomically updated directly

swapping just the data pointer while the length and capacity are modified non-atomically is not thread-safe. atomic.Pointer is cleaner and safer.

```
const maxCellBytes = 288 // 256 (cells) + 32 (ansi overhead)

// ttyCell represents a flat block of memory, and requires zero heap allocs when modified.
type ttyCell struct {
  buffer [maxCellBytes]byte
  length int
}

// API boundary ingestion of Report(..., status string)
// this implements a zero-alloc byte cast:
func ingestString(input string) []byte {
  if input == "" { return nil }
  // the []byte returned here is read-only and must be copied into the pre-allocated buffer by the caller
  return unsafe.Slice(unsafe.StringData(input), len(input)) // direct, zero-copy of the string's underlying data array
}

// use sync.Pool to recycle memory structures and atomic.Pointer to guarantee lock-free concurrent reads and writes:

// reset clears the length pointer so the struct can be safely reused.
func (c *ttyCell) reset() { c.length = 0 }

// cellPool manages the reuse of ttyCell structs.
var cellPool = sync.Pool{ New: func() any { return new(ttyCell) } }

// concurrentCell provides a lock-free, concurrency-safe wrapper around ttyCell.
type concurrentCell struct { cell atomic.Pointer[ttyCell] }

func (cc *concurrentCell) store(newCell * ttyCell) {
  c.value.Store(newCell)
}

// load retrieves the current cell configuration safely across goroutines.
func (cc *concurrentCell) load() *ttyCell { return cc.cell.Load() }

// update safely swaps the cell using data ingested from the string passed by callers via Report()
func (cc *concurrentCell) update(bgR, bgG, bgB, fgR, fgG, fgB uint8, status string) {
  newCell := cellPool.Get().(*ttyCell) // grab a clean, pre-allocated cell from the pool
  newCell.reset()
  textBytes     := unsafe.Slice(unsafe.StringData(status), len(status)) // zero-copy ingestion of the status string passed by callers via Report()
  newCell.length = writeAnsiSeq(newCell.buffer[:], bgR, bgG, bgB, fgR, fgG, fgB, textBytes) // manually construct the maximum-width ANSI sequence into the flat buffer
  oldCell       := cc.cell.Swap(newCell)
  if oldCell != nil { cellPool.Put(oldCell) } // return the old cell to the pool for reuse
}

func writeAnsiSeq(dst []byte, bgR, bgG, bgB, fgR, fgG, fgB uint8, text []byte) int {
  // append background colors
  dst[0], dst[1], dst[2], dst[3], dst[4], dst[5], dst[6], dst[7] = '\033', '[', '4', '8', ';', '2', ';', ' '
  idx := 7
  idx = appendUint8Ascii(dst, idx, bgR); dst[idx] = ';'; idx++
  idx = appendUint8Ascii(dst, idx, bgG); dst[idx] = ';'; idx++
  idx = appendUint8Ascii(dst, idx, bgB); dst[idx] = ';'; idx++
  // append foreground colors
  dst[idx], dst[idx+1], dst[idx+2], dst[idx+3], dst[idx+4] = '3', '8', ';', '2', ';'
  idx += 5
  idx = appendUint8Ascii(dst, idx, fgR); dst[idx] = ';'; idx++
  idx = appendUint8Ascii(dst, idx, fgG); dst[idx] = ';'; idx++
  idx = appendUint8Ascii(dst, idx, fgB); dst[idx] = 'm'; idx++
  // copy the raw text bytes directly into the buffer
  copy(dst[idx:], text)
  idx += len(text)
  return idx
}

// Fast integer to ASCII digit converter that writes directly to an existing slice
func appendUint8Ascii(dst []byte, idx int, val uint8) int {
  if val >= 100 {
    dst[idx] = '0' + (val / 100); idx++
    val %= 100
    dst[idx] = '0' + (val / 10); idx++
    dst[idx] = '0' + (val % 10); idx++
  } else if val >= 10 {
    dst[idx] = '0' + (val / 10); idx++
    dst[idx] = '0' + (val % 10); idx++
  } else {
    dst[idx] = '0' + val; idx++
  }
  return idx
}
```

- instead of passing strings down through the call stack, pass either:
  - uintptr
  - ptr
  - *[]byte
  - []byte
- move all constants currently defined in render.go to layout.go (need to think about this some more...)
- move all test helpers to init_test.go
- check if test-only hooks get compiled into the integration test binary; i think they are, and the should be, so the integration test can use them (e.g., drawNotify)

clearSeq       = "\r\033[?2026h"            // move cursor to beginning of line; activate synchronized output mode
lineTerminator = "\033[K\033[0m\033[?2026l" // erase from cursor position to end of line; reset all attributes; deactivate synchronized output mode
prepColorSeq   = "\033[30;48;2;"            // set foreground (text) color to black (30); set background color (48) per incoming 24-bit RGB triplet (2)
setFgColor     = "\033[38;2;"               // set foreground (text) color (38) per incoming 24-bit RGB triplet (2)
prepSetBgColor = ";48;2;"                   // prepare to set background color (48) per incoming 24-bit RGB triplet
resetAttr      = "\033[0m"                  // reset all attributes to defaults

prefix               = "processing (" // prepended to each progress status line rendered to the terminal
defaultSuffix        = "%): "         // appended to each percentage status calculation rendered to the terminal

minWidth             = 80 // fallback for pipes, redirects, and non-tty outputs
pctFieldLen          =  3 // the fixed length of the percentage displayed (e.g., "0.0", " 37", "100")
colorBlockMultiplier = 23 // 23 bytes per column for 24-bit color gradient blocks
utf8TruncMultiplier  =  4 //  4 bytes per column for worst-case UTF-8 status text truncation thresholds

// layout encapsulates the terminal-specific rendering layout configuration.
type layout struct {
	staticWidth    int    // the static width reserved for the prefix prepended to each status message, e.g., "processing (7.4%): "
	prefix         string // prepended to each progress status line rendered to the terminal
	suffix         string // appended to each status percentage calculation rendered to the terminal, e.g., "%): " or "%)"
	clearSeq       string // ANSI escape sequence used to clear the current terminal line
	doneSeq        string // ANSI escape sequence used to restore the terminal cursor
	lineTerminator string // output line terminator: "" when *Progress.output (nominally os.Stderr) is a terminal; "\n" otherwise
	finalStatus    string // status message to display upon completion (e.g., "done")
}

func defaultLayout() layout {
	layout := layout{
		prefix:         prefix,
		suffix:         defaultSuffix,
		clearSeq:       "",
		doneSeq:        "\n",
		lineTerminator: "\n",
		finalStatus:    "done",
	}
	layout.staticWidth = len(layout.prefix) + pctFieldLen + len(layout.suffix)
	return layout
}

func (l layout) bufCap(termWidth int) int {
	return (colorBlockMultiplier * termWidth) +
	       (utf8TruncMultiplier  * termWidth) +
	       len(l.prefix                     ) +
	       len(l.suffix                     ) +
	       len(l.clearSeq                   ) +
	       len(l.lineTerminator             )
}
