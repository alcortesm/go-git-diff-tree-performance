package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"srcd.works/go-git.v4/plumbing/object"
	"srcd.works/go-git.v4/utils/merkletrie"
)

var commitsOverwrite = []string{
// changes in type of file are detected as modifications
//   http://github.com/git/git.git
//	"bb831db6774aaa733199360dc7af6f3ce375fc20", // mode: 0x1a4
//	"18d0fec24027ac226dc2c4df2b955eef2a16462a", // mode: 0x800a000
//
// non-unicode chars in names are not escaped
//   http://github.com/git/git.git
//"aa9349d449bbf6bd7d28a5279f30a9734f77da8f",
//"6ab69bf253848d641fb08348eca10b7cf79fd275",
//
// submodules are ignored:
//   http://github.com/tensorflow/tensorflow.git
//     "634c37e65928c2556774d4273060b637dbcb9bc8",
//     "e7d00e3bed71af45614a7d733e081804b386cb8d", // modifies a submodule
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "bad number of arguments")
		usage()
		os.Exit(1)
	}

	err := do()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func do() error {
	path := os.Args[1]
	fmt.Printf("repo = %s\n", path)

	if strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "git://") {
		var err error
		path, err = clone(path)
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if len(exitErr.Stderr) != 0 {
					return fmt.Errorf(
						"%s: stderr=%q", err, string(exitErr.Stderr))
				}
			}
			return err
		}
		defer os.RemoveAll(path)
	}

	gitDiffs, err := doWithGit(path)
	if err != nil {
		return fmt.Errorf("git: %s", err)
	}

	gogitDiffs, err := doWithGogit(path)
	if err != nil {
		return fmt.Errorf("go-git: %s", err)
	}

	equal, report := compare(gitDiffs, gogitDiffs)
	if equal {
		fmt.Println("SUCCESS", report)
	} else {
		fmt.Println("FAIL", report)
	}

	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr,
		"usage: %s [path_to_repo | url_of_repo]\n", os.Args[0])
}

func clone(url string) (string, error) {
	path, err := ioutil.TempDir("", "go-git-diff-tree-performance-")
	if err != nil {
		return "", err
	}

	cmd := exec.Command("git", "clone", url, path)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, string(out))
		return "", err
	}

	return path, nil
}

func compare(a []*diff, b []*gogitResult) (bool, string) {
	if a == nil && b == nil {
		return true, "both are nil"
	}

	if a == nil {
		return false, "a is nil"
	}

	if b == nil {
		return false, "b is nil"
	}

	if len(a) != len(b) {
		return false,
			fmt.Sprintf("different lengths (%d, %d)", len(a), len(b))
	}

	bb, err := difftreeChangesToDiffs(b)
	if err != nil {
		log.Fatal(err)
	}

	fail := false
	for i, ea := range a {
		eb := bb[i]
		sort.Strings(ea.lines)
		sort.Strings(eb.lines)
		if !reflect.DeepEqual(ea, eb) {
			fmt.Println("==================================")
			fmt.Printf("GIT:\n%sGOGIT:\n%s", ea, eb)
			fmt.Println("==================================\n")
			fail = true
		}
	}

	return !fail, ""
}

func difftreeChangesToDiffs(rs []*gogitResult) ([]*diff, error) {
	ret := make([]*diff, 0, len(rs))

	for _, r := range rs {
		d := &diff{
			from:  r.from,
			to:    r.to,
			lines: make([]string, 0, len(r.changes)),
		}

		for _, c := range r.changes {
			var code string
			var name string
			action, err := c.Action()
			if err != nil {
				return nil, err
			}
			switch action {
			case merkletrie.Insert:
				code = "A"
				name = c.To.Name
			case merkletrie.Delete:
				code = "D"
				name = c.From.Name
			case merkletrie.Modify:
				if fileTypeChange(c.From.TreeEntry.Mode, c.To.TreeEntry.Mode) {
					code = "T"
				} else {
					code = "M"
				}
				name = c.From.Name
			}

			line := code + "\t" + quoteUnicode(name)
			d.lines = append(d.lines, line)
		}
		ret = append(ret, d)
	}

	return ret, nil
}

// git qoutes non-unicode charactes in a special way:
// example: gitweb/test/Märchen will be output as
// "gitweb/test/M\303\244rchen", which is bytes in octal,
// prefixed by a "\" and all the string inside quotes.
func quoteUnicode(s string) string {
	var buf bytes.Buffer
	rune := make([]byte, 3)
	nonAscii := false

	for i, r := range s {
		value, width := utf8.DecodeRuneInString(s[i:])
		if width > 1 {
			nonAscii = true
			n := utf8.EncodeRune(rune, value)
			for i := 0; i < n; i++ {
				buf.WriteRune('\\')
				fmt.Fprintf(&buf, "%o", rune[i])
			}
		} else {
			buf.WriteRune(r)
		}
	}

	if nonAscii {
		return `"` + buf.String() + `"`
	}
	return buf.String()
}

// When a file has change from link to file,
// git will not say that is has been modified ("M"),
// but that its type has change ("T").
//
// This function detects this situation.
func fileTypeChange(a, b os.FileMode) bool {
	//fmt.Printf("fileTypeChange a: %#v (%16b) isRegular:%t isLink:%t\n", a, uint32(a), isRegular(a), isLink(a))
	//fmt.Printf("fileTypeChange b: %#v (%16b) isRegular:%t isLink:%t\n", b, uint32(b), isRegular(b), isLink(b))
	if isRegular(a) && isLink(b) ||
		isLink(a) && isRegular(b) {
		return true
	}
	return false
}

func isRegular(a os.FileMode) bool {
	//fmt.Printf("isRegular original %#v (%16b)\n", a, uint32(a))
	//fmt.Printf("isRegular filemode %#v (%16b)\n", object.FileMode, uint32(object.FileMode))
	//fmt.Printf("isRegular deprecat %#v (%16b)\n", object.FileModeDeprecated, uint32(object.FileModeDeprecated))
	//fmt.Printf("isRegular 0644     %#v (%16b)\n", os.FileMode(0644), uint32(os.FileMode(0644)))
	return a == object.FileMode ||
		a == object.FileModeDeprecated ||
		a == os.FileMode(0644)
}

func isLink(a os.FileMode) bool {
	//fmt.Printf("isLink original %#v (%16b)\n", a, uint32(a))
	//fmt.Printf("isLink symlink  %#v (%16b)\n", object.SymlinkMode, uint32(object.SymlinkMode))
	return a&object.SymlinkMode == object.SymlinkMode
}
