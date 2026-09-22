// eshdump 将 easyshell 录制文件转储为可读文本。
//
//	用法: eshdump [-hex] [-no-redact] <recording-file>
package main

import (
	"flag"
	"fmt"
	"github.com/3th1nk/easyshell/v2/record"
	"os"
)

func main() {
	hex := flag.Bool("hex", false, "以十六进制显示负载")
	noRedact := flag.Bool("no-redact", false, "不打码含密码的帧")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: eshdump [-hex] [-no-redact] <recording-file>")
		os.Exit(2)
	}

	src, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer src.Close()

	if err = record.Dump(src, os.Stdout, record.DumpOptions{Hex: *hex, NoRedact: *noRedact}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
