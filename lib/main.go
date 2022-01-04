package main

//#include <stdlib.h>
import "C"
import (
	"fmt"
	"github.com/myml/webssh/common"
	"unsafe"
)

//export query
func query(token *C.char) *C.char {
	ip, err := common.Query(C.GoString(token))
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	return C.CString(ip)
}

//export release
func release(ip *C.char) {
	C.free(unsafe.Pointer(ip))
}

func main() {}
