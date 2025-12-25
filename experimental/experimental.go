// experimental is collection of functions I find useful but can't categorize yet into their own package.
// WARN: All exported symbols in this package will be removed eventually.
package experimental

import (
	"encoding/json"
	"fmt"
)

// PrintJSON is useful for pretty printing structs instead of the "%#v" format specifier which
// writes everything in one line.
func PrintJSON(obj interface{}) {
	text, _ := json.MarshalIndent(obj, "\t", "\t")
	fmt.Println(string(text))
}
