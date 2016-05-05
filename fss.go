package main

/*
#cgo LDFLAGS: -lfzip -lz

#include <stdlib.h>

#include <femtozip.h>

const char *get_callback(int doc_index, int *doc_len, void *user_data);
void release_callback(const char *buf, void *user_data);
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

var docs [][]byte

//export go_get_callback
func go_get_callback(index C.int, length *C.int, data unsafe.Pointer) *C.char {
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

	if len(os.Args) == 1 {
		panic("use subcommand build or compress")
	}

	switch os.Args[1] {
	case "build":
		build.Parse(os.Args[2:])
	case "compress":
		compress.Parse(os.Args[2:])
	default:
		panic("invalid command")
	}

	flag.Parse()

	md := C.CString(mpath)
	defer C.free(unsafe.Pointer(md))

	if compress.Parsed() {
		if compress.NArg() != 1 {
			panic("invalid arguments")
		}

		b, err := hex.DecodeString(compress.Arg(0))

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

		dst := C.malloc(C.size_t(len(b) * 2))

		if dst == nil {
			panic("malloc failed")
		}

		defer C.free(dst)

		start := time.Now()
		l := int(C.fz_compress(model, src, C.int(len(b)), (*C.char)(dst), C.int(len(b)*2)))
		elapsed := time.Since(start)

		if l <= 0 {
			panic("fz_compress failed")
		}

		fmt.Printf("compressed %d bytes to %d bytes, %d net bytes\n", len(b), l, l-len(b))
		fmt.Printf("\toriginal %s\n", compress.Arg(0))
		fmt.Printf("\tcompressed %s\n", hex.EncodeToString((*[1 << 30]byte)(dst)[:l]))
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

	model := C.fz_build_model(C.int(len(docs)), (*[0]byte)(unsafe.Pointer(C.get_callback)), (*[0]byte)(unsafe.Pointer(C.release_callback)), nil)

	if model == nil {
		panic("fz_build_model failed")
	}

	defer C.fz_release_model(model)

	if C.fz_save_model(model, md) != C.int(0) {
		panic("fz_save_model failed")
	}
}
