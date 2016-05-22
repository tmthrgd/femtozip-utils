package main

/*
#cgo LDFLAGS: -lfzip -lz

#include <stdlib.h>

#include <femtozip.h>

const char *get_callback(int doc_index, int *doc_len, void *user_data);
void release_callback(const char *buf, void *user_data);
int dest_writer(const char *buf, size_t len, void *arg);
*/
import "C"

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	"unsafe"
)

//export go_get_callback
func go_get_callback(index C.int, length *C.int, data unsafe.Pointer) *C.char {
	docs := *(*[][]byte)(data)

	p := bytesToC(docs[index])

	if p == nil {
		return nil
	}

	*length = C.int(len(docs[index]))
	return p
}

//export go_release_callback
func go_release_callback(buf *C.char, data unsafe.Pointer) {
	C.free(unsafe.Pointer(buf))
}

//export go_dest_writer
func go_dest_writer(buf *C.char, len C.size_t, arg unsafe.Pointer) C.int {
	*(*[]byte)(arg) = append([]byte(nil), (*[1 << 30]byte)(unsafe.Pointer(buf))[:len:len]...)

	return C.int(0)
}

func bytesToC(data []byte) *C.char {
	p := C.malloc(C.size_t(len(data)))

	if p == nil {
		return nil
	}

	pp := (*[1 << 30]byte)(p)
	copy(pp[:], data)

	return (*C.char)(p)
}

func main() {
	var mpath string

	build := flag.NewFlagSet("build", flag.ExitOnError)
	build.StringVar(&mpath, "model", "session.model", "")

	compress := flag.NewFlagSet("compress", flag.ExitOnError)
	compress.StringVar(&mpath, "model", "session.model", "")

	decompress := flag.NewFlagSet("decompress", flag.ExitOnError)
	decompress.StringVar(&mpath, "model", "session.model", "")

	if len(os.Args) == 1 {
		panic("use subcommand build, compress or decompress")
	}

	switch os.Args[1] {
	case "build":
		build.Parse(os.Args[2:])
	case "compress":
		compress.Parse(os.Args[2:])
	case "decompress":
		decompress.Parse(os.Args[2:])
	default:
		panic("invalid command")
	}

	flag.Parse()

	md := C.CString(mpath)
	defer C.free(unsafe.Pointer(md))

	if compress.Parsed() || decompress.Parsed() {
		var b []byte
		var err error

		if compress.Parsed() {
			if compress.NArg() != 1 {
				panic("invalid arguments")
			}

			b, err = hex.DecodeString(compress.Arg(0))
		} else if decompress.Parsed() {
			if decompress.NArg() != 1 {
				panic("invalid arguments")
			}

			b, err = hex.DecodeString(decompress.Arg(0))
		}

		if err != nil {
			panic(err)
		}

		model := C.fz_load_model(md)

		if model == nil {
			panic("fz_load_model failed")
		}

		defer C.fz_release_model(model)

		src := bytesToC(b)

		if src == nil {
			panic("malloc failed")
		}

		defer C.free(unsafe.Pointer(src))

		var dst []byte
		var elapsed time.Duration

		if compress.Parsed() {
			start := time.Now()
			if C.fz_compress_writer(model, src, C.size_t(len(b)), (*[0]byte)(unsafe.Pointer(C.dest_writer)), unsafe.Pointer(&dst)) != C.int(0) {
				panic("fz_compress_writer failed")
			}
			elapsed = time.Since(start)

			fmt.Printf("compressed %d bytes to %d bytes, %d net bytes\n", len(b), len(dst), len(dst)-len(b))
			fmt.Printf("\toriginal %s\n", compress.Arg(0))
			fmt.Printf("\tcompressed %x\n", dst)
		} else /* decompress.Parsed() */ {
			start := time.Now()
			if C.fz_decompress_writer(model, src, C.size_t(len(b)), (*[0]byte)(unsafe.Pointer(C.dest_writer)), unsafe.Pointer(&dst)) != C.int(0) {
				panic("fz_decompress_writer failed")
			}
			elapsed = time.Since(start)

			fmt.Printf("decompressed %d bytes to %d bytes, %d net bytes\n", len(b), len(dst), len(dst)-len(b))
			fmt.Printf("\tcompressed %s\n", decompress.Arg(0))
			fmt.Printf("\toriginal %x\n", dst)
		}

		fmt.Println()
		fmt.Printf("\ttook %s\n", elapsed)
		return
	}

	/* build.Parsed() */
	var f *os.File

	if build.NArg() == 0 || build.Arg(0) == "-" {
		f = os.Stdin
	} else {
		var err error
		f, err = os.Open(build.Arg(0))

		if err != nil {
			panic(err)
		}
	}

	var docs [][]byte

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		// 2016/05/04 22:25:13 [info] 15333#15333:
		s := strings.SplitN(scanner.Text(), " ", 5)
		h := s[len(s)-1]
		b, err := hex.DecodeString(h)

		if err != nil {
			panic(err)
		}

		docs = append(docs, b)
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}

	if len(docs) == 0 {
		return
	}

	model := C.fz_build_model(C.int(len(docs)), (*[0]byte)(unsafe.Pointer(C.get_callback)), (*[0]byte)(unsafe.Pointer(C.release_callback)), unsafe.Pointer(&docs))

	if model == nil {
		panic("fz_build_model failed")
	}

	defer C.fz_release_model(model)

	if C.fz_save_model(model, md) != C.int(0) {
		panic("fz_save_model failed")
	}
}
