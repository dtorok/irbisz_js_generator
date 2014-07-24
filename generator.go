package main

import "fmt"
import "io/ioutil"
import "os"
import "path/filepath"
import "bytes"

import "regexp"
import "strings"

import (
	iconv "github.com/djimenez/iconv-go"
)

type TreeElement interface {
	ToString(int) string
}

type Leaf struct {
	title  string
	picUrl string
	icoUrl string
}

type Popup struct {
	title  string
	icoUrl string
	url    string
}

type Branch struct {
	title    string
	picUrl   string
	icoUrl   string
	content  string
	children []TreeElement
}

func (leaf Leaf) ToString(_ int) string {
	return fmt.Sprintf("new leaf('%s', '%s', '%s')",
		leaf.title, leaf.picUrl, leaf.icoUrl)
}

func (popup Popup) ToString(_ int) string {
	return fmt.Sprintf("new popup('%s', '%s', '%s')",
		popup.title, popup.icoUrl, popup.url)
}

func (branch Branch) ToString(indent int) string {
	var buff *bytes.Buffer = bytes.NewBufferString("")
	buff.WriteString("[")
	for i, ch := range branch.children {
		if i > 0 {
			buff.WriteString(", ")
		}

		newLine(buff, indent+1)
		buff.WriteString(ch.ToString(indent + 1))
	}
	buff.WriteString("]")

	children := buff.String()

	re := regexp.MustCompile("^[0-9]+_")
	index := re.FindStringIndex(branch.title)
	var title string
	if (len(index) > 0) {
		title = branch.title[index[1]:]
	} else {
		title = branch.title
	}

	return fmt.Sprintf("new branch('%s', '%s', '%s', '%s', %s)",
		title, branch.picUrl, branch.content, branch.icoUrl, children)
}

func newLine(buff *bytes.Buffer, indent int) {
	buff.WriteString("\n")
	for i := 0; i < indent; i++ {
		buff.WriteString("    ")
	}
}

func urlPath(in string) string {
	return strings.Replace(in, " ", "%20", -1)
}

func filterFiles(files []os.FileInfo) []os.FileInfo {
	p := 0
	for _, file := range files {
		fname := file.Name()
		if strings.HasPrefix(fname, "_") {
			continue
		}

		if strings.HasPrefix(fname, "this") {
			continue
		}

		if !file.IsDir() && !strings.HasSuffix(fname, ".jpg") && !strings.HasSuffix(fname, ".png") &&
			!strings.HasSuffix(fname, ".JPG") && !strings.HasSuffix(fname, ".PNG") {
			continue
		}

		files[p] = file
		p += 1
	}

	return files[:p]
}

func findIn(c byte, s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}

	return -1
}

func normalizeUTF8(s string) string {
	table := [][]string{
		[]string{"é", "é"},
		[]string{"á", "á"},
		[]string{"ű", "ű"},
		[]string{"ő", "ő"},
		[]string{"ú", "ú"},
		[]string{"ö", "ö"},
		[]string{"ü", "ü"},
		[]string{"ó", "ó"},
		[]string{"í", "í"},
		[]string{"É", "É"},
		[]string{"Á", "Á"},
		[]string{"Ű", "Ű"},
		[]string{"Ő", "Ő"},
		[]string{"Ú", "Ú"},
		[]string{"Ö", "Ö"},
		[]string{"Ü", "Ü"},
		[]string{"Ó", "Ó"},
		[]string{"Í", "Í"},
	}

	for _, pair := range table {
		s = strings.Replace(s, pair[0], pair[1], -1)
	}

	return s
}

func formatTitle(title string) string {
	title = strings.Replace(title, "Oo", "Ő", -1)
	title = strings.Replace(title, "oo", "ő", -1)
	title = strings.Replace(title, "Uu", "Ű", -1)
	title = strings.Replace(title, "uu", "ű", -1)

	return title
}

func findFile(root string, urlroot string, filename string, find_jpegs bool) string {
	var options []string
	if find_jpegs {
		ext := filepath.Ext(filename)
		basename := filename[:len(filename)-len(ext)]
		options = []string{basename + ".jpg", basename + ".png"}
	} else {
		options = []string{filename}
	}
	

	for _, opt := range options {
		path := filepath.Join(root, opt)
		_, err := os.Stat(path)
		if err == nil {
			return filepath.Join(urlroot, path[len(root):])
		}
	}

	return ""
}

func BuildTree(path string, url string) TreeElement {
	var elem TreeElement

	path = normalizeUTF8(path)

	info, err := os.Stat(path)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if info.IsDir() {
		popup_url := urlPath(findFile(path, url, "popup.html", false))
		if popup_url == "" {
			this_txt := filepath.Join(path, "this.txt")

			title := formatTitle(filepath.Base(path))
			if title == "myRoot" {
				title = "Kezdőlap"
			}
			picUrl := urlPath(findFile(path, url, "this.jpg", true))
			icoUrl := urlPath(findFile(path, url, "_this.jpg", true))
			contentLatin2, _ := ioutil.ReadFile(this_txt)
			contents, _ := iconv.ConvertString(string(contentLatin2), "iso8859-2", "utf-8")
			contents = strings.Replace(contents, "\r\n", "\\n", -1)
			contents = strings.Replace(contents, "\n", "\\n", -1)
			contents = strings.Replace(contents, "'", "\\'", -1)

			files, _ := ioutil.ReadDir(path)
			files = filterFiles(files)
			children := make([]TreeElement, len(files))
			for i, file := range files {
				children[i] = BuildTree(filepath.Join(path, file.Name()), filepath.Join(url, file.Name()))
			}

			elem = Branch{title, picUrl, icoUrl, contents, children}
		} else {
			icoUrl := urlPath(findFile(path, url, "_this.jpg", true))
			title := formatTitle(filepath.Base(path))
			elem = Popup{title, icoUrl, popup_url}
		}
	} else {
		title := formatTitle(filepath.Base(url))
		picUrl := urlPath(url)
		icoUrl := urlPath(findFile(filepath.Dir(path), filepath.Dir(url), "_"+filepath.Base(path), true))
		if icoUrl == "" {
			urlPath(findFile(filepath.Dir(path), filepath.Dir(url), filepath.Base(path), true))
		}
		elem = Leaf{title, picUrl, icoUrl}
	}

	return elem
}

func generateJS(elem TreeElement) string {
	buff := bytes.NewBufferString("")
	buff.WriteString("function buildSiteMap() {\n")
	buff.WriteString("    var SiteMap = new tree();\n")
	buff.WriteString("    SiteMap.add(")
	buff.WriteString(elem.ToString(1))
	buff.WriteString(");\n")
	buff.WriteString("    return SiteMap;\n")
	buff.WriteString("}")

	return buff.String()
}

func main() {
	elem := BuildTree("/Users/dtorok/Workspace/misc/dadi/myRoot", "myRoot")
	ioutil.WriteFile("/tmp/output.js", []byte(generateJS(elem)), 0777)
}
