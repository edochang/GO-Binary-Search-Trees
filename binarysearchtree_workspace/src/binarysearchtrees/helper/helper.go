package helper

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"
)

func Check(e error) {
	if e != nil {
		fmt.Fprintf(os.Stderr, "os.Open: %v\n", e)
		panic(e)
	}
}

func ReadFile(filename string) []string {
	fileData, err := os.Open(filename)

	Check(err)

	fileScanner := bufio.NewScanner(fileData)
	fileScanner.Split(bufio.ScanLines)

	var fileLines []string

	for fileScanner.Scan() {
		//fmt.Fprint(os.Stdout, "line: %v", fileScanner.Text())
		fileLines = append(fileLines, fileScanner.Text())
	}

	fileData.Close()

	/*
		// debug statement
		for _, line := range fileLines {
			fmt.Println(line)
		}
		fmt.Println(fileLines)
	*/

	return fileLines
}

func EqualSlices(x, y []int) bool {
	if len(x) != len(y) {
		return false
	}

	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func PrintTime(time time.Duration, label string, mw *io.Writer) {
	fmt.Fprintf(*mw, "%s: %v \n", label, time)
}
