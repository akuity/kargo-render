package file

import (
	"fmt"
	"os"
	"strings"
)

// Exists returns a bool indicating if the specified file exists or not. It
// returns any errors that are encountered that are NOT an os.ErrNotExist error.
func Exists(filename string) (bool, error) {
	_, err := os.Stat(filename)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// ExpandPath expands the provided pathTemplate, replacing placeholders of the
// form ${n} where n is a non-negative integer, with corresponding values from
// the provided string array. The expanded path is returned.
func ExpandPath(pathTemplate string, values []string) string {
	for i := 0; i < len(values); i++ {
		pathTemplate = strings.ReplaceAll(
			pathTemplate,
			fmt.Sprintf("${%d}", i),
			values[i],
		)
	}
	return pathTemplate
}
